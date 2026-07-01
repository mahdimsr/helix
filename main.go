package main

import (
	"fmt"
	"helix/metatrader"
	"log"
	"net"
)

func main() {

	const ADDRESS = "127.0.0.1:8585"

	fmt.Printf("Starting Listener...")

	listener, err := net.Listen("tcp", ADDRESS)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Listening to %s", ADDRESS)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}

		go metatrader.Handle(conn)
	}
}
