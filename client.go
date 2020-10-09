package main

import (
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
	"os/signal"
	"sync"
)

type BoringProxyClient struct {
}

func NewBoringProxyClient() *BoringProxyClient {
	return &BoringProxyClient{}
}

func (c *BoringProxyClient) RunPuppetClient() {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	server := flagSet.String("server", "", "boringproxy server")
	token := flagSet.String("token", "", "Access token")
	name := flagSet.String("client-name", "", "Client name")
	flagSet.Parse(os.Args[2:])

	httpClient := &http.Client{}

	url := fmt.Sprintf("https://%s/api/tunnels?client-name=%s", *server, *name)

	listenReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("Failed making request", err)
	}

	if len(*token) > 0 {
		listenReq.Header.Add("Authorization", "bearer "+*token)
	}

	resp, err := httpClient.Do(listenReq)
	if err != nil {
		log.Fatal("Failed make tunnel request", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		log.Fatal("Failed to create tunnel: " + string(body))
	}

	tunnels := make(map[string]Tunnel)

	err = json.Unmarshal(body, &tunnels)
	if err != nil {
		log.Fatal("Failed to parse response", err)
	}

	for _, tun := range tunnels {
		go c.BoreTunnel(tun)
	}

	//go c.BoreTunnel(tunnels["apitman.com"])

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	for range sigChan {
		break
	}
}

func (c *BoringProxyClient) BoreTunnel(tun Tunnel) {
	privKeyFile, err := ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}

	defer os.Remove(privKeyFile.Name())

	if _, err := privKeyFile.Write([]byte(tun.TunnelPrivateKey)); err != nil {
		log.Fatal(err)
	}
	if err := privKeyFile.Close(); err != nil {
		log.Fatal(err)
	}

	tunnelSpec := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", tun.TunnelPort, tun.ClientPort)
	sshLogin := fmt.Sprintf("%s@%s", tun.Username, tun.ServerAddress)
	serverPortStr := fmt.Sprintf("%d", tun.ServerPort)
	fmt.Println(tunnelSpec, sshLogin, serverPortStr)
	cmd := exec.Command("ssh", "-i", privKeyFile.Name(), "-NR", tunnelSpec, sshLogin, "-p", serverPortStr)
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
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
