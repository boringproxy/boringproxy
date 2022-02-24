package boringproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/mdp/qrterminal/v3"

	"github.com/takingnames/namedrop-go"
)

type Config struct {
	SshServerPort  int    `json:"ssh_server_port"`
	PublicIp       string `json:"public_ip"`
	namedropClient *namedrop.Client
	autoCerts      bool
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

func Listen() {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	newAdminDomain := flagSet.String("admin-domain", "", "Admin Domain")
	sshServerPort := flagSet.Int("ssh-server-port", 22, "SSH Server Port")
	dbDir := flagSet.String("db-dir", "", "Database file directory")
	certDir := flagSet.String("cert-dir", "", "TLS cert directory")
	printLogin := flagSet.Bool("print-login", false, "Prints admin login information")
	httpPort := flagSet.Int("http-port", 80, "HTTP (insecure) port")
	httpsPort := flagSet.Int("https-port", 443, "HTTPS (secure) port")
	allowHttp := flagSet.Bool("allow-http", false, "Allow unencrypted (HTTP) requests")
	publicIp := flagSet.String("public-ip", "", "Public IP")
	behindProxy := flagSet.Bool("behind-proxy", false, "Whether we're running behind another reverse proxy")
	acmeEmail := flagSet.String("acme-email", "", "Email for ACME (ie Let's Encrypt)")
	acmeUseStaging := flagSet.Bool("acme-use-staging", false, "Use ACME (ie Let's Encrypt) staging servers")
	acceptCATerms := flagSet.Bool("accept-ca-terms", false, "Automatically accept CA terms")
	customCA := flagSet.String("custom-ca", "", "Custom ACME CA")
	err := flagSet.Parse(os.Args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: parsing flags: %s\n", os.Args[0], err)
	}

	log.Println("Starting up")

	db, err := NewDatabase(*dbDir)
	if err != nil {
		log.Fatal(err)
	}

	namedropClient := namedrop.NewClient(db, db.GetAdminDomain(), "takingnames.io/namedrop")

	var ip string

	if *publicIp != "" {
		ip = *publicIp
	} else {
		ip, err = namedropClient.GetPublicIp()
		if err != nil {
			fmt.Printf("WARNING: Failed to determine public IP: %s\n", err.Error())
		}
	}

	err = namedrop.CheckPublicAddress(ip, *httpPort)
	if err != nil {
		fmt.Printf("WARNING: Failed to access %s:%d from the internet\n", ip, *httpPort)
	}

	err = namedrop.CheckPublicAddress(ip, *httpsPort)
	if err != nil {
		fmt.Printf("WARNING: Failed to access %s:%d from the internet\n", ip, *httpsPort)
	}

	autoCerts := true
	if *httpPort != 80 || *httpsPort != 443 {
		fmt.Printf("WARNING: LetsEncrypt only supports HTTP/HTTPS ports 80/443. You are using %d/%d. Disabling automatic certificate management\n", *httpPort, *httpsPort)
		autoCerts = false
	}

	if *certDir != "" {
		certmagic.Default.Storage = &certmagic.FileStorage{*certDir}
	}
	//certmagic.DefaultACME.DisableHTTPChallenge = true
	//certmagic.DefaultACME.DisableTLSALPNChallenge = true

	if *acmeEmail != "" {
		certmagic.DefaultACME.Email = *acmeEmail
	}

	if *acceptCATerms {
		certmagic.DefaultACME.Agreed = true
		log.Print(fmt.Sprintf("Automatic agreement to CA terms with email (%s)", *acmeEmail))
	}

	if *acmeUseStaging {
		certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA
	}

	if *customCA != "" {
		//	https://smallstep.com/blog/private-acme-server/
		//	example: https://<domain>:<port>/acme/acme/directory
		certmagic.DefaultACME.CA = *customCA
	}

	certConfig := certmagic.NewDefault()

	if *newAdminDomain != "" {
		db.SetAdminDomain(*newAdminDomain)
	}

	adminDomain := db.GetAdminDomain()

	if adminDomain == "" {

		err = setAdminDomain(certConfig, db, namedropClient, autoCerts)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		if autoCerts {
			err = certConfig.ManageSync(context.Background(), []string{adminDomain})
			if err != nil {
				log.Fatal(err)
			}
			log.Print(fmt.Sprintf("Successfully acquired certificate for admin domain (%s)", adminDomain))
		}
	}

	// Add admin user if it doesn't already exist
	users := db.GetUsers()
	if len(users) == 0 {
		db.AddUser("admin", true)
		_, err := db.AddToken("admin", "")
		if err != nil {
			log.Fatal("Failed to initialize admin user")
		}

	}

	if *printLogin {
		tokens := db.GetTokens()

		for token, tokenData := range tokens {
			if tokenData.Owner == "admin" {
				printLoginInfo(token, db.GetAdminDomain())
				break
			}
		}
	}

	config := &Config{
		SshServerPort:  *sshServerPort,
		PublicIp:       ip,
		namedropClient: namedropClient,
		autoCerts:      autoCerts,
	}

	tunMan := NewTunnelManager(config, db, certConfig)

	auth := NewAuth(db)

	api := NewApi(config, db, auth, tunMan)

	webUiHandler := NewWebUiHandler(config, db, api, auth)

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

		remoteIp, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			w.WriteHeader(500)
			io.WriteString(w, err.Error())
			return
		}
		fmt.Println(fmt.Sprintf("%s %s %s %s %s", timestamp, remoteIp, r.Method, r.Host, r.URL.Path))

		hostParts := strings.Split(r.Host, ":")
		hostDomain := hostParts[0]

		if r.URL.Path == "/namedrop/callback" {
			r.ParseForm()

			errorParam := r.Form.Get("error")
			requestId := r.Form.Get("state")
			code := r.Form.Get("code")

			if errorParam != "" {
				db.DeleteDNSRequest(requestId)

				http.Redirect(w, r, "/alert?message=Domain request failed", 303)
				return
			}

			namedropTokenData, err := namedropClient.GetToken(requestId, code)
			if err != nil {
				w.WriteHeader(500)
				io.WriteString(w, err.Error())
				return
			}

			domain := namedropTokenData.Scopes[0].Domain
			host := namedropTokenData.Scopes[0].Host

			recordType := "AAAA"
			if IsIPv4(config.PublicIp) {
				recordType = "A"
			}

			createRecordReq := namedrop.Record{
				Domain: domain,
				Host:   host,
				Type:   recordType,
				Value:  config.PublicIp,
				TTL:    300,
			}

			err = namedropClient.CreateRecord(createRecordReq)
			if err != nil {
				w.WriteHeader(500)
				io.WriteString(w, err.Error())
				return
			}

			fqdn := host + "." + domain

			if db.GetAdminDomain() == "" {
				db.SetAdminDomain(fqdn)
				namedropClient.SetDomain(fqdn)

				if autoCerts {
					// TODO: Might want to get all certs here, not just the admin domain
					err := certConfig.ManageSync(r.Context(), []string{fqdn})
					if err != nil {
						log.Fatal(err)
					}
				}

				url := fmt.Sprintf("https://%s", fqdn)

				// Automatically log using the first found admin token. This is safe to do here
				// because we know that retrieving the admin domain was initiated from the CLI.
				tokens := db.GetTokens()
				for token, tokenData := range tokens {
					if tokenData.Owner == "admin" {
						url = url + "/login?access_token=" + token
						break
					}
				}

				http.Redirect(w, r, url, 303)
			} else {
				adminDomain := db.GetAdminDomain()
				http.Redirect(w, r, fmt.Sprintf("https://%s/edit-tunnel?domain=%s", adminDomain, fqdn), 303)
			}

		} else if hostDomain == db.GetAdminDomain() {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.StripPrefix("/api", api).ServeHTTP(w, r)
			} else {
				webUiHandler.handleWebUiRequest(w, r)
			}
		} else {

			tunnel, exists := db.GetTunnel(hostDomain)
			if !exists {
				errMessage := fmt.Sprintf("No tunnel attached to %s", hostDomain)
				w.WriteHeader(500)
				io.WriteString(w, errMessage)
				return
			}

			proxyRequest(w, r, tunnel, httpClient, "localhost", tunnel.TunnelPort, *behindProxy)
		}
	})

	go func() {

		if *allowHttp {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", *httpPort), nil); err != nil {
				log.Fatalf("ListenAndServe error: %v", err)
			}
		} else {
			redirectTLS := func(w http.ResponseWriter, r *http.Request) {
				url := fmt.Sprintf("https://%s:%d%s", r.Host, *httpsPort, r.RequestURI)
				http.Redirect(w, r, url, http.StatusMovedPermanently)
			}

			if err := http.ListenAndServe(fmt.Sprintf(":%d", *httpPort), http.HandlerFunc(redirectTLS)); err != nil {
				log.Fatalf("ListenAndServe error: %v", err)
			}
		}

	}()

	go http.Serve(tlsListener, nil)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *httpsPort))
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

	if exists && (tunnel.TlsTermination == "client" || tunnel.TlsTermination == "passthrough") || tunnel.TlsTermination == "client-tls" {
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

func setAdminDomain(certConfig *certmagic.Config, db *Database, namedropClient *namedrop.Client, autoCerts bool) error {
	action := prompt("\nNo admin domain set. Select an option below:\nEnter '1' to input manually\nEnter '2' to configure through TakingNames.io\n")
	switch action {
	case "1":
		adminDomain := prompt("\nEnter admin domain:\n")

		if autoCerts {
			err := certConfig.ManageSync(context.Background(), []string{adminDomain})
			if err != nil {
				log.Fatal(err)
			}
		}

		db.SetAdminDomain(adminDomain)
	case "2":

		log.Println("Get bootstrap domain")

		namedropLink, err := namedropClient.BootstrapLink()
		if err != nil {
			log.Fatal(err)
		}

		qrterminal.GenerateHalfBlock(namedropLink, qrterminal.L, os.Stdout)
		fmt.Println("Use the link below or scan the QR code above to select an admin domain:\n")
		fmt.Printf("%s\n\n", namedropLink)

	default:
		log.Fatal("Invalid option")
	}

	return nil
}

func prompt(promptText string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(promptText)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func printLoginInfo(token, adminDomain string) {
	log.Println("Admin token: " + token)
	url := fmt.Sprintf("https://%s/login?access_token=%s", adminDomain, token)
	log.Println(fmt.Sprintf("Admin login link: %s", url))
	qrterminal.GenerateHalfBlock(url, qrterminal.L, os.Stdout)
}

// Taken from https://stackoverflow.com/a/48519490/943814
func IsIPv4(address string) bool {
	return strings.Count(address, ":") < 2
}
