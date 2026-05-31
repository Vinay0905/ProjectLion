# Project LION: Code Walkthrough & Architecture Guide

Welcome to the comprehensive code explanation for **Project LION**—a local area network messaging and file transfer application built in Go.

This document breaks down every file in the project, explaining what each snippet does, highlighting critical lines of code, and detailing the custom TCP communication protocol.

---

## Table of Contents
1. [Network Protocol & Handshake Flow](#1-network-protocol--handshake-flow)
2. [The Server Code (`lion_chat/cmd/server/main.go`)](#2-the-server-code-cmdservermaingo)
3. [The Client Code (`lion_chat/cmd/client/main.go`)](#3-the-client-code-cmdclientmaingo)
4. [The Protocol Helpers (`lion_chat/internal/protocol/file.go`)](#4-the-protocol-helpers-internalprotocolfilego)
5. [The Message Framing (`lion_chat/internal/protocol/message.go`)](#5-the-message-framing-internalprotocolmessagego)

---

## 1. Network Protocol & Handshake Flow

Project LION communicates over raw TCP sockets on port `9000` by default. It uses a line-oriented ASCII control protocol, which means that command headers are separated by newline characters (`\n`), allowing us to parse commands easily using standard readers.

### Client-Server Flow (WhatsApp-Style Relayed File Sharing):
```text
Client A (Alice)                      Server (Hub)                      Client B (Bob)
       |                                   |                                   |
       |----- (1) Handshake Connection ---->|                                   |
       |      "NICK: Alice\n"              |                                   |
       |                                   |                                   |
       |                                   |<---- (2) Handshake Connection ----|
       |                                   |       "NICK: Bob\n"               |
       |                                   |                                   |
       |<- (3) [System: Bob joined chat] --|                                   |
       |                                   |                                   |
       |----- (4) Send chat message ------>|                                   |
       |      "hello world\n"              |                                   |
       |                                   |--- (5) Broadcast chat message --->|
       |                                   |    "Alice: hello world\n"         |
       |                                   |                                   |
       |                                   |<--- (6) Private upload to Hub ----|
       |                                   |    FILE_META project.zip|1048576\n|
       |                                   |    FILE_DATA 1048576\n            |
       |                                   |    [1,048,576 raw bytes]          |
       |                                   |                                   |
       |<- (7) Confirm upload complete ----|                                   |
       |    FILE_RECEIVED project.zip      |                                   |
       |                                   |                                   |
       |                                   |-- (8) Notify upload completed --->|
       |                                   |   "System: Bob uploaded file...   |
       |                                   |   Type '/get project.zip' to get" |
       |                                   |                                   |
       |                                   |<-- (9) Request download ----------|
       |                                   |    "/get project.zip\n"           |
       |                                   |                                   |
       |                                   |--- (10) Relayed download payload ->|
       |                                   |    FILE_META project.zip|1048576\n|
       |                                   |    FILE_DATA 1048576\n            |
       |                                   |    [1,048,576 raw bytes]          |
       |                                   |    (Saved by Bob's receiver loop) |
```

---

## 2. The Server Code (`lion_chat/cmd/server/main.go`)

The server is a concurrent TCP hub. It listens for incoming socket connections, spawns a new goroutine for each client, tracks connected nicknames, enforces session size limits, and handles text broadcasting, file uploads, and file downloads.

### Imports & Global State
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

	"example.com/lion_chat/internal/protocol"
)

var (
	clients    = make(map[net.Conn]string) // Key is TCP connection, value is nickname
	clientsMu  sync.Mutex
	writeMus   = make(map[net.Conn]*sync.Mutex) // Lock per connection to prevent interleaved writes
	writeMusMu sync.Mutex
)
```
* **`clients` Map**: Stored globally. Keys are `net.Conn` (active sockets), and values are `string` (nicknames).
* **`writeMus` Map**: Tracks a `sync.Mutex` pointer for each connection. This prevents a critical TCP issue: if one goroutine is writing a file download payload to Client B and another goroutine tries to write a broadcast text message to Client B simultaneously, the raw bytes will interleave and corrupt the downloaded file.

---

### Main Listener Loop
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
* **`net.Listen("tcp", ":9000")`**: Starts listening for incoming TCP traffic on port `9000`.
* **`go handleConnection(conn)`**: Spins up a concurrent goroutine to handle each connection asynchronously.

---

### Connection Handling & Session Limits
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
* **Session Limiting**: Checks `len(clients) >= 2` inside the map lock. If two clients are already active, the connection is rejected with a system message and closed. This keeps the application focused strictly on a 1-to-1 session.
* **`safeWrite`**: Executes a write block locked by the connection's dedicated mutex, keeping TCP frames safe from collision.

---

### Registration Cleanup & Command Router
```go
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
```
* **Disconnect Defer Cleanup**: Removes the client from the connection registry map and the write mutex registry map to clean up resource allocations, and broadcasts a system alert.

---

### Handling File Uploads
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
* **Relayed Notification**: Once a file is successfully saved under `downloads/`, the server broadcasts a message to the peer indicating that the file is available, specifying the exact download trigger: `Type '/get <filename>' to download it.`

---

### Handling File Downloads
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
* **`/get ` Command**: Sanitizes the requested file path via `filepath.Base` to block directory traversal attempts, checks if the file exists on the server, and uses `protocol.SendFile` wrapped in `safeWrite` to stream the payload back to the client.

---

### Thread-Safe Write Utilities
```go
// broadcast sends a message to all active clients.
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
```

---

## 3. The Client Code (`lion_chat/cmd/client/main.go`)

The client registers its nickname, launches an asynchronous receiver goroutine to intercept chat messages and incoming file transfers, and listens for console inputs (supporting `/send <path>` file uploads and `/get <filename>` downloads).

### Dual-Channel Receiver (Message and File Downloader)
```go
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
```
* **No Scanner Conflict**: Using `bufio.NewReader(conn)` is vital. Because a `bufio.Scanner` buffers arbitrary data blocks, parsing a `FILE_META` text line would buffer the trailing file payload bytes inside the scanner. Passing the socket to `ReceiveFile` afterward would hang, as those bytes were already consumed by the scanner's internal buffer. `bufio.Reader` allows us to read text lines and then stream raw payload bytes consecutively from the exact same read pointer.

---

## 4. The Protocol Helpers (`lion_chat/internal/protocol/file.go`)

### `ReceiveFile` with Byte Discard Safety
```go
// ReceiveFile reads size bytes from reader and saves them under receivedName in downloads/.
// If an error is encountered after size validation, it safely discards size bytes to keep
// the TCP socket connection synchronized.
func ReceiveFile(reader io.Reader, receivedName string, size int64) error {
	if size <= 0 || size > 50<<20 {
		return errors.New("file size must be between 1 byte and 50 MB")
	}

	// Helper to consume and discard the payload bytes in case of error.
	// This keeps the TCP buffer synchronized for subsequent commands.
	discardPayload := func() {
		_, _ = io.CopyN(io.Discard, reader, size)
	}

	safeName := filepath.Base(receivedName)
	if safeName != receivedName || safeName == "." {
		discardPayload()
		return errors.New("unsafe filename")
	}

	if err := os.MkdirAll("downloads", 0755); err != nil {
		discardPayload()
		return err
	}

	dst := filepath.Join("downloads", safeName)
	if _, err := os.Stat(dst); err == nil {
		discardPayload()
		return errors.New("file already exists")
	}

	out, err := os.Create(dst)
	if err != nil {
		discardPayload()
		return err
	}
	defer out.Close()

	_, err = io.CopyN(out, reader, size)
	return err
}
```
* **`discardPayload`**: If any step (directory creation, path verification, or file creation) fails, we call `discardPayload()` which consumes exactly `size` bytes from the stream into `io.Discard` before throwing the error. This keeps the network buffer aligned so the next reader calls don't read raw binary file data as chat text.

---

## 5. The Message Framing (`lion_chat/internal/protocol/message.go`)

This file is a placeholder structure for potential refactoring from line-terminated messages to size-framed structures.

```go
package protocol

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type Frame struct {
	Type    string
	Size    int64
	Payload io.Reader
}

func WriteText(w io.Writer, message string) error {
	_, err := fmt.Fprintf(w, "TEXT %d\n%s", len(message), message)
	return err
}

func ReadHeader(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(line), nil
}
```
* **Frame Struct**: Establishes a generic layout containing type tags and size parameters.
* **Future Upgrade Scope**: In standard raw TCP chats, users hitting Return with a message containing newlines (`\n`) can break line-based protocol parsers. Implementing `WriteText` (which prefixes the payload with its character count `TEXT <len>\n<data>`) allows the reader to safely consume multiline messages.
