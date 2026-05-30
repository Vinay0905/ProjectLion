package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
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
		if _, err := fmt.Fprintln(conn, input.Text()); err != nil {
			log.Fatal(err)
		}
	}
	if err := input.Err(); err != nil {
		log.Println("read error from server:", err)
	}

}
