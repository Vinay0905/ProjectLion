package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"example.com/lion_chat/pkg/protocol"
)

var (
	clients    = make(map[net.Conn]string) // Key is TCP connection, value is nickname
	clientsMu  sync.Mutex
	writeMus   = make(map[net.Conn]*sync.Mutex) // Lock per connection to prevent interleaved writes
	writeMusMu sync.Mutex
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
			log.Println("accept error:", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	reader := bufio.NewReader(conn)

	// Expect the first line to be the nickname handshake: NICK: <nickname>
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return
	}
	firstLine = strings.TrimSpace(firstLine)

	var nickname string
	if strings.HasPrefix(firstLine, "NICK: ") {
		nickname = strings.TrimPrefix(firstLine, "NICK: ")
		nickname = strings.TrimSpace(nickname)
	}

	// Fallback to a default name using the client port if empty
	if nickname == "" {
		nickname = fmt.Sprintf("user_%s", getPort(conn.RemoteAddr().String()))
	}

	// Register client in thread-safe map if space is available (max 2 participants)
	clientsMu.Lock()
	if len(clients) >= 2 {
		clientsMu.Unlock()
		safeWrite(conn, func() error {
			fmt.Fprintln(conn, "[System: Chat session is full (maximum 2 participants)]")
			return nil
		})
		conn.Close()
		log.Printf("rejected connection from %s: session full", conn.RemoteAddr())
		return
	}
	clients[conn] = nickname
	clientsMu.Unlock()

	// Broadcast join notification to the other client
	broadcast(conn, fmt.Sprintf("[System: %s joined the chat]", nickname))
	log.Printf("%s (%s) connected", nickname, conn.RemoteAddr())

	// Defer clean up on disconnect
	defer func() {
		clientsMu.Lock()
		delete(clients, conn)
		clientsMu.Unlock()

		writeMusMu.Lock()
		delete(writeMus, conn)
		writeMusMu.Unlock()

		conn.Close()
		log.Printf("%s (%s) disconnected", nickname, conn.RemoteAddr())
		broadcast(nil, fmt.Sprintf("[System: %s left the chat]", nickname))
	}()

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

		// Handle file transfer command (kept private between sender and server)
		if strings.HasPrefix(message, "FILE_META ") {
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

			// Read next line expecting FILE_DATA <size>
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

			// Stream the file payload securely using the protocol package
			if err := protocol.ReceiveFile(reader, name, size); err != nil {
				log.Println("receiveFile error:", err)
				safeWrite(conn, func() error {
					fmt.Fprintln(conn, "FILE_ERROR", err)
					return nil
				})
			} else {
				log.Printf("received file from %s: %s (%d bytes)", nickname, name, size)
				safeWrite(conn, func() error {
					fmt.Fprintf(conn, "FILE_RECEIVED %s (%d bytes)\n", name, size)
					return nil
				})

				// Broadcast to Client 2 that a file is available for download on-demand
				broadcast(conn, fmt.Sprintf("[System: %s uploaded a file: %s (%d bytes). Type '/get %s' to download it.]", nickname, name, size, name))
			}
			continue
		}

		// Handle file download request
		if strings.HasPrefix(message, "/get ") {
			filename := strings.TrimPrefix(message, "/get ")
			filename = filepath.Clean(filepath.Base(filename))
			filePath := filepath.Join("downloads", filename)

			if _, err := os.Stat(filePath); err != nil {
				safeWrite(conn, func() error {
					fmt.Fprintln(conn, "[System: File not found on server]")
					return nil
				})
				continue
			}

			log.Printf("sending file '%s' to %s", filename, nickname)
			err := safeWrite(conn, func() error {
				return protocol.SendFile(conn, filePath)
			})
			if err != nil {
				log.Printf("SendFile error to %s: %v", nickname, err)
				safeWrite(conn, func() error {
					fmt.Fprintf(conn, "[System: Error downloading file: %v]\n", err)
					return nil
				})
			}
			continue
		}

		// Normal chat message - broadcast to all other clients
		formattedMsg := fmt.Sprintf("%s: %s", nickname, message)
		fmt.Println(formattedMsg)
		broadcast(conn, formattedMsg)
	}
}

// broadcast sends a message to all active clients.
// If sender is non-nil, it skips writing to that client to avoid echo.
func broadcast(sender net.Conn, message string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	for client := range clients {
		if client == sender {
			continue
		}
		safeWrite(client, func() error {
			_, err := fmt.Fprintln(client, message)
			return err
		})
	}
}

// safeWrite locks a mutex dedicated to the connection to ensure that multiple
// goroutines (such as file-downloads and broadcasts) do not write overlapping bytes.
func safeWrite(conn net.Conn, writeFunc func() error) error {
	writeMusMu.Lock()
	mu, exists := writeMus[conn]
	if !exists {
		mu = &sync.Mutex{}
		writeMus[conn] = mu
	}
	writeMusMu.Unlock()

	mu.Lock()
	defer mu.Unlock()
	return writeFunc()
}

// getPort extracts the port number from an address string (e.g. "127.0.0.1:12345" -> "12345")
func getPort(addr string) string {
	parts := strings.Split(addr, ":")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "unknown"
}
