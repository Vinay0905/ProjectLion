package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
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
			log.Println("accept error: ", err)
			continue
		}
		go handleConnection(conn)
	}
}
func handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Println("connected: ", conn.RemoteAddr())
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		message := scanner.Text()

		if strings.HasPrefix(message, "FILE_SEND") {
			var name string
			var size int64
			_, err := fmt.Sscanf(message, "FILE_SEND %s %d", &name, &size)
			if err != nil {
				log.Println("Bad file command", err)
				continue
			}
			if err := receiveFile(conn, name, size); err != nil {
				log.Println("recieved file error:", err)
			}
			continue

		}

		fmt.Println("message:", message)

		// echo back to the same client:
		if _, err := fmt.Fprintln(conn, message); err != nil {
			log.Println("write error:", err)
			return
		}
	}
	if err := scanner.Err(); err != nil {
		log.Println("read error:", err)
	}
}
func receiveFile(conn net.Conn, receivedName string, size int64) error {
	safeName := filepath.Base(receivedName)
	if safeName != receivedName || safeName == "." {
		return errors.New("unsafe filename")
	}
	if size < 0 || size > 50<<20 {
		return errors.New("File too large")
	}
	dst := filepath.Join("downloads", safeName)
	if _, err := os.Stat(dst); err == nil {
		return errors.New("file already exists")
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.CopyN(out, conn, size)
	return err
}
