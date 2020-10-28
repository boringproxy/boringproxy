package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

type BoringProxyClient struct {
	httpClient       *http.Client
	tunnels          map[string]Tunnel
	previousEtag     string
	server           string
	token            string
	clientName       string
	user             string
	cancelFuncs      map[string]context.CancelFunc
	cancelFuncsMutex *sync.Mutex
}

func NewBoringProxyClient() *BoringProxyClient {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	server := flagSet.String("server", "", "boringproxy server")
	token := flagSet.String("token", "", "Access token")
	name := flagSet.String("client-name", "", "Client name")
	user := flagSet.String("user", "admin", "user")
	flagSet.Parse(os.Args[2:])

	httpClient := &http.Client{}
	tunnels := make(map[string]Tunnel)
	cancelFuncs := make(map[string]context.CancelFunc)
	cancelFuncsMutex := &sync.Mutex{}

	return &BoringProxyClient{
		httpClient:       httpClient,
		tunnels:          tunnels,
		previousEtag:     "",
		server:           *server,
		token:            *token,
		clientName:       *name,
		user:             *user,
		cancelFuncs:      cancelFuncs,
		cancelFuncsMutex: cancelFuncsMutex,
	}
}

func (c *BoringProxyClient) RunPuppetClient() {

	url := fmt.Sprintf("https://%s/api/users/%s/clients/%s", c.server, c.user, c.clientName)
	clientReq, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		log.Fatal("Failed to PUT client")
	}
	if len(c.token) > 0 {
		clientReq.Header.Add("Authorization", "bearer "+c.token)
	}
	resp, err := c.httpClient.Do(clientReq)
	if err != nil {
		log.Fatal("Failed to PUT client")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Fatal("Failed to PUT client")
	}

	for {
		err := c.PollTunnels()
		if err != nil {
			log.Print(err)
		}
		time.Sleep(2 * time.Second)
	}
}

func (c *BoringProxyClient) PollTunnels() error {

	//log.Println("PollTunnels")

	url := fmt.Sprintf("https://%s/api/tunnels?client-name=%s", c.server, c.clientName)

	listenReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	if len(c.token) > 0 {
		listenReq.Header.Add("Authorization", "bearer "+c.token)
	}

	resp, err := c.httpClient.Do(listenReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("Failed to listen (not 200 status)")
	}

	etag := resp.Header["Etag"][0]

	if etag != c.previousEtag {

		body, err := ioutil.ReadAll(resp.Body)

		tunnels := make(map[string]Tunnel)

		err = json.Unmarshal(body, &tunnels)
		if err != nil {
			return err
		}

		c.SyncTunnels(tunnels)

		c.previousEtag = etag
	}

	return nil
}

func (c *BoringProxyClient) SyncTunnels(serverTunnels map[string]Tunnel) {
	log.Println("SyncTunnels")

	// update tunnels to match server
	for k, newTun := range serverTunnels {

		tun, exists := c.tunnels[k]
		if !exists {
			log.Println("New tunnel", k)
			c.tunnels[k] = newTun
			cancel := c.BoreTunnel(newTun)

			c.cancelFuncsMutex.Lock()
			c.cancelFuncs[k] = cancel
			c.cancelFuncsMutex.Unlock()

		} else if newTun != tun {
			log.Println("Restart tunnel", k)

			c.cancelFuncsMutex.Lock()
			c.cancelFuncs[k]()
			c.cancelFuncsMutex.Unlock()

			cancel := c.BoreTunnel(newTun)

			c.cancelFuncsMutex.Lock()
			c.cancelFuncs[k] = cancel
			c.cancelFuncsMutex.Unlock()
		}
	}

	// delete any tunnels that no longer exist on server
	for k, _ := range c.tunnels {
		_, exists := serverTunnels[k]
		if !exists {
			log.Println("Kill tunnel", k)
			c.cancelFuncsMutex.Lock()
			c.cancelFuncs[k]()
			delete(c.cancelFuncs, k)
			c.cancelFuncsMutex.Unlock()
			delete(c.tunnels, k)
		}
	}
}

func (c *BoringProxyClient) BoreTunnel(tunnel Tunnel) context.CancelFunc {

	log.Println("BoreTunnel", tunnel.Domain)

	ctx, cancelFunc := context.WithCancel(context.Background())

	go func() {
		signer, err := ssh.ParsePrivateKey([]byte(tunnel.TunnelPrivateKey))
		if err != nil {
			log.Fatalf("unable to parse private key: %v", err)
		}

		//var hostKey ssh.PublicKey

		config := &ssh.ClientConfig{
			User: tunnel.Username,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			//HostKeyCallback: ssh.FixedHostKey(hostKey),
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}

		sshHost := fmt.Sprintf("%s:%d", tunnel.ServerAddress, tunnel.ServerPort)
		client, err := ssh.Dial("tcp", sshHost, config)
		if err != nil {
			log.Fatal("Failed to dial: ", err)
		}
		//defer client.Close()

		bindAddr := "127.0.0.1"
		if tunnel.AllowExternalTcp {
			bindAddr = "0.0.0.0"
		}
		tunnelAddr := fmt.Sprintf("%s:%d", bindAddr, tunnel.TunnelPort)
		listener, err := client.Listen("tcp", tunnelAddr)
		if err != nil {
			log.Fatal("unable to register tcp forward: ", err)
		}
		//defer listener.Close()

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					// TODO: Currently assuming an error means the
					// tunnel was manually deleted, but there
					// could be other errors that we should be
					// attempting to recover from rather than
					// breaking.
					break
					//continue
				}
				go c.handleConnection(conn, tunnel.ClientAddress, tunnel.ClientPort)
			}
		}()

		<-ctx.Done()
		listener.Close()
		client.Close()
	}()

	return cancelFunc
}

func (c *BoringProxyClient) handleConnection(conn net.Conn, addr string, port int) {

	defer conn.Close()

	upstreamConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", addr, port))
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
		upstreamConn.(*net.TCPConn).CloseWrite()
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
