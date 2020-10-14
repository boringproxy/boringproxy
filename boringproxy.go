package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/caddyserver/certmagic"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

type BoringProxyConfig struct {
	WebUiDomain string      `json:"webui_domain"`
	Smtp        *SmtpConfig `json:"smtp"`
}

type SmtpConfig struct {
	Server   string
	Port     int
	Username string
	Password string
}

type BoringProxy struct {
	tunMan     *TunnelManager
	httpClient *http.Client
}

func Listen() {

	config := &BoringProxyConfig{}

	configJson, err := ioutil.ReadFile("boringproxy_config.json")
	if err != nil {
		log.Println(err)
	}

	err = json.Unmarshal(configJson, config)
	if err != nil {
		log.Println(err)
		config = &BoringProxyConfig{}
	}

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
	}

	auth := NewAuth(db)

	certmagic.DefaultACME.DisableHTTPChallenge = true
	//certmagic.DefaultACME.DisableTLSALPNChallenge = true
	//certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA
	certConfig := certmagic.NewDefault()

	tunMan := NewTunnelManager(config, db, certConfig)

	err = certConfig.ManageSync([]string{config.WebUiDomain})
	if err != nil {
		log.Println("CertMagic error")
		log.Println(err)
	}

	api := NewApi(config, db, auth, tunMan)
	http.Handle("/api/", http.StripPrefix("/api", api))

	webUiHandler := NewWebUiHandler(config, db, api, auth, tunMan)

	httpClient := &http.Client{}

	p := &BoringProxy{tunMan, httpClient}

	tlsConfig := &tls.Config{
		GetCertificate: certConfig.GetCertificate,
		NextProtos:     []string{"h2", "acme-tls/1"},
	}
	tlsListener, err := tls.Listen("tcp", ":443", tlsConfig)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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

	log.Println("BoringProxy ready")

	http.Serve(tlsListener, nil)
}

func (p *BoringProxy) proxyRequest(w http.ResponseWriter, r *http.Request) {

	port, err := p.tunMan.GetPort(r.Host)
	if err != nil {
		log.Print(err)
		errMessage := fmt.Sprintf("No tunnel attached to %s", r.Host)
		w.WriteHeader(500)
		io.WriteString(w, errMessage)
		return
	}

	downstreamReqHeaders := r.Header.Clone()

	upstreamAddr := fmt.Sprintf("localhost:%d", port)
	upstreamUrl := fmt.Sprintf("http://%s%s", upstreamAddr, r.URL.RequestURI())

	upstreamReq, err := http.NewRequest(r.Method, upstreamUrl, r.Body)
	if err != nil {
		log.Print(err)
		errMessage := fmt.Sprintf("%s", err)
		w.WriteHeader(500)
		io.WriteString(w, errMessage)
		return
	}

	upstreamReq.Header = downstreamReqHeaders

	upstreamReq.Header["X-Forwarded-Host"] = []string{r.Host}

	upstreamRes, err := p.httpClient.Do(upstreamReq)
	if err != nil {
		log.Print(err)
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
	log.Println("redir", url)
	http.Redirect(w, r, url, http.StatusMovedPermanently)
}
