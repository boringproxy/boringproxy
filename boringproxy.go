package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/caddyserver/certmagic"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type BoringProxyConfig struct {
	WebUiDomain   string `json:"webui_domain"`
	SshServerPort int    `json:"ssh_server_port"`
}

type SmtpConfig struct {
	Server   string
	Port     int
	Username string
	Password string
}

type BoringProxy struct {
	db           *Database
	tunMan       *TunnelManager
	httpClient   *http.Client
	httpListener *PassthroughListener
}

func Listen() {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	adminDomain := flagSet.String("admin-domain", "", "Admin Domain")
	sshServerPort := flagSet.Int("ssh-server-port", 22, "SSH Server Port")
	flagSet.Parse(os.Args[2:])

	webUiDomain := *adminDomain

	if *adminDomain == "" {
		reader := bufio.NewReader(os.Stdin)
		log.Print("Enter Admin Domain: ")
		text, _ := reader.ReadString('\n')
		webUiDomain = strings.TrimSpace(text)
	}

	config := &BoringProxyConfig{
		WebUiDomain:   webUiDomain,
		SshServerPort: *sshServerPort,
	}

	certmagic.DefaultACME.DisableHTTPChallenge = true
	//certmagic.DefaultACME.DisableTLSALPNChallenge = true
	//certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA
	certConfig := certmagic.NewDefault()

	err := certConfig.ManageSync([]string{config.WebUiDomain})
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Successfully acquired admin certificate")

	db, err := NewDatabase()
	if err != nil {
		log.Fatal(err)
	}

	users := db.GetUsers()
	if len(users) == 0 {
		db.AddUser("admin", true)
		token, err := db.AddToken("admin")
		if err != nil {
			log.Fatal("Failed to initialize admin user")
		}

		log.Println("Admin token: " + token)
		log.Println(fmt.Sprintf("Admin login link: https://%s/login?access_token=%s", webUiDomain, token))
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

	p := &BoringProxy{db, tunMan, httpClient, httpListener}

	tlsConfig := &tls.Config{
		GetCertificate: certConfig.GetCertificate,
		NextProtos:     []string{"h2", "acme-tls/1"},
	}
	tlsListener := tls.NewListener(httpListener, tlsConfig)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		timestamp := time.Now().Format(time.RFC3339)
		srcIp := strings.Split(r.RemoteAddr, ":")[0]
		fmt.Println(fmt.Sprintf("%s %s %s %s %s", timestamp, srcIp, r.Method, r.Host, r.URL.Path))
		if r.Host == config.WebUiDomain {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.StripPrefix("/api", api).ServeHTTP(w, r)
			} else {
				webUiHandler.handleWebUiRequest(w, r)
			}
		} else {
			p.proxyRequest(w, r)
		}
	})

	// taken from: https://stackoverflow.com/a/37537134/943814
	go func() {
		if err := http.ListenAndServe(":80", http.HandlerFunc(redirectTLS)); err != nil {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	go http.Serve(tlsListener, nil)

	listener, err := net.Listen("tcp", ":443")
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err)
			continue
		}

		go p.handleConnection(conn)
	}
}

func (p *BoringProxy) handleConnection(clientConn net.Conn) {

	clientHello, clientReader, err := peekClientHello(clientConn)
	if err != nil {
		log.Println("peekClientHello error", err)
		return
	}

	passConn := NewProxyConn(clientConn, clientReader)

	tunnel, exists := p.db.GetTunnel(clientHello.ServerName)

	if exists && tunnel.TlsPassthrough {
		p.passthroughRequest(passConn, tunnel)
	} else {
		p.httpListener.PassConn(passConn)
	}
}

func (p *BoringProxy) passthroughRequest(conn net.Conn, tunnel Tunnel) {

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

func (p *BoringProxy) proxyRequest(w http.ResponseWriter, r *http.Request) {

	tunnel, exists := p.db.GetTunnel(r.Host)
	if !exists {
		errMessage := fmt.Sprintf("No tunnel attached to %s", r.Host)
		w.WriteHeader(500)
		io.WriteString(w, errMessage)
		return
	}

	if tunnel.AuthUsername != "" || tunnel.AuthPassword != "" {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header()["WWW-Authenticate"] = []string{"Basic"}
			w.WriteHeader(401)
			return
		}

		if username != tunnel.AuthUsername || password != tunnel.AuthPassword {
			w.Header()["WWW-Authenticate"] = []string{"Basic"}
			w.WriteHeader(401)
			// TODO: should probably use a better form of rate limiting
			time.Sleep(2 * time.Second)
			return
		}
	}

	downstreamReqHeaders := r.Header.Clone()

	upstreamAddr := fmt.Sprintf("localhost:%d", tunnel.TunnelPort)
	upstreamUrl := fmt.Sprintf("http://%s%s", upstreamAddr, r.URL.RequestURI())

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		errMessage := fmt.Sprintf("%s", err)
		w.WriteHeader(500)
		io.WriteString(w, errMessage)
		return
	}

	upstreamReq, err := http.NewRequest(r.Method, upstreamUrl, bytes.NewReader(body))
	if err != nil {
		errMessage := fmt.Sprintf("%s", err)
		w.WriteHeader(500)
		io.WriteString(w, errMessage)
		return
	}

	upstreamReq.Header = downstreamReqHeaders

	upstreamReq.Header["X-Forwarded-Host"] = []string{r.Host}
	upstreamReq.Host = fmt.Sprintf("%s:%d", tunnel.ClientAddress, tunnel.ClientPort)

	upstreamRes, err := p.httpClient.Do(upstreamReq)
	if err != nil {
		errMessage := fmt.Sprintf("%s", err)
		w.WriteHeader(502)
		io.WriteString(w, errMessage)
		return
	}
	defer upstreamRes.Body.Close()

	downstreamResHeaders := w.Header()

	for k, v := range upstreamRes.Header {
		downstreamResHeaders[k] = v
	}

	w.WriteHeader(upstreamRes.StatusCode)
	io.Copy(w, upstreamRes.Body)
}

func redirectTLS(w http.ResponseWriter, r *http.Request) {
	url := fmt.Sprintf("https://%s:443%s", r.Host, r.RequestURI)
	http.Redirect(w, r, url, http.StatusMovedPermanently)
}
