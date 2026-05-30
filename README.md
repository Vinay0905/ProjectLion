# Project LION

Project LION is a beginner-friendly local network messaging app written in Go.
The project is being built in small milestones: start with TCP text messages,
then add file transfer and an optional desktop interface.

## Current Status

The current implementation is a working TCP echo chat prototype:

- The server listens for TCP connections on port `9000`.
- Each connection is handled in its own goroutine.
- The client reads messages from the terminal and sends them to the server.
- The server prints each message and echoes it back to the same client.
- The client prints messages received from the server.

File transfer, multi-client broadcasting, and the desktop GUI are planned but
not implemented yet.

## Project Structure

```text
Project_LION/
  lion_chat/
    cmd/
      server/main.go
      client/main.go
    downloads/
    internal/
      protocol/
    go.mod
  intro/
    build_lion_chat_guide.py
    LocalNetworkMessengerFinal.docx
  messengerFinal.pdf
```

The generated DOCX and PDF files are ignored by Git. The tutorial builder in
`intro/build_lion_chat_guide.py` remains trackable so the guide can be
regenerated.

## Requirements

- Go installed on macOS, Linux, or Windows
- Two terminals for local testing
- Two computers on the same local network for LAN testing

Verify the Go installation:

```bash
go version
```

Official installation guide: [Install Go](https://go.dev/doc/install)

## Run the Text Chat Prototype

Open a terminal and start the server:

```bash
cd lion_chat
go run ./cmd/server
```

The server should print:

```text
server listening on :9000
```

Open a second terminal and run the client:

```bash
cd lion_chat
go run ./cmd/client
```

Type a message and press Return. The server prints the message and the client
receives the echoed response.

## Configure the Server Address

The client currently uses a LAN IP address directly in
`lion_chat/cmd/client/main.go`:

```go
conn, err := net.Dial("tcp", "192.168.29.203:9000")
```

For same-computer testing, replace the address with:

```go
conn, err := net.Dial("tcp", "localhost:9000")
```

For testing from another computer on the same Wi-Fi network, find the server
Mac's local IP address:

```bash
ipconfig getifaddr en0
```

Then update the client address to use that IP and port `9000`.

## Troubleshooting

- `connection refused`: start the server before the client and confirm both use
  port `9000`.
- `timeout`: confirm both computers are on the same local network and recheck
  the server IP address.
- No address from `en0`: run `ifconfig` and look for the active network adapter.
- macOS firewall prompt: allow Terminal or the compiled Go app to accept local
  connections during testing.

Do not expose this prototype directly to the internet. It does not include
authentication or encryption.

## Roadmap

1. Move shared message framing into `internal/protocol`.
2. Broadcast messages between multiple connected clients.
3. Add framed file transfer with filename validation, size limits, and safe
   writes into `downloads/`.
4. Add tests for malformed messages and interrupted transfers.
5. Add an optional desktop GUI with [Fyne](https://docs.fyne.io/).
6. Consider [Wails](https://wails.io/docs/gettingstarted/installation) later if
   a web-style desktop interface is useful.

## Tutorial Guide

A fuller beginner tutorial is included as generated documentation:

- `intro/LocalNetworkMessengerFinal.docx`
- `messengerFinal.pdf`

The guide covers Go setup, TCP server-client basics, LAN testing, a simple file
transfer design, safety checks, and GUI options.

## References

- [Install Go](https://go.dev/doc/install)
- [Go `net` package](https://pkg.go.dev/net)
- [Fyne documentation](https://docs.fyne.io/)
- [Wails installation guide](https://wails.io/docs/gettingstarted/installation)
