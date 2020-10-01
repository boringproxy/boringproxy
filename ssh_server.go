package main

import (
        "fmt"
        "log"
        "net"
        "io/ioutil"
        "golang.org/x/crypto/ssh"
)


type SshServer struct {
        config *ssh.ServerConfig
        listener net.Listener
}


func NewSshServer() *SshServer {
        config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "user" && string(pass) == "yolo" {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}

        privateBytes, err := ioutil.ReadFile("id_rsa_boringproxy")
	if err != nil {
		log.Fatal("Failed to load private key: ", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key: ", err)
	}

	config.AddHostKey(private)

        listener, err := net.Listen("tcp", "0.0.0.0:2022")
	if err != nil {
		log.Fatal("failed to listen for connection: ", err)
	}

        server := &SshServer{config, listener}

        go server.acceptAll()

        return server
}

func (s *SshServer) acceptAll() {
        for {
                nConn, err := s.listener.Accept()
                if err != nil {
                        log.Print("failed to accept incoming connection: ", err)
                        continue
                }

                go s.handleServerConn(nConn)
        }
}

func (s *SshServer) handleServerConn(nConn net.Conn) {

        conn, chans, reqs, err := ssh.NewServerConn(nConn, s.config)
	if err != nil {
		log.Print("failed to handshake: ", err)
                return
	}

	go ssh.DiscardRequests(reqs)

        go func() {
                for newChannel := range chans {
                        newChannel.Reject(ssh.ResourceShortage, "too bad")
                }
        }()

        ch, cReqs, err := conn.OpenChannel("boringproxy-tunnel", []byte{25, 25})
        if err != nil {
                log.Print(err)
                return
        }

        go ssh.DiscardRequests(cReqs)

        ch.Write([]byte("Hi there"))
        ch.Close()
}
