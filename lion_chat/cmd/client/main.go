package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"example.com/lion_chat/pkg/protocol"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	// (base) mast@Mastans-MacBook-Air ~ % ipconfig getifaddr en0
	// 192.168.29.203

	// 1. Prompt for server address at startup
	fmt.Print("Enter server address (default 192.168.29.203:9000): ")
	addrInput, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal("failed to read address input:", err)
	}
	addrInput = strings.TrimSpace(addrInput)
	if addrInput == "" {
		addrInput = "localhost:9000"
	}

	// 2. Prompt for nickname at startup
	fmt.Print("Enter your nickname: ")
	nickInput, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal("failed to read nickname input:", err)
	}
	nickInput = strings.TrimSpace(nickInput)

	// 3. Connect to the specified TCP server
	conn, err := net.Dial("tcp", addrInput)
	if err != nil {
		log.Fatal("connection error:", err)
	}
	defer conn.Close()

	// 4. Send nickname handshake immediately after connection
	if _, err := fmt.Fprintf(conn, "NICK: %s\n", nickInput); err != nil {
		log.Fatal("failed to send nickname handshake:", err)
	}

	fmt.Printf("Connected to %s! Type a message and press Return.\n> ", addrInput)

	// 5. Read incoming messages/files from the server in a separate goroutine
	go func() {
		// Use bufio.Reader instead of Scanner to safely share access with ReceiveFile
		netReader := bufio.NewReader(conn)
		for {
			line, err := netReader.ReadString('\n')
			if err != nil {
				log.Println("\nconnection read error:", err)
				return
			}
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")

			// Check if the server is sending us a file payload
			if strings.HasPrefix(line, "FILE_META ") {
				meta := strings.TrimPrefix(line, "FILE_META ")
				parts := strings.Split(meta, "|")
				if len(parts) != 2 {
					log.Println("\nbad file metadata received")
					fmt.Print("> ")
					continue
				}
				name := parts[0]
				size, err := strconv.ParseInt(parts[1], 10, 64)
				if err != nil {
					log.Println("\nbad file size received")
					fmt.Print("> ")
					continue
				}

				// Next line should be FILE_DATA <size>
				dataLine, err := netReader.ReadString('\n')
				if err != nil {
					log.Println("\nconnection lost while waiting for FILE_DATA")
					return
				}
				dataLine = strings.TrimSpace(dataLine)
				if dataLine != fmt.Sprintf("FILE_DATA %d", size) {
					log.Printf("\nunexpected file data header: %s\n> ", dataLine)
					continue
				}

				fmt.Printf("\n[System: Downloading file '%s' (%d bytes)...]\n", name, size)
				if err := protocol.ReceiveFile(netReader, name, size); err != nil {
					fmt.Printf("[System: Download error: %v]\n> ", err)
				} else {
					fmt.Printf("[System: Download complete! Saved in downloads/%s]\n> ", name)
				}
				continue
			}

			// Print normal chat/system message on a new line and re-display prompt
			fmt.Printf("\n%s\n> ", line)
		}
	}()

	// 6. Read console input in the main goroutine
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		line := input.Text()

		// If user types: /send <file_path>
		if len(line) > 6 && line[:6] == "/send " {
			path := line[6:]
			if err := protocol.SendFile(conn, path); err != nil {
				log.Println("sendFile error:", err)
			}
			fmt.Print("> ")
			continue
		}

		// Send normal text message/download command to the server
		if _, err := fmt.Fprintln(conn, line); err != nil {
			log.Fatal("write error to server:", err)
		}
		fmt.Print("> ")
	}
	if err := input.Err(); err != nil {
		log.Println("stdin read error:", err)
	}
}
