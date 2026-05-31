# Line-by-Line Explanation of Wails `app.go`

This file serves as the Go backend controller for the desktop client. It bridges the Wails HTML/JS frontend and the low-level Go TCP socket layer. It handles state, triggers native dialog boxes, and runs a background socket reader that emits events to the visual interface.

---

### Code Breakdown

#### Lines 1–15: Setup & Imports
```go
package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"example.com/lion_chat/pkg/protocol"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)
```
* **`package main`**: Part of the main executable bundle.
* **Imports**:
  * `"bufio"`: Reads lines of text from the TCP network buffer.
  * `"context"`: Manages Wails runtime lifetimes.
  * `"net"`: Opens TCP sockets to connect to the LION server.
  * `"os"`: Operating System tools. Used to find the current directory.
  * `"path/filepath"`: Creates cross-platform paths (e.g. `downloads/file.txt`).
  * `"strconv"`: Parses string numbers (like file sizes) into integers.
  * `"strings"`: Cleans and matches command prefixes.
  * `"sync"`: Thread-synchronization utilities (Mutexes).
  * `"protocol"`: Our core Project LION TCP packet framing helpers.
  * `"runtime"`: Wails runtime utilities for file pickers, event emissions, and logging.

---

#### Lines 17–25: App Struct
```go
type App struct {
	ctx        context.Context
	conn       net.Conn
	connected  bool
	nickname   string
	serverAddr string
	mu         sync.Mutex
}
```
* **`App`**: The main struct bound to the Javascript window context.
  * `ctx`: Context pointer used to invoke Wails runtime commands.
  * `conn`: Active TCP socket connection to the server.
  * `connected`: Boolean status tracking connection.
  * `nickname` / `serverAddr`: Cached details of the current connection.
  * `mu`: Mutex lock. Prevents data race condition bugs if the GUI calls backend actions concurrently.

---

#### Lines 27–37: Initializers and Lifecycle Hooks
```go
func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}
```
* **`NewApp`**: Instantiates the App struct.
* **`startup`**: Triggered by Wails when the GUI window boots up. Saves the context pointer.

---

#### Lines 39–77: Connecting and Starting Reader
```go
func (a *App) Connect(serverAddr string, nickname string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.connected {
		return "Already connected", nil
	}

	addr := strings.TrimSpace(serverAddr)
	if addr == "" {
		addr = "localhost:9000"
	}
	nick := strings.TrimSpace(nickname)
	if nick == "" {
		return "", fmt.Errorf("nickname cannot be empty")
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("connection error: %v", err)
	}

	if _, err := fmt.Fprintf(conn, "NICK: %s\n", nick); err != nil {
		conn.Close()
		return "", fmt.Errorf("handshake error: %v", err)
	}

	a.conn = conn
	a.connected = true
	a.nickname = nick
	a.serverAddr = addr

	go a.startReader(conn)

	return fmt.Sprintf("Connected to %s", addr), nil
}
```
* **`Connect`**: Invoked by the frontend when you click "Connect".
* **Handshake Check**: Connects via TCP, sends `NICK: <nickname>\n` immediately.
* **`go a.startReader(conn)`**: Spins off a background goroutine loop to read from the socket so the desktop window remains responsive.

---

#### Lines 79–93: Disconnecting
```go
func (a *App) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.connected || a.conn == nil {
		return nil
	}

	err := a.conn.Close()
	a.connected = false
	a.conn = nil
	return err
}
```
* **`Disconnect`**: Closes the active TCP connection and resets app states.

---

#### Lines 95–114: Sending Text Message
```go
func (a *App) SendMessage(message string) (string, error) {
	a.mu.Lock()
	conn := a.conn
	connected := a.connected
	a.mu.Unlock()

	if !connected || conn == nil {
		return "", fmt.Errorf("not connected to server")
	}

	msg := strings.TrimSpace(message)
	if msg == "" {
		return "", nil
	}

	if _, err := fmt.Fprintln(conn, message); err != nil {
		return "", fmt.Errorf("failed to send message: %v", err)
	}

	return "Sent", nil
}
```
* **`SendMessage`**: Writes raw text lines to the socket connection.

---

#### Lines 116–160: Native File Selection and Upload
```go
func (a *App) SelectAndSendFile() (string, error) {
	a.mu.Lock()
	conn := a.conn
	connected := a.connected
	a.mu.Unlock()

	if !connected || conn == nil {
		return "", fmt.Errorf("not connected to server")
	}

	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select File to Send",
	})
	if err != nil {
		return "", fmt.Errorf("dialog error: %v", err)
	}
	if filePath == "" {
		return "", nil
	}

	go func() {
		fileName := filepath.Base(filePath)
		runtime.EventsEmit(a.ctx, "upload_status", map[string]interface{}{
			"status": "started",
			"path":   filePath,
			"name":   fileName,
		})

		err := protocol.SendFile(conn, filePath)
		if err != nil {
			runtime.EventsEmit(a.ctx, "upload_status", map[string]interface{}{
				"status": "error",
				"path":   filePath,
				"name":   fileName,
				"error":  err.Error(),
			})
		} else {
			runtime.EventsEmit(a.ctx, "upload_status", map[string]interface{}{
				"status": "completed",
				"path":   filePath,
				"name":   fileName,
			})
		}
	}()

	return filepath.Base(filePath), nil
}
```
* **`runtime.OpenFileDialog`**: Shows the native system OS file picker to let the user select any file.
* **Goroutine Uploader**: Uploads the file in a separate background thread.
* **`runtime.EventsEmit(...)`**: Sends updates (`upload_status: started/completed/error`) to the JavaScript UI.

---

#### Lines 162–184: Downloading Files
```go
func (a *App) DownloadFile(filename string) (string, error) {
	a.mu.Lock()
	conn := a.conn
	connected := a.connected
	a.mu.Unlock()

	if !connected || conn == nil {
		return "", fmt.Errorf("not connected to server")
	}

	safeName := filepath.Base(filename)
	if safeName != filename || safeName == "." {
		return "", fmt.Errorf("unsafe filename")
	}

	_, err := fmt.Fprintf(conn, "/get %s\n", safeName)
	if err != nil {
		return "", fmt.Errorf("failed to request file: %v", err)
	}

	return fmt.Sprintf("Requested %s", safeName), nil
}
```
* **`DownloadFile`**: Sends a `/get <filename>` string command to the server TCP stream.

---

#### Lines 186–207: State Getters
```go
func (a *App) GetDownloadsDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "downloads"
	}
	return filepath.Join(wd, "downloads")
}

func (a *App) IsConnected() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.connected
}

func (a *App) GetNickname() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.nickname
}
```
* State helper methods requested by the frontend view to show connection and download statuses.

---

#### Lines 209–313: Background TCP Socket Listener Loop
```go
func (a *App) startReader(conn net.Conn) {
	netReader := bufio.NewReader(conn)
	for {
		line, err := netReader.ReadString('\n')
		if err != nil {
			a.mu.Lock()
			a.connected = false
			a.conn = nil
			a.mu.Unlock()

			runtime.EventsEmit(a.ctx, "disconnected", "Connection lost: "+err.Error())
			return
		}

		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		if strings.HasPrefix(line, "FILE_META ") {
			meta := strings.TrimPrefix(line, "FILE_META ")
			parts := strings.Split(meta, "|")
			if len(parts) != 2 {
				runtime.EventsEmit(a.ctx, "app_error", "Bad file metadata received")
				continue
			}
			name := parts[0]
			size, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				runtime.EventsEmit(a.ctx, "app_error", "Bad file size received")
				continue
			}

			dataLine, err := netReader.ReadString('\n')
			if err != nil {
				runtime.EventsEmit(a.ctx, "disconnected", "Connection lost waiting for file payload")
				return
			}
			dataLine = strings.TrimSpace(dataLine)
			if dataLine != fmt.Sprintf("FILE_DATA %d", size) {
				runtime.EventsEmit(a.ctx, "app_error", "Unexpected file data header: "+dataLine)
				continue
			}

			runtime.EventsEmit(a.ctx, "download_status", map[string]interface{}{
				"status": "started",
				"name":   name,
				"size":   size,
			})

			err = protocol.ReceiveFile(netReader, name, size)
			if err != nil {
				runtime.EventsEmit(a.ctx, "download_status", map[string]interface{}{
					"status": "error",
					"name":   name,
					"error":  err.Error(),
				})
			} else {
				runtime.EventsEmit(a.ctx, "download_status", map[string]interface{}{
					"status": "completed",
					"name":   name,
					"path":   filepath.Join("downloads", name),
				})
			}
			continue
		}

		isSystem := strings.HasPrefix(line, "[System:") && strings.HasSuffix(line, "]")
		var sender, text string

		if isSystem {
			sender = "System"
			text = strings.TrimSuffix(strings.TrimPrefix(line, "[System: "), "]")
		} else {
			idx := strings.Index(line, ": ")
			if idx != -1 {
				sender = line[:idx]
				text = line[idx+2:]
			} else {
				sender = "Server"
				text = line
			}
		}

		runtime.EventsEmit(a.ctx, "message", map[string]interface{}{
			"sender": sender,
			"text":   text,
			"raw":    line,
		})
	}
}
```
* **Listening Loop**:
  * Continually reads text lines from the TCP stream.
  * **File Streaming**: If the line starts with `FILE_META `, it reads the data sizes, parses the header, emits `download_status: started`, calls `protocol.ReceiveFile(netReader, ...)` to pull raw bytes off the TCP stream, and emits `download_status: completed` once saved.
  * **Chat Broadcasts**: If it is normal chat, it splits the string into `sender` and `text`, and uses `EventsEmit` to notify the UI to append a bubble.
