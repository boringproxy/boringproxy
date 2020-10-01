package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
        "html/template"
	"strconv"
	"sync"
	"github.com/caddyserver/certmagic"
        "github.com/GeertJohan/go.rice"
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
        sshServer     *SshServer
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

        sshServer := NewSshServer()

	p := &BoringProxy{config, auth, tunMan, adminListener, certConfig, sshServer}

	http.HandleFunc("/", p.handleAdminRequest)
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

func (p *BoringProxy) handleAdminRequest(w http.ResponseWriter, r *http.Request) {

	switch r.URL.Path {
	case "/login":
		p.handleLogin(w, r)
	case "/":
                box, err := rice.FindBox("webui")
                if err != nil {
			w.WriteHeader(500)
                        io.WriteString(w, "Error opening webui")
			return
                }

		token, err := extractToken("access_token", r)
		if err != nil {

                        loginTemplate, err := box.String("login.tmpl")
                        if err != nil {
                                log.Println(err)
                                w.WriteHeader(500)
                                io.WriteString(w, "Error reading login.tmpl")
                                return
                        }

			w.WriteHeader(401)
                        io.WriteString(w, loginTemplate)
			return
		}

		if !p.auth.Authorized(token) {
			w.WriteHeader(403)
			w.Write([]byte("Not authorized"))
			return
		}


                indexTemplate, err := box.String("index.tmpl")
                if err != nil {
			w.WriteHeader(500)
                        io.WriteString(w, "Error reading index.tmpl")
			return
                }

                tmpl, err := template.New("test").Parse(indexTemplate)
                if err != nil {
			w.WriteHeader(500)
                        log.Println(err)
                        io.WriteString(w, "Error compiling index.tmpl")
			return
                }


                tmpl.Execute(w, p.tunMan.tunnels)

                //io.WriteString(w, indexTemplate)

	case "/tunnels":

		token, err := extractToken("access_token", r)
		if err != nil {
			w.WriteHeader(401)
			w.Write([]byte("No token provided"))
			return
		}

		if !p.auth.Authorized(token) {
			w.WriteHeader(403)
			w.Write([]byte("Not authorized"))
			return
		}

		p.handleTunnels(w, r)

	case "/delete-tunnel":
                token, err := extractToken("access_token", r)
		if err != nil {
			w.WriteHeader(401)
			w.Write([]byte("No token provided"))
			return
		}

		if !p.auth.Authorized(token) {
			w.WriteHeader(403)
			w.Write([]byte("Not authorized"))
			return
		}

                r.ParseForm()

		if len(r.Form["host"]) != 1 {
			w.WriteHeader(400)
			w.Write([]byte("Invalid host parameter"))
			return
		}
		host := r.Form["host"][0]

		p.tunMan.DeleteTunnel(host)

                http.Redirect(w, r, "/", 307)
	default:
		w.WriteHeader(400)
		w.Write([]byte("Invalid endpoint"))
		return
	}
}

func (p *BoringProxy) handleTunnels(w http.ResponseWriter, r *http.Request) {

	query := r.URL.Query()

	if r.Method == "GET" {
		body, err := json.Marshal(p.tunMan.tunnels)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("Error encoding tunnels"))
			return
		}
		w.Write([]byte(body))
	} else if r.Method == "POST" {
		p.handleCreateTunnel(w, r)
	} else if r.Method == "DELETE" {
		if len(query["host"]) != 1 {
			w.WriteHeader(400)
			w.Write([]byte("Invalid host parameter"))
			return
		}
		host := query["host"][0]

		p.tunMan.DeleteTunnel(host)
	}
}

func (p *BoringProxy) handleLogin(w http.ResponseWriter, r *http.Request) {

        switch r.Method {
        case "GET":
                query := r.URL.Query()
                key, exists := query["key"]

                if !exists {
                        w.WriteHeader(400)
                        fmt.Fprintf(w, "Must provide key for verification")
                        return
                }

                token, err := p.auth.Verify(key[0])

                if err != nil {
                        w.WriteHeader(400)
                        fmt.Fprintf(w, "Invalid key")
                        return
                }

                cookie := &http.Cookie{Name: "access_token", Value: token, Secure: true, HttpOnly: true}
                http.SetCookie(w, cookie)

                http.Redirect(w, r, "/", 307)


        case "POST":

                r.ParseForm()

                toEmail, ok := r.Form["email"]

                if !ok {
                        w.WriteHeader(400)
                        w.Write([]byte("Email required for login"))
                        return
                }

                // run in goroutine because it can take some time to send the
                // email
                go p.auth.Login(toEmail[0], p.config)

                io.WriteString(w, "Check your email to finish logging in")
        default:
		w.WriteHeader(405)
		w.Write([]byte("Invalid method for login"))
	}
}

func (p *BoringProxy) handleCreateTunnel(w http.ResponseWriter, r *http.Request) {

        r.ParseForm()

	if len(r.Form["host"]) != 1 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid host parameter"))
		return
	}
	host := r.Form["host"][0]

	if len(r.Form["port"]) != 1 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid port parameter"))
		return
	}

	port, err := strconv.Atoi(r.Form["port"][0])
	if err != nil {
		w.WriteHeader(400)
		w.Write([]byte("Invalid port parameter"))
		return
	}

	err = p.tunMan.SetTunnel(host, port)
        if err != nil {
		w.WriteHeader(400)
                io.WriteString(w, "Failed to get cert. Ensure your domain is valid")
		return
        }

        http.Redirect(w, r, "/", 303)
}

func (p *BoringProxy) handleConnection(clientConn net.Conn) {
	// TODO: does this need to be closed manually, or is it handled when decryptedConn is closed?
	//defer clientConn.Close()

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
