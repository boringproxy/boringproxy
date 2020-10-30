package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/caddyserver/certmagic"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type BoringProxyConfig struct {
	WebUiDomain string `json:"webui_domain"`
}

type SmtpConfig struct {
	Server   string
	Port     int
	Username string
	Password string
}

type BoringProxy struct {
	db         *Database
	tunMan     *TunnelManager
	httpClient *http.Client
}

func Listen() {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	adminDomain := flagSet.String("admin-domain", "", "Admin Domain")
	flagSet.Parse(os.Args[2:])

	webUiDomain := *adminDomain

	if *adminDomain == "" {
		reader := bufio.NewReader(os.Stdin)
		log.Print("Enter Admin Domain: ")
		text, _ := reader.ReadString('\n')
		webUiDomain = strings.TrimSpace(text)
	}

	config := &BoringProxyConfig{WebUiDomain: webUiDomain}

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
	http.Handle("/api/", http.StripPrefix("/api", api))

	webUiHandler := NewWebUiHandler(config, db, api, auth, tunMan)

	httpClient := &http.Client{
		// Don't follow redirects
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	p := &BoringProxy{db, tunMan, httpClient}

	tlsConfig := &tls.Config{
		GetCertificate: certConfig.GetCertificate,
		NextProtos:     []string{"h2", "acme-tls/1"},
	}
	tlsListener, err := tls.Listen("tcp", ":443", tlsConfig)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		timestamp := time.Now().Format(time.RFC3339)
		srcIp := strings.Split(r.RemoteAddr, ":")[0]
		fmt.Println(fmt.Sprintf("%s %s %s %s %s", timestamp, srcIp, r.Method, r.Host, r.URL.Path))
		if r.Host == config.WebUiDomain {
			webUiHandler.handleWebUiRequest(w, r)
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

	http.Serve(tlsListener, nil)
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

	upstreamReq, err := http.NewRequest(r.Method, upstreamUrl, r.Body)
	if err != nil {
		errMessage := fmt.Sprintf("%s", err)
		w.WriteHeader(500)
		io.WriteString(w, errMessage)
		return
	}

	upstreamReq.Header = downstreamReqHeaders

	upstreamReq.Header["X-Forwarded-Host"] = []string{r.Host}

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
