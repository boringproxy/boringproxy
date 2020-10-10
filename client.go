package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
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
	cancelFuncs      map[string]context.CancelFunc
	cancelFuncsMutex *sync.Mutex
}

func NewBoringProxyClient() *BoringProxyClient {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	server := flagSet.String("server", "", "boringproxy server")
	token := flagSet.String("token", "", "Access token")
	name := flagSet.String("client-name", "", "Client name")
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
		cancelFuncs:      cancelFuncs,
		cancelFuncsMutex: cancelFuncsMutex,
	}
}

func (c *BoringProxyClient) RunPuppetClient() {

	for {
		c.PollTunnels()
		time.Sleep(2 * time.Second)
	}
}

func (c *BoringProxyClient) PollTunnels() {
	url := fmt.Sprintf("https://%s/api/tunnels?client-name=%s", c.server, c.clientName)

	listenReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("Failed making request", err)
	}

	if len(c.token) > 0 {
		listenReq.Header.Add("Authorization", "bearer "+c.token)
	}

	resp, err := c.httpClient.Do(listenReq)
	if err != nil {
		log.Fatal("Failed listen request", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Fatal("Failed to listen (not 200 status)")
	}

	etag := resp.Header["Etag"][0]

	if etag != c.previousEtag {

		body, err := ioutil.ReadAll(resp.Body)

		tunnels := make(map[string]Tunnel)

		err = json.Unmarshal(body, &tunnels)
		if err != nil {
			log.Fatal("Failed to parse response", err)
		}

		c.SyncTunnels(tunnels)

		c.previousEtag = etag
	}

}

func (c *BoringProxyClient) SyncTunnels(serverTunnels map[string]Tunnel) {
	fmt.Println("SyncTunnels")

	// update tunnels to match server
	for k, newTun := range serverTunnels {

		tun, exists := c.tunnels[k]
		if !exists {
			log.Println("New tunnel", k)
			c.tunnels[k] = newTun
			cancel := c.BoreTunnel(newTun)
			c.cancelFuncs[k] = cancel
		} else if newTun != tun {
			log.Println("Restart tunnel", k)
			c.cancelFuncs[k]()
			cancel := c.BoreTunnel(newTun)
			c.cancelFuncs[k] = cancel
		}
	}

	// delete any tunnels that no longer exist on server
	for k, _ := range c.tunnels {
		_, exists := serverTunnels[k]
		if !exists {
			log.Println("Kill tunnel", k)
			c.cancelFuncs[k]()
			delete(c.tunnels, k)
			delete(c.cancelFuncs, k)
		}
	}
}

func (c *BoringProxyClient) BoreTunnel(tun Tunnel) context.CancelFunc {

	//log.Println("BoreTunnel", tun)

	privKeyFile, err := ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}

	if _, err := privKeyFile.Write([]byte(tun.TunnelPrivateKey)); err != nil {
		log.Fatal(err)
	}
	if err := privKeyFile.Close(); err != nil {
		log.Fatal(err)
	}

	tunnelSpec := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", tun.TunnelPort, tun.ClientPort)
	sshLogin := fmt.Sprintf("%s@%s", tun.Username, tun.ServerAddress)
	serverPortStr := fmt.Sprintf("%d", tun.ServerPort)

	ctx, cancelFunc := context.WithCancel(context.Background())

	privKeyPath := privKeyFile.Name()

	go func() {
		// TODO: Clean up private key files on exit
		defer os.Remove(privKeyPath)
		fmt.Println(privKeyPath, tunnelSpec, sshLogin, serverPortStr)
		cmd := exec.CommandContext(ctx, "ssh", "-i", privKeyPath, "-NR", tunnelSpec, sshLogin, "-p", serverPortStr)
		err = cmd.Run()
		if err != nil {
			log.Print(err)
		}
	}()

	return cancelFunc
}

func (c *BoringProxyClient) Run() {
	log.Println("Run client")

	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	server := flagSet.String("server", "", "boringproxy server")
	token := flagSet.String("token", "", "Access token")
	domain := flagSet.String("domain", "", "Tunnel domain")
	port := flagSet.Int("port", 9001, "Local port for tunnel")
	flagSet.Parse(os.Args[2:])

	httpClient := &http.Client{}

	url := fmt.Sprintf("https://%s/api/tunnels?domain=%s", *server, *domain)
	makeTunReq, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Fatal("Failed making request", err)
	}

	if len(*token) > 0 {
		makeTunReq.Header.Add("Authorization", "bearer "+*token)
	}

	resp, err := httpClient.Do(makeTunReq)
	if err != nil {
		log.Fatal("Failed make tunnel request", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		log.Fatal("Failed to create tunnel: " + string(body))
	}

	tunnel := &Tunnel{}

	err = json.Unmarshal(body, &tunnel)
	if err != nil {
		log.Fatal("Failed to parse response", err)
	}

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
	defer client.Close()

	tunnelAddr := fmt.Sprintf("127.0.0.1:%d", tunnel.TunnelPort)
	l, err := client.Listen("tcp", tunnelAddr)
	if err != nil {
		log.Fatal("unable to register tcp forward: ", err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go c.handleConnection(conn, *port)
	}
}

func (c *BoringProxyClient) handleConnection(conn net.Conn, port int) {
	log.Println("new conn")

	defer conn.Close()

	upstreamConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		log.Print(err)
		return
	}
	defer upstreamConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		io.Copy(conn, upstreamConn)
		wg.Done()
	}()
	go func() {
		io.Copy(upstreamConn, conn)
		wg.Done()
	}()

	wg.Wait()
}
