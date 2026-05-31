package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"example.com/lion_chat/internal/protocol"
)

func main() {
	listener, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatal(err)

	}
	defer listener.Close()

	log.Println("server listening on :9000")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("accept error: ", err)
			continue
		}
		go handleConnection(conn)
	}
}
func handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Println("connected:", conn.RemoteAddr())
	// CHANGE 2: Use one buffered reader for headers and file bytes. A Scanner can
	// buffer part of a file payload, which makes a later direct read from conn hang.
	reader := bufio.NewReader(conn)

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Println("read error:", err)
			}
			return
		}
		message = strings.TrimSuffix(message, "\n")
		message = strings.TrimSuffix(message, "\r")

		// Handle file metadata
		if strings.HasPrefix(message, "FILE_META ") {
			// message: FILE_META name|size
			meta := strings.TrimPrefix(message, "FILE_META ")
			parts := strings.Split(meta, "|")
			if len(parts) != 2 {
				log.Println("bad FILE_META:", message)
				continue
			}

			name := parts[0]
			size, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				log.Println("bad size in FILE_META:", err)
				continue
			}

			// Next line should be FILE_DATA <size>
			dataLine, err := reader.ReadString('\n')
			if err != nil {
				log.Println("read error while waiting for FILE_DATA:", err)
				return
			}
			dataLine = strings.TrimSpace(dataLine)
			if dataLine != fmt.Sprintf("FILE_DATA %d", size) {
				log.Println("unexpected FILE_DATA line:", dataLine)
				continue
			}

			// Read from the same buffered reader used for the headers.
			if err := protocol.ReceiveFile(reader, name, size); err != nil {
				log.Println("receiveFile error:", err)
				fmt.Fprintln(conn, "FILE_ERROR", err)
			} else {
				// Confirm the saved file in both the server and client terminals.
				log.Printf("received file: %s (%d bytes)", name, size)
				fmt.Fprintf(conn, "FILE_RECEIVED %s (%d bytes)\n", name, size)
			}
			continue
		}

		// Normal chat message
		fmt.Println("message:", message)
		if _, err := fmt.Fprintln(conn, message); err != nil {
			log.Println("write error:", err)
			return
		}
	}
}
