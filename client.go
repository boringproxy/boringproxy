package main

import (
        "log"
        //"bytes"
        "net/http"
        "fmt"
        "golang.org/x/crypto/ssh"
        "io/ioutil"
)


type BoringProxyClient struct {
}

func NewBoringProxyClient() *BoringProxyClient {
        return &BoringProxyClient{}
}

func (c *BoringProxyClient) Run() {
        log.Println("Run client")

        //var hostKey ssh.PublicKey

        key, err := ioutil.ReadFile("/home/anders/.ssh/id_rsa_test")
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("unable to parse private key: %v", err)
	}

        config := &ssh.ClientConfig{
		User: "anders",
		Auth: []ssh.AuthMethod{
                        ssh.PublicKeys(signer),
		},
		//HostKeyCallback: ssh.FixedHostKey(hostKey),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

        client, err := ssh.Dial("tcp", "boringproxy.io:22", config)
	if err != nil {
		log.Fatal("Failed to dial: ", err)
	}

        // Request the remote side to open port 8080 on all interfaces.
	l, err := client.Listen("tcp", "0.0.0.0:9001")
	if err != nil {
		log.Fatal("unable to register tcp forward: ", err)
	}
	defer l.Close()

	// Serve HTTP with your SSH server acting as a reverse proxy.
	http.Serve(l, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(resp, "Hi there\n")
	}))
}
