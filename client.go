package main

import (
        "log"
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

        config := &ssh.ClientConfig{
		User: "user",
		Auth: []ssh.AuthMethod{
                        ssh.Password("yolo"),
		},
		//HostKeyCallback: ssh.FixedHostKey(hostKey),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

        client, err := ssh.Dial("tcp", "boringproxy.io:2022", config)
	if err != nil {
		log.Fatal("Failed to dial: ", err)
	}

        tunnelRequests := client.HandleChannelOpen("boringproxy-tunnel")

        for req := range tunnelRequests {
                go handleTunnelRequest(req)
        }
}

func handleTunnelRequest(req ssh.NewChannel) error {

        tun, reqs, err := req.Accept()
        if err != nil {
                return err
        }

        go ssh.DiscardRequests(reqs)

        port := req.ExtraData()

        fmt.Println(port)

        data, err := ioutil.ReadAll(tun)
        if err != nil {
                return err
        }

        fmt.Println(string(data))
        return nil
}
