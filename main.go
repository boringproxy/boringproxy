package main

import (
	"fmt"
	"os"
)

const usage = `Usage: %s [command] [flags]

Commands:
    server       Start a new server.
    client       Connect to a server.

Use "%[1]s command -h" for a list of flags for the command.
`

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
		Listen()
	case "client":
		client := NewBoringProxyClient()
		client.RunPuppetClient()
	default:
		fmt.Fprintln(os.Stderr, os.Args[0]+": Invalid command "+command)
		os.Exit(1)
	}
}
