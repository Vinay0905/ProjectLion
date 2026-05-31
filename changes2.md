# Changes 2: Make Two Clients Chat Through the Server

## Goal

Right now, your app behaves like this:

```text
Client A ---> Server ---> Client A
```

The server receives a text message and sends it back to the same client. This is
called an **echo**.

The next step is to make the server act as a chat-room hub:

```text
Client A ---> Server ---> Client B
Client B ---> Server ---> Client A
```

Both clients still connect to the same server IP address and port. The clients
do not connect directly to each other.

For this step:

- Normal text messages will be sent to the other connected clients.
- File uploads will continue to go from one client to the server only.
- The GUI can wait until text chat works reliably between two clients.

## How the Server Will Remember Clients

The server needs a shared collection of active connections. In Go, a map is a
simple way to store them:

```go
var clients = make(map[net.Conn]bool)
```

Each key in the map is a client connection. The `bool` value is only a
placeholder. If a connection exists in the map, that client is connected.

Multiple goroutines can access this map at the same time because each client is
handled by its own goroutine. Go maps are not safe for simultaneous writes.
Use a mutex to ensure that only one goroutine changes or reads the map at a
time:

```go
var clientsMu sync.Mutex
```

## Step 1: Import `sync`

Open:

```text
lion_chat/cmd/server/main.go
```

Find the import block:

```go
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
```

Add `"sync"` after `"strings"`:

```go
import (
    "bufio"
    "fmt"
    "io"
    "log"
    "net"
    "strconv"
    "strings"
    "sync"

    "example.com/lion_chat/internal/protocol"
)
```

Why:

- The `sync` package provides `sync.Mutex`.
- The mutex prevents two client goroutines from changing the client map at the
  same time.

## Step 2: Add the Shared Client Map

Add this code below the import block and above `func main()`:

```go
var (
    clients   = make(map[net.Conn]bool)
    clientsMu sync.Mutex
)
```

Line-by-line explanation:

```go
var (
```

Starts a group of package-level variables. These variables are outside every
function, so all connection-handling goroutines can use them.

```go
clients = make(map[net.Conn]bool)
```

Creates an empty map. Each connected client will be stored using its
`net.Conn`.

```go
clientsMu sync.Mutex
```

Creates a mutex for the map.

## Step 3: Register and Remove Each Client

At the beginning of `handleConnection`, replace:

```go
func handleConnection(conn net.Conn) {
    defer conn.Close()
    log.Println("connected:", conn.RemoteAddr())
```

with:

```go
func handleConnection(conn net.Conn) {
    clientsMu.Lock()
    clients[conn] = true
    clientsMu.Unlock()

    defer func() {
        clientsMu.Lock()
        delete(clients, conn)
        clientsMu.Unlock()

        conn.Close()
        log.Println("disconnected:", conn.RemoteAddr())
    }()

    log.Println("connected:", conn.RemoteAddr())
```

Why the old line is replaced:

```go
defer conn.Close()
```

This only closed the TCP connection. The server now also needs to remove the
client from the shared map when the connection ends.

Line-by-line explanation:

```go
clientsMu.Lock()
clients[conn] = true
clientsMu.Unlock()
```

Locks the map, adds the new client, and unlocks the map.

```go
defer func() {
    ...
}()
```

Schedules cleanup for later. This cleanup runs automatically when
`handleConnection` returns because the client disconnects or an error occurs.

```go
delete(clients, conn)
```

Removes the disconnected client so the server does not try to send future
messages to a dead connection.

## Step 4: Add the Broadcast Function

Add this new function below `handleConnection`:

```go
func broadcast(sender net.Conn, message string) {
    clientsMu.Lock()
    defer clientsMu.Unlock()

    for client := range clients {
        if client == sender {
            continue
        }

        if _, err := fmt.Fprintln(client, message); err != nil {
            log.Println("broadcast error:", err)
        }
    }
}
```

Line-by-line explanation:

```go
func broadcast(sender net.Conn, message string)
```

Receives the connection that sent the message and the text to forward.

```go
clientsMu.Lock()
defer clientsMu.Unlock()
```

Locks the shared map while looping through it. `defer` guarantees that the map
is unlocked when the function finishes.

```go
for client := range clients {
```

Loops through every connected client.

```go
if client == sender {
    continue
}
```

Skips the sender. Without this check, the sender would receive its own message
back as an echo.

```go
fmt.Fprintln(client, message)
```

Writes the message to each other client's TCP connection.

## Step 5: Replace Echo With Broadcast

Find this section inside `handleConnection`:

```go
// Normal chat message
fmt.Println("message:", message)
if _, err := fmt.Fprintln(conn, message); err != nil {
    log.Println("write error:", err)
    return
}
```

Replace it with:

```go
// Normal chat message
fmt.Println("message:", message)
broadcast(conn, message)
```

Why:

- `fmt.Fprintln(conn, message)` writes only to the sender's connection.
- `broadcast(conn, message)` writes to every other connected client.

## Complete Server File After the Changes

Your `lion_chat/cmd/server/main.go` should look like this:

```go
package main

import (
    "bufio"
    "fmt"
    "io"
    "log"
    "net"
    "strconv"
    "strings"
    "sync"

    "example.com/lion_chat/internal/protocol"
)

var (
    clients   = make(map[net.Conn]bool)
    clientsMu sync.Mutex
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
    clientsMu.Lock()
    clients[conn] = true
    clientsMu.Unlock()

    defer func() {
        clientsMu.Lock()
        delete(clients, conn)
        clientsMu.Unlock()

        conn.Close()
        log.Println("disconnected:", conn.RemoteAddr())
    }()

    log.Println("connected:", conn.RemoteAddr())
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

            if err := protocol.ReceiveFile(reader, name, size); err != nil {
                log.Println("receiveFile error:", err)
                fmt.Fprintln(conn, "FILE_ERROR", err)
            } else {
                log.Printf("received file: %s (%d bytes)", name, size)
                fmt.Fprintf(conn, "FILE_RECEIVED %s (%d bytes)\n", name, size)
            }
            continue
        }

        fmt.Println("message:", message)
        broadcast(conn, message)
    }
}

func broadcast(sender net.Conn, message string) {
    clientsMu.Lock()
    defer clientsMu.Unlock()

    for client := range clients {
        if client == sender {
            continue
        }

        if _, err := fmt.Fprintln(client, message); err != nil {
            log.Println("broadcast error:", err)
        }
    }
}
```

## Step 6: Test With Two Clients

Restart the server after changing the code:

```bash
cd lion_chat
go run ./cmd/server
```

Run a client on laptop A:

```bash
cd lion_chat
go run ./cmd/client
```

Run another client on laptop B:

```bash
cd lion_chat
go run ./cmd/client
```

Both clients must connect to the server laptop's IP address:

```go
conn, err := net.Dial("tcp", "YOUR_SERVER_IP:9000")
```

Type this in client A:

```text
hello from A
```

Client B should print:

```text
peer: hello from A
```

Type this in client B:

```text
hello from B
```

Client A should print:

```text
peer: hello from B
```

The sender will not see its own message echoed back. This is expected.

## File Transfer During This Step

File upload still works as before:

```text
/send sample.txt
```

The file is saved in the server's `downloads/` directory. File forwarding from
one client to another is a later step because the server must safely stream the
file to each intended recipient.

## What Comes After This

Once two-way text chat works:

1. Add usernames so messages look like `vinay: hello`.
2. Decide whether chat is a shared room or a private one-to-one conversation.
3. Forward files from one client to another.
4. Add the GUI only after the terminal behavior is stable.
