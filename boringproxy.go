package boringproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/caddyserver/certmagic"
)

type Config struct {
	WebUiDomain   string `json:"webui_domain"`
	SshServerPort int    `json:"ssh_server_port"`
	PublicIp      string `json:"public_ip"`
}

type SmtpConfig struct {
	Server   string
	Port     int
	Username string
	Password string
}

type Server struct {
	db           *Database
	tunMan       *TunnelManager
	httpClient   *http.Client
	httpListener *PassthroughListener
}

type IpResponse struct {
	Ip string `json:"ip"`
}

func checkPublicAddress(host string, port int) error {

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	defer ln.Close()

	code, err := genRandomCode(32)
	if err != nil {
		return err
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				break
			}
			conn.Write([]byte(code))
			conn.Close()
		}
	}()

	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil
	}
	defer conn.Close()

	data, err := io.ReadAll(conn)
	if err != nil {
		return nil
	}

	retCode := string(data)

	if retCode != code {
		return errors.New("Mismatched codes")
	}

	return nil
}

func getPublicIp() (string, error) {
	resp, err := http.Get("https://api.ipify.org?format=json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	var ipRes *IpResponse
	err = json.Unmarshal([]byte(body), &ipRes)
	if err != nil {
		return "", err
	}

	return ipRes.Ip, nil
}

func Listen() {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	newAdminDomain := flagSet.String("admin-domain", "", "Admin Domain")
	sshServerPort := flagSet.Int("ssh-server-port", 22, "SSH Server Port")
	certDir := flagSet.String("cert-dir", "", "TLS cert directory")
	err := flagSet.Parse(os.Args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: parsing flags: %s\n", os.Args[0], err)
	}

	log.Println("Starting up")

	ip, err := getPublicIp()
	if err != nil {
		log.Fatal(err)
	}

	err = checkPublicAddress(ip, 80)
	if err != nil {
		log.Fatal(err)
	}

	err = checkPublicAddress(ip, 443)
	if err != nil {
		log.Fatal(err)
	}

	db, err := NewDatabase()
	if err != nil {
		log.Fatal(err)
	}

	if *certDir != "" {
		certmagic.Default.Storage = &certmagic.FileStorage{*certDir}
	}
	//certmagic.DefaultACME.DisableHTTPChallenge = true
	//certmagic.DefaultACME.DisableTLSALPNChallenge = true
	//certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA
	certConfig := certmagic.NewDefault()

	if *newAdminDomain != "" {
		db.SetAdminDomain(*newAdminDomain)
	}

	adminDomain := db.GetAdminDomain()

	if adminDomain == "" {

		adminDomain = getAdminDomain(ip, certConfig)

		db.SetAdminDomain(adminDomain)
	} else {
		err = certConfig.ManageSync([]string{adminDomain})
		if err != nil {
			log.Fatal(err)
		}
		log.Print(fmt.Sprintf("Successfully acquired certificate for admin domain (%s)", adminDomain))
	}

	users := db.GetUsers()
	if len(users) == 0 {
		db.AddUser("admin", true)
		token, err := db.AddToken("admin")
		if err != nil {
			log.Fatal("Failed to initialize admin user")
		}

		log.Println("Admin token: " + token)
		log.Println(fmt.Sprintf("Admin login link: https://%s/login?access_token=%s", adminDomain, token))
	}

	config := &Config{
		WebUiDomain:   adminDomain,
		SshServerPort: *sshServerPort,
		PublicIp:      ip,
	}

	tunMan := NewTunnelManager(config, db, certConfig)

	auth := NewAuth(db)

	api := NewApi(config, db, auth, tunMan)

	webUiHandler := NewWebUiHandler(config, db, api, auth, tunMan)

	httpClient := &http.Client{
		// Don't follow redirects
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	httpListener := NewPassthroughListener()

	p := &Server{db, tunMan, httpClient, httpListener}

	tlsConfig := &tls.Config{
		GetCertificate: certConfig.GetCertificate,
		NextProtos:     []string{"h2", "acme-tls/1"},
	}
	tlsListener := tls.NewListener(httpListener, tlsConfig)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		timestamp := time.Now().Format(time.RFC3339)
		srcIp := strings.Split(r.RemoteAddr, ":")[0]
		fmt.Println(fmt.Sprintf("%s %s %s %s %s", timestamp, srcIp, r.Method, r.Host, r.URL.Path))
		if r.URL.Path == "/domain-callback" {
			r.ParseForm()

			domain := r.Form.Get("domain")
			// TODO: Check request ID
			http.Redirect(w, r, fmt.Sprintf("https://%s/edit-tunnel?domain=%s", config.WebUiDomain, domain), 303)
		} else if r.Host == config.WebUiDomain {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.StripPrefix("/api", api).ServeHTTP(w, r)
			} else {
				webUiHandler.handleWebUiRequest(w, r)
			}
		} else {

			tunnel, exists := db.GetTunnel(r.Host)
			if !exists {
				errMessage := fmt.Sprintf("No tunnel attached to %s", r.Host)
				w.WriteHeader(500)
				io.WriteString(w, errMessage)
				return
			}

			proxyRequest(w, r, tunnel, httpClient, tunnel.TunnelPort)
		}
	})

	go func() {
		if err := http.ListenAndServe(":80", nil); err != nil {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	go http.Serve(tlsListener, nil)

	listener, err := net.Listen("tcp", ":443")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Ready")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err)
			continue
		}

		go p.handleConnection(conn)
	}
}

func (p *Server) handleConnection(clientConn net.Conn) {

	clientHello, clientReader, err := peekClientHello(clientConn)
	if err != nil {
		log.Println("peekClientHello error", err)
		return
	}

	passConn := NewProxyConn(clientConn, clientReader)

	tunnel, exists := p.db.GetTunnel(clientHello.ServerName)

	if exists && (tunnel.TlsTermination == "client" || tunnel.TlsTermination == "passthrough") {
		p.passthroughRequest(passConn, tunnel)
	} else {
		p.httpListener.PassConn(passConn)
	}
}

func (p *Server) passthroughRequest(conn net.Conn, tunnel Tunnel) {

	upstreamAddr := fmt.Sprintf("localhost:%d", tunnel.TunnelPort)
	upstreamConn, err := net.Dial("tcp", upstreamAddr)

	if err != nil {
		log.Print(err)
		return
	}
	defer upstreamConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		io.Copy(conn, upstreamConn)
		conn.(*ProxyConn).CloseWrite()
		wg.Done()
	}()
	go func() {
		io.Copy(upstreamConn, conn)
		upstreamConn.(*net.TCPConn).CloseWrite()
		wg.Done()
	}()

	wg.Wait()
}

func getAdminDomain(ip string, certConfig *certmagic.Config) string {
	action := prompt("\nNo admin domain set. Enter '1' to input manually, or '2' to configure through TakingNames.io\n")

	var adminDomain string

	switch action {
	case "1":
		adminDomain = prompt("\nEnter admin domain:\n")

		err := certConfig.ManageSync([]string{adminDomain})
		if err != nil {
			log.Fatal(err)
		}
	case "2":

		requestId, _ := genRandomCode(32)

		req := &Request{
			RequestId:   requestId,
			RedirectUri: fmt.Sprintf("http://%s/domain-callback", ip),
			Records: []*Record{
				&Record{
					Type:  "A",
					Value: ip,
					TTL:   300,
				},
			},
		}

		jsonBytes, err := json.Marshal(req)
		if err != nil {
			os.Exit(1)
		}

		tnLink := "https://takingnames.io/approve?r=" + url.QueryEscape(string(jsonBytes))

		// Create a temporary web server to handle the callback which contains the domain

		mux := http.NewServeMux()

		server := http.Server{
			Addr:    ":80",
			Handler: mux,
		}

		mux.HandleFunc("/domain-callback", func(w http.ResponseWriter, r *http.Request) {
			r.ParseForm()

			domain := r.Form.Get("domain")
			returnedReqId := r.Form.Get("request-id")

			if domain == "" {
				log.Fatal("Blank domain from TakingNames.io")
			}

			if returnedReqId != requestId {
				log.Fatal("request-id doesn't match")
			}

			adminDomain = domain

			err = certConfig.ManageSync([]string{adminDomain})
			if err != nil {
				log.Fatal(err)
			}

			go func() {
				ctx := context.Background()
				server.Shutdown(ctx)
			}()

			http.Redirect(w, r, fmt.Sprintf("https://%s", adminDomain), 303)
		})

		fmt.Println("Use the link below to select a domain:\n\n" + tnLink)

		server.ListenAndServe()

	default:
		log.Fatal("Invalid option")
	}

	return adminDomain
}

func prompt(promptText string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(promptText)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}
