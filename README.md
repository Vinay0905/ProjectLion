# Project LION

Project LION is a beginner-friendly **local network messaging app** written in Go. 
Start with multi-client text chat, then add file transfer and an optional desktop GUI.

✅ **Working Features:**
- Server listens on port `9000` and handles multiple clients  
- Clients connect with a nickname (handshake)
- Messages broadcast to all connected clients
- Built-in LAN support for same-computer or Wi-Fi testing
- Modern Desktop GUI application (built using Wails, HTML, CSS, and Vanilla JS)

🚧 **Planned:**
- Framed file transfer (with validation & safety checks)


## Quick Start

### Prerequisites
- Go installed ([Install Go](https://go.dev/doc/install))
- Three terminals (one server + two clients to test)

### Run Server

**Terminal 1:**
```bash
cd lion_chat
go run ./cmd/server
```

You should see:
```
server listening on :9000
```

### Run Client 1

**Terminal 2:**
```bash
cd lion_chat
go run ./cmd/client
```

When prompted:
- **Server address:** Press Enter for `localhost:9000` (same computer) or enter your server's IP (e.g., `192.168.29.203:9000`)
- **Nickname:** Enter a unique name (e.g., `Alice`)

### Run Client 2

**Terminal 3:**
```bash
cd lion_chat
go run ./cmd/client
```

When prompted:
- **Server address:** Same as Client 1
- **Nickname:** Enter a different name (e.g., `Bob`)

**Test it:** Type a message in one client — both clients should receive it!

## LAN Testing (Different Computers)

Find your server's local IP:
```bash
ipconfig getifaddr en0
```

On the client machine, when prompted for server address, enter the IP with port:
```
192.168.29.203:9000
```

## Project Structure

```
lion_chat/
  cmd/
    server/main.go    ← Server code
    client/main.go    ← Client code
  internal/
    protocol/         ← Message framing logic
  downloads/          ← For future file transfer
  go.mod
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| **Connection refused** | Ensure server is running on port 9000 before starting clients |
| **Timeout** | Check both machines are on same Wi-Fi; verify IP with `ipconfig getifaddr en0` |
| **Firewall popup** | Allow Terminal/Go app to accept local connections |

**Security Note:** This is a prototype—no encryption or authentication. Don't expose to the internet.

## Roadmap

1. ✅ Multi-client messaging
2. ✅ File transfer with frame protocol (integrated into GUI)
3. ⏳ Input validation & safety checks
4. ✅ Desktop GUI (Wails)

## Running the Desktop GUI App

### Prerequisites
- Node.js & NPM installed
- Wails CLI installed: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Running in Development Mode
You can run the GUI application in live-reload mode (which updates instantly when you edit CSS, HTML, JS, or Go code):

```bash
cd lion_gui
wails dev
```

### Building the Native App
To compile the standalone desktop app:

```bash
cd lion_gui
wails build
```

This creates a native app inside `lion_gui/build/bin/lion_gui.app` on macOS. You can open and run this binary directly on your system!

## Tutorial Guide

Full tutorial available as generated documentation:

- `intro/LocalNetworkMessengerFinal.docx`
- `messengerFinal.pdf`

## References

- [Install Go](https://go.dev/doc/install)
- [Go `net` package](https://pkg.go.dev/net)
- [Wails (Web-style Desktop)](https://wails.io/)
