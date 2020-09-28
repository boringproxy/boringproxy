package main

import (
        "fmt"
	"log"
	"net"
	"net/http"
	"crypto/tls"
        "io"
        "sync"
        "strconv"
        "encoding/json"
        "io/ioutil"
        "github.com/caddyserver/certmagic"
)


type BoringProxyConfig struct {
        AdminDomain string `json:"admin_domain"`
        Smtp *SmtpConfig `json:"smtp"`
}

type SmtpConfig struct {
        Server string
        Port int
        Username string
        Password string
}


type BoringProxy struct {
        config *BoringProxyConfig
        tunMan *TunnelManager
        adminListener *AdminListener
        certConfig *certmagic.Config
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


        p := &BoringProxy{config, tunMan, adminListener, certConfig}

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
        if r.URL.Path == "/tunnels" {
                p.handleTunnels(w, r)
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

func (p *BoringProxy) handleCreateTunnel(w http.ResponseWriter, r *http.Request) {

        query := r.URL.Query()

        if len(query["host"]) != 1 {
                w.WriteHeader(400)
                w.Write([]byte("Invalid host parameter"))
                return
        }
        host := query["host"][0]

        if len(query["port"]) != 1 {
                w.WriteHeader(400)
                w.Write([]byte("Invalid port parameter"))
                return
        }

        port, err := strconv.Atoi(query["port"][0])
        if err != nil {
                w.WriteHeader(400)
                w.Write([]byte("Invalid port parameter"))
                return
        }

        p.tunMan.SetTunnel(host, port)
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
