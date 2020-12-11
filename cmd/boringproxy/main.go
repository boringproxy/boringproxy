package main

import (
	"flag"
	"fmt"
	"github.com/boringproxy/boringproxy"
	"log"
	"os"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Println("Invalid arguments")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "server":
		log.Println("Starting up")
		boringproxy.Listen()

	case "client":

		flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		server := flagSet.String("server", "", "boringproxy server")
		token := flagSet.String("token", "", "Access token")
		name := flagSet.String("client-name", "", "Client name")
		user := flagSet.String("user", "admin", "user")
		certDir := flagSet.String("cert-dir", "", "TLS cert directory")
		acmeEmail := flagSet.String("acme-email", "", "Email for ACME (ie Let's Encrypt)")
		dnsServer := flagSet.String("dns-server", "", "Custom DNS server")
		flagSet.Parse(os.Args[2:])

		config := &boringproxy.ClientConfig{
			ServerAddr: *server,
			Token:      *token,
			ClientName: *name,
			User:       *user,
			CertDir:    *certDir,
			AcmeEmail:  *acmeEmail,
			DnsServer:  *dnsServer,
		}

		client := boringproxy.NewClient(config)
		client.RunPuppetClient()
	default:
		fmt.Println("Invalid command " + command)
		os.Exit(1)
	}
}
