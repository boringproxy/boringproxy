package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/caddyserver/certmagic"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
)

type BoringProxyConfig struct {
	AdminDomain string      `json:"admin_domain"`
	Smtp        *SmtpConfig `json:"smtp"`
}

type SmtpConfig struct {
	Server   string
	Port     int
	Username string
	Password string
}

type BoringProxy struct {
	config        *BoringProxyConfig
	auth          *Auth
	tunMan        *TunnelManager
	adminListener *AdminListener
	certConfig    *certmagic.Config
}

func NewBoringProxy() *BoringProxy {

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

	//certmagic.DefaultACME.DisableHTTPChallenge = true
	certmagic.DefaultACME.DisableTLSALPNChallenge = true
	//certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA
	certConfig := certmagic.NewDefault()

	tunMan := NewTunnelManager(certConfig)
	adminListener := NewAdminListener()

	err = certConfig.ManageSync([]string{config.AdminDomain})
	if err != nil {
		log.Println("CertMagic error")
		log.Println(err)
	}

	auth := NewAuth()

	p := &BoringProxy{config, auth, tunMan, adminListener, certConfig}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p.handleAdminRequest(w, r)
	})

	api := NewApi(config, auth, tunMan)
	http.Handle("/api/", http.StripPrefix("/api", api))

	go http.Serve(adminListener, nil)

	log.Println("BoringProxy ready")

	return p
}

func (p *BoringProxy) Run() {

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
	// TODO: does this need to be closed manually, or is it handled when decryptedConn is closed?
	//defer clientConn.Close()

        log.Println("handleConnection")

	var serverName string

	decryptedConn := tls.Server(clientConn, &tls.Config{
		GetCertificate: func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {

			serverName = clientHello.ServerName

			return p.certConfig.GetCertificate(clientHello)
		},
	})
	//defer decryptedConn.Close()

	// Need to manually do handshake to ensure serverName is populated by this point. Usually Handshake()
	// is automatically called on first read/write
	decryptedConn.Handshake()

	if serverName == p.config.AdminDomain {
		p.handleAdminConnection(decryptedConn)
	} else {
		p.handleTunnelConnection(decryptedConn, serverName)
	}
}

func (p *BoringProxy) handleAdminConnection(decryptedConn net.Conn) {
	p.adminListener.connChan <- decryptedConn
}

func (p *BoringProxy) handleTunnelConnection(decryptedConn net.Conn, serverName string) {

	defer decryptedConn.Close()

	port, err := p.tunMan.GetPort(serverName)
	if err != nil {
		log.Print(err)
		errMessage := fmt.Sprintf("HTTP/1.1 500 Internal server error\n\nNo tunnel attached to %s", serverName)
		decryptedConn.Write([]byte(errMessage))
		return
	}

	upstreamAddr := fmt.Sprintf("127.0.0.1:%d", port)

	upstreamConn, err := net.Dial("tcp", upstreamAddr)
	if err != nil {
		log.Print(err)
		return
	}
	defer upstreamConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		io.Copy(decryptedConn, upstreamConn)
		//decryptedConn.(*net.TCPConn).CloseWrite()
		wg.Done()
	}()
	go func() {
		io.Copy(upstreamConn, decryptedConn)
		//upstreamConn.(*net.TCPConn).CloseWrite()
		wg.Done()
	}()

	wg.Wait()
}
