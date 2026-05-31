# Line-by-Line Explanation of Server `main.go`

This file runs the chat server. It listens for incoming network connections, allows a maximum of 2 participants, routes chat messages from one participant to another, and serves as an intermediary file storage hub (where files can be uploaded and then downloaded on-demand).

---

### Code Breakdown

#### Lines 1–16: Setup & Imports
```go
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
```
* **`package main`**: Tells Go that this file compiles to an executable program (the server).
* **Imports**:
  * `"bufio"`: Reads incoming network streams line-by-line.
  * `"fmt"`: Used to format status messages and write outputs to connections.
  * `"io"`: Deals with basic input/output errors like `io.EOF` (End of File/Disconnected).
  * `"log"`: Prints timestamps alongside server logs.
  * `"net"`: Core networking library. We use it to start the TCP listener.
  * `"os"`: Operating System tools. Used to check if files exist on the server.
  * `"path/filepath"`: Safely constructs file paths (e.g. `"downloads/photo.png"`).
  * `"strconv"`: Converts string numbers into integers.
  * `"strings"`: Cleans spaces and matches prefix commands.
  * `"sync"`: Thread-synchronization utilities (Mutexes). Very important because multiple clients will connect and talk at the same time.
  * `"protocol"`: Imports the protocol package containing our file transfer helpers.

---

#### Lines 18–23: Global State Variables
```go
var (
	clients    = make(map[net.Conn]string) // Key is TCP connection, value is nickname
	clientsMu  sync.Mutex
	writeMus   = make(map[net.Conn]*sync.Mutex) // Lock per connection to prevent interleaved writes
	writeMusMu sync.Mutex
)
```
* **`clients`**: A `map` (like a dictionary in Python) that stores active chatters. The key is the client's network connection, and the value is their nickname string.
* **`clientsMu`**: A `sync.Mutex` (Mutual Exclusion lock). Since multiple clients are processed concurrently, this lock prevents two clients from editing the `clients` map at the exact same fraction of a second, which would crash Go.
* **`writeMus`**: A map that holds a lock (`*sync.Mutex`) for each client connection. This prevents overlapping writes (e.g., if the server is sending a file to a user and broadcasts a new chat message to them at the same time, we don't want the bytes to mix up).
* **`writeMusMu`**: A lock to protect the `writeMus` map.

---

#### Lines 25–42: The `main()` Function (Starting the Server)
```go
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
```
* **`net.Listen("tcp", ":9000")`**: Tells the operating system to start listening for incoming TCP traffic on Port 9000.
* **`defer listener.Close()`**: Automatically shuts down the server socket when the program stops.
* **`for { ... }`**: An infinite loop. The server sits in this loop, waiting for people to connect.
* **`listener.Accept()`**: Pauses execution and waits until a client connects. When someone connects, it returns a network connection (`conn`).
* **`go handleConnection(conn)`**: Starts a separate background goroutine to handle this client. By doing this, the server can immediately loop back and accept the next client without waiting for the first one to finish.

---

#### Lines 44–64: `handleConnection()` & Handshake Phase
```go
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
```
* **`reader.ReadString('\n')`**: Reads the very first line sent by the client.
* **Handshake check**: If the client doesn't send the `NICK: <nickname>` protocol line, the server drops the connection.
* **Fallback name**: If the user didn't enter a nickname, we assign them a default one like `user_54321` based on their random network port number.

---

#### Lines 66–80: Registering and Restricting Clients
```go
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
```
* **`clientsMu.Lock() / Unlock()`**: Locks the client list while we inspect it.
* **`if len(clients) >= 2`**: This chat server is a 1-on-1 direct room. If there are already 2 users, it rejects the third client, sends them a "Chat session is full" system message, closes their connection, and exits.
* **`clients[conn] = nickname`**: If there's room, we save the user to our roster.

---

#### Lines 82–84: Join Notification
```go
	// Broadcast join notification to the other client
	broadcast(conn, fmt.Sprintf("[System: %s joined the chat]", nickname))
	log.Printf("%s (%s) connected", nickname, conn.RemoteAddr())
```
* **`broadcast(conn, ...)`**: Sends a system alert to the other client to announce that someone joined.

---

#### Lines 86–98: Defer Cleanup (Disconnections)
```go
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
```
* This block is executed *only* when the client disconnects and `handleConnection` terminates.
* It removes the client from our map roster, deletes their connection write locks, closes the TCP connection, prints a log, and tells the remaining chatter that this user has left.

---

#### Lines 100–110: Main Message Loop
```go
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
```
* This loop continuously listens for incoming lines of text from this client connection.
* If a read error occurs (e.g. client force-closes the window), we stop the loop and let the `defer` cleanup routine take over.

---

#### Lines 112–137: Handling File Metadata Command
```go
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
```
* If the user sends a file, the message starts with `"FILE_META "`.
* We parse the filename and size.
* We read the next line and make sure it says `FILE_DATA <size>`.

---

#### Lines 139–157: Receiving File and Broadcasting Notification
```go
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
```
* **`protocol.ReceiveFile(...)`**: Saves the uploaded file into the server's local `downloads/` folder.
* **`safeWrite(conn, ...)`**: Confirms to the sender whether the transfer succeeded or failed.
* **`broadcast(conn, ...)`**: Tells the other chatter that a file is available, instructing them to type `/get <filename>` if they want to download it.
* **`continue`**: Skips broadcasting the protocol headers as regular chat messages.

---

#### Lines 159–185: Handling File Downloads (`/get`)
```go
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
```
* When a chatter types `/get myphoto.jpg`, the server extracts the filename.
* **`filepath.Clean(filepath.Base(...))`**: Prevents hackers from requesting files like `/get ../../../../etc/passwd` to view sensitive files on the server.
* **`os.Stat(filePath)`**: Checks if the file exists on the server. If not, it returns a "File not found" message.
* **`protocol.SendFile(conn, filePath)`**: Sends the file back down the connection to the client who requested it.

---

#### Lines 187–192: Standard Chat Messages
```go
		// Normal chat message - broadcast to all other clients
		formattedMsg := fmt.Sprintf("%s: %s", nickname, message)
		fmt.Println(formattedMsg)
		broadcast(conn, formattedMsg)
	}
}
```
* If the message is a regular line of text, we format it as `Name: Message`, print it on the server console, and broadcast it to the other connected client.

---

#### Lines 194–209: The `broadcast` Helper Function
```go
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
```
* Loops through the list of active connections in `clients`.
* Skips the sender (so they don't see an echo of what they just typed).
* Uses `safeWrite` to send the text to everyone else.

---

#### Lines 211–225: The `safeWrite` Helper Function
```go
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
```
* When multiple threads want to send messages/files to the same client connection, they call this helper.
* It looks up or creates a unique mutex lock (`mu`) for that client connection, locks it, writes the data safely via `writeFunc()`, and unlocks it, ensuring no two writes clash.

---

#### Lines 227–234: The `getPort` Helper Function
```go
// getPort extracts the port number from an address string (e.g. "127.0.0.1:12345" -> "12345")
func getPort(addr string) string {
	parts := strings.Split(addr, ":")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "unknown"
}
```
* Takes a network address like `192.168.29.203:54321`, splits it by the colon `:`, and returns the last part `54321` which is the unique port code assigned to that client connection.
