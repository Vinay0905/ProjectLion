package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"

	"example.com/lion_chat/internal/protocol"
)

func main() {
	conn, err := net.Dial("tcp", "192.168.29.203:9000")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	fmt.Println("connected; type a message and press Return")
	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			fmt.Println("\npeer:", scanner.Text())
			fmt.Print("> ")

		}
		if err := scanner.Err(); err != nil {
			log.Println("read error from server:", err)
		}
	}()
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		line := input.Text()

		// if user types: /send path/to/file
		if len(line) > 6 && line[:6] == "/send " {
			path := line[6:]
			if err := protocol.SendFile(conn, path); err != nil {
				log.Println("sendFile error:", err)
			}
			continue
		}

		if _, err := fmt.Fprintln(conn, input.Text()); err != nil {
			log.Fatal(err)
		}
	}
	if err := input.Err(); err != nil {
		log.Println("read error from server:", err)
	}
}
