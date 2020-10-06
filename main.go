package main

import (
	"fmt"
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
                Listen()

	case "client":
		client := NewBoringProxyClient()
		client.Run()
	default:
		fmt.Println("Invalid command " + command)
		os.Exit(1)
	}
}
