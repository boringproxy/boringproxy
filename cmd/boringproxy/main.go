package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/boringproxy/boringproxy"
)

const usage = `Usage: %s [command] [flags]

Commands:
    server       Start a new server.
    client       Connect to a server.

Use "%[1]s command -h" for a list of flags for the command.
`

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, os.Args[0]+": Need a command")
		fmt.Printf(usage, os.Args[0])
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "help", "-h", "--help", "-help":
		fmt.Printf(usage, os.Args[0])
	case "server":
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

		err := flagSet.Parse(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: parsing flags: %s\n", os.Args[0], err)
		}

		if *server == "" {
			fail("-server is required")
		}

		if *token == "" {
			fail("-token is required")
		}

		if *name == "" {
			fail("-client-name is required")
		}

		config := &boringproxy.ClientConfig{
			ServerAddr: *server,
			Token:      *token,
			ClientName: *name,
			User:       *user,
			CertDir:    *certDir,
			AcmeEmail:  *acmeEmail,
			DnsServer:  *dnsServer,
		}

		ctx := context.Background()

		client, err := boringproxy.NewClient(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}

		err = client.Run(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}

	default:
		fail(os.Args[0] + ": Invalid command " + command)
	}
}
