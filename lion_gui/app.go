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

// App struct
type App struct {
	ctx        context.Context
	conn       net.Conn
	connected  bool
	nickname   string
	serverAddr string
	mu         sync.Mutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Connect establishes a connection to the TCP server and does the nickname handshake
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

	// Send nickname handshake immediately after connection
	if _, err := fmt.Fprintf(conn, "NICK: %s\n", nick); err != nil {
		conn.Close()
		return "", fmt.Errorf("handshake error: %v", err)
	}

	a.conn = conn
	a.connected = true
	a.nickname = nick
	a.serverAddr = addr

	// Start background reader
	go a.startReader(conn)

	return fmt.Sprintf("Connected to %s", addr), nil
}

// Disconnect closes the active connection
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

// SendMessage sends a text message or requests a download
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

	// Send normal text message/download command to the server
	if _, err := fmt.Fprintln(conn, message); err != nil {
		return "", fmt.Errorf("failed to send message: %v", err)
	}

	return "Sent", nil
}

// SelectAndSendFile opens a native dialog and uploads the selected file
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
		return "", nil // User cancelled
	}

	// Run file transfer in background to keep GUI responsive
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

// DownloadFile sends a request to the server to download the specified file
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

	// Send get command to server
	_, err := fmt.Fprintf(conn, "/get %s\n", safeName)
	if err != nil {
		return "", fmt.Errorf("failed to request file: %v", err)
	}

	return fmt.Sprintf("Requested %s", safeName), nil
}

// GetDownloadsDir returns the absolute path of the local downloads folder
func (a *App) GetDownloadsDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "downloads"
	}
	return filepath.Join(wd, "downloads")
}

// IsConnected returns whether the client is connected to a server
func (a *App) IsConnected() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.connected
}

// GetNickname returns the user's nickname
func (a *App) GetNickname() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.nickname
}

// startReader reads incoming socket messages and broadcasts them to the frontend
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

		// 1. Check if server is sending a file payload
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

			// Read FILE_DATA line
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

			// Notify download started
			runtime.EventsEmit(a.ctx, "download_status", map[string]interface{}{
				"status": "started",
				"name":   name,
				"size":   size,
			})

			// Download file
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

		// 2. Otherwise, it is a normal text chat or system broadcast message
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
