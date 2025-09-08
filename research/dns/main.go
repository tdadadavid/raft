package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"
)

var (
	TARGET_ADDRESS = "127.0.0.1:53"
	log            slog.Logger
)

func init() {
	log = *slog.Default()
}

func main() {
	// specify port and ip address for UDP
	addr := net.UDPAddr{
		Port: 53,
		IP:   net.ParseIP("127.0.0.1"),
	}

	// udp connection creatino
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Error("unable to connect to udp address", slog.Any("error", err))
		os.Exit(1)
	}
	defer conn.Close()

	buffer := make([]byte, 1024)

	for {
		// read content from client (this blocks until data is sent from the client)
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Error("error reading contents from the udp client", slog.Any("error", err))
			continue
		}

		message := buffer[:n]
		fmt.Printf("received %s from %s", message, clientAddr)
	}
}
