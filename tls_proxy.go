package boringproxy

import (
	//"errors"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/caddyserver/certmagic"
)

func ProxyTcp(conn net.Conn, addr string, port int, useTls bool, certConfig *certmagic.Config) error {

	if useTls {
		tlsConfig := &tls.Config{
			GetCertificate: certConfig.GetCertificate,
		}

		tlsConfig.NextProtos = append([]string{"http/1.1", "h2", "acme-tls/1"}, tlsConfig.NextProtos...)

		tlsConn := tls.Server(conn, tlsConfig)

		tlsConn.Handshake()
		if tlsConn.ConnectionState().NegotiatedProtocol == "acme-tls/1" {
			tlsConn.Close()
			return nil
		}

		go handleConnection(tlsConn, addr, port)
	} else {
		go handleConnection(conn, addr, port)
	}

	return nil
}

func handleConnection(conn net.Conn, upstreamAddr string, port int) {

	defer conn.Close()

	useTls := false
	addr := upstreamAddr

	if strings.HasPrefix(upstreamAddr, "https://") {
		addr = upstreamAddr[len("https://"):]
		useTls = true
	}

	var upstreamConn net.Conn
	var err error

	if useTls {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		upstreamConn, err = tls.Dial("tcp", fmt.Sprintf("%s:%d", addr, port), tlsConfig)
	} else {
		upstreamConn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", addr, port))
	}

	if err != nil {
		log.Print(err)
		return
	}

	defer upstreamConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// Copy request to upstream
	go func() {
		_, err := io.Copy(upstreamConn, conn)
		if err != nil {
			log.Println(err.Error())
		}

		if c, ok := upstreamConn.(*net.TCPConn); ok {
			c.CloseWrite()
		} else if c, ok := upstreamConn.(*tls.Conn); ok {
			c.CloseWrite()
		}

		wg.Done()
	}()

	// Copy response to downstream
	go func() {
		_, err := io.Copy(conn, upstreamConn)
		//conn.(*net.TCPConn).CloseWrite()
		if err != nil {
			log.Println(err.Error())
		}
		// TODO: I added this to fix a bug where the copy to
		// upstreamConn was never closing, even though the copy to
		// conn was. It seems related to persistent connections going
		// idle and upstream closing the connection. I'm a bit worried
		// this might not be thread safe.
		conn.Close()
		wg.Done()
	}()

	wg.Wait()
}
