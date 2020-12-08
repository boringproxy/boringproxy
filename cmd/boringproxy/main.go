package main

import (
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
		client := boringproxy.NewClient()
		client.RunPuppetClient()
	default:
		fmt.Println("Invalid command " + command)
		os.Exit(1)
	}
}
