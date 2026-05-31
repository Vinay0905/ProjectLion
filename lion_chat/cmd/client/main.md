# Line-by-Line Explanation of Client `main.go`

This file is the client application. When you run it, it prompts you for the server's IP address and your desired nickname, connects to the server, and starts a two-way communication: it listens for incoming messages (and downloads files) in the background while letting you type messages and send files from your keyboard.

---

### Code Breakdown

#### Lines 1–13: Setup & Imports
```go
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
```
* **`package main`**: Tells Go that this file contains the `main` entry point function and is an executable program (not just a library).
* **Imports**:
  * `"bufio"`: Reads inputs from the keyboard and the network.
  * `"fmt"`: Prints messages to the terminal screen.
  * `"log"`: Prints log messages with the current time (and terminates the app if critical errors happen).
  * `"net"`: The core network package. We use this to connect to the server using TCP/IP.
  * `"os"`: Operating System package. Used here to read from standard input (`os.Stdin` - the keyboard).
  * `"strconv"`: String conversions (converting text numbers to actual math integers).
  * `"strings"`: Used to modify text, clean up white spaces, and check prefixes.
  * `"example.com/lion_chat/pkg/protocol"`: Imports the protocol helper packages we explained in `message.md` and `file.md`.

---

#### Lines 15–16: The `main()` Function Start & Stdin Reader
```go
func main() {
	reader := bufio.NewReader(os.Stdin)
```
* **`func main()`**: This is the starting point of the application when you run it.
* **`bufio.NewReader(os.Stdin)`**: Prepares a reader to listen to whatever you type in the terminal (standard input).

---

#### Lines 20–29: Prompting for the Server IP Address
```go
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
```
* **`fmt.Print(...)`**: Prints a prompt asking you for the server address.
* **`reader.ReadString('\n')`**: Pauses and waits for you to type the address and press Return/Enter.
* **`log.Fatal(...)`**: If reading failed (e.g. terminal closed), print the error and exit the app.
* **`strings.TrimSpace(...)`**: Cleans up the input, removing the newline character at the end.
* **`if addrInput == ""`**: If you didn't type anything and just pressed Enter, default to connecting to your own local machine at `localhost:9000`.

---

#### Lines 31–37: Prompting for a Nickname
```go
	// 2. Prompt for nickname at startup
	fmt.Print("Enter your nickname: ")
	nickInput, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal("failed to read nickname input:", err)
	}
	nickInput = strings.TrimSpace(nickInput)
```
* Asks you for your nickname in the chat, reads it from the console, and cleans it up.

---

#### Lines 39–44: Connecting to the TCP Server
```go
	// 3. Connect to the specified TCP server
	conn, err := net.Dial("tcp", addrInput)
	if err != nil {
		log.Fatal("connection error:", err)
	}
	defer conn.Close()
```
* **`net.Dial("tcp", addrInput)`**: Opens a TCP connection to the server address you provided.
* **`if err != nil`**: If the server isn't running or the IP is wrong, it will print the error and close the app immediately.
* **`defer conn.Close()`**: Automatically closes the network connection when the program exits.

---

#### Lines 46–49: Sending Nickname to the Server (Handshake)
```go
	// 4. Send nickname handshake immediately after connection
	if _, err := fmt.Fprintf(conn, "NICK: %s\n", nickInput); err != nil {
		log.Fatal("failed to send nickname handshake:", err)
	}
```
* Immediately sends the server a special first line: `"NICK: <your nickname>\n"`. This is the handshake that lets the server know who you are.

---

#### Lines 51: Printing Connection Success
```go
	fmt.Printf("Connected to %s! Type a message and press Return.\n> ", addrInput)
```
* Tells you that you are connected and displays a prompt `>` to show you can start typing.

---

#### Lines 54–55: Starting a background listener (Goroutine)
```go
	// 5. Read incoming messages/files from the server in a separate goroutine
	go func() {
```
* **`go func() { ... }()`**: The `go` keyword starts a **goroutine**. This is a lightweight background thread. Because the client needs to *listen* to the server and *accept* keyboard typing at the same time, we do the listening in the background so it doesn't block you from typing.

---

#### Lines 56–65: Listening Loop Start & Reading line-by-line
```go
		netReader := bufio.NewReader(conn)
		for {
			line, err := netReader.ReadString('\n')
			if err != nil {
				log.Println("\nconnection read error:", err)
				return
			}
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
```
* **`bufio.NewReader(conn)`**: Sets up a reader to read bytes coming from the server.
* **`for { ... }`**: An infinite loop. It will run forever until the connection drops or the app closes.
* **`netReader.ReadString('\n')`**: Waits for the server to send a line of text (ending with `\n`).
* **`if err != nil`**: If the connection breaks, it prints the error and stops the goroutine (`return`).
* **`TrimSuffix`**: Cleans up carriage returns and newlines.

---

#### Lines 67–82: Checking for Incoming File metadata
```go
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
```
* **`strings.HasPrefix(line, "FILE_META ")`**: If the message line starts with `"FILE_META "`, we know the server is not sending regular chat text, but is starting a file transfer.
* **`strings.Split(meta, "|")`**: Splits the metadata by the pipe symbol `|` to get the filename and filesize.
* **`strconv.ParseInt(...)`**: Converts the filesize (which is text, like `"1024"`) into an actual integer number.

---

#### Lines 84–93: Checking for the File Data Header
```go
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
```
* Reads the next line from the network. It must match `FILE_DATA <size>`. If it does not, something is corrupted, and we skip it.

---

#### Lines 95–102: Downloading the File
```go
				fmt.Printf("\n[System: Downloading file '%s' (%d bytes)...]\n", name, size)
				if err := protocol.ReceiveFile(netReader, name, size); err != nil {
					fmt.Printf("[System: Download error: %v]\n> ", err)
				} else {
					fmt.Printf("[System: Download complete! Saved in downloads/%s]\n> ", name)
				}
				continue
			}
```
* **`protocol.ReceiveFile(...)`**: Calls the helper function we defined in `file.go` to download the file and save it in the `downloads/` directory.
* If it succeeds, it prints a success message; otherwise, it prints a download error.

---

#### Lines 104–107: Displaying Regular Messages and Loop End
```go
			// Print normal chat/system message on a new line and re-display prompt
			fmt.Printf("\n%s\n> ", line)
		}
	}()
```
* If the line wasn't a file transfer, it's a regular chat message. We print it to the screen and re-show the `> ` prompt so the user knows they can continue typing.
* The `}` closes the loop, and the `}()` executes the background goroutine immediately.

---

#### Lines 109–112: Main Thread - Reading Console Input
```go
	// 6. Read console input in the main goroutine
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		line := input.Text()
```
* Now we are back in the main thread (the user's terminal interface).
* **`bufio.NewScanner(os.Stdin)`**: Listens to the keyboard input.
* **`for input.Scan()`**: Loops every time you press Enter, saving what you typed in `line`.

---

#### Lines 114–122: Handling the `/send` Command
```go
		// If user types: /send <file_path>
		if len(line) > 6 && line[:6] == "/send " {
			path := line[6:]
			if err := protocol.SendFile(conn, path); err != nil {
				log.Println("sendFile error:", err)
			}
			fmt.Print("> ")
			continue
		}
```
* If you type `/send <file_path>` (like `/send documents/invoice.pdf`), it will slice the input to get the file path.
* **`protocol.SendFile(conn, path)`**: Calls the helper from `file.go` to send the file across the network to the server.
* Once done, it prints `> ` and skips the rest of the loop (`continue`) so we don't send the literal "/send" string as a chat message.

---

#### Lines 124–129: Sending Chat Messages to Server
```go
		// Send normal text message/download command to the server
		if _, err := fmt.Fprintln(conn, line); err != nil {
			log.Fatal("write error to server:", err)
		}
		fmt.Print("> ")
	}
```
* If it's a regular message, we use `fmt.Fprintln` to write your text directly into the network connection `conn` so the server receives it.
* It prints `> ` to prepare for your next input.

---

#### Lines 130–133: End of Program Error Checks
```go
	if err := input.Err(); err != nil {
		log.Println("stdin read error:", err)
	}
}
```
* If the terminal keyboard scanning loop encounters any system error, it prints it here right before the program finishes.
