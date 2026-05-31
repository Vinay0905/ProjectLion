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
	"strconv"
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
	log.Println("connected:", conn.RemoteAddr())
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		message := scanner.Text()

		// Handle file metadata
		if strings.HasPrefix(message, "FILE_META ") {
			// message: FILE_META name|size
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

			// Next line should be FILE_DATA <size>
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					log.Println("read error while waiting for FILE_DATA:", err)
				}
				break
			}
			dataLine := scanner.Text()
			if dataLine != fmt.Sprintf("FILE_DATA %d", size) {
				log.Println("unexpected FILE_DATA line:", dataLine)
				continue
			}

			// Now read exactly `size` bytes from conn into downloads/
			if err := receiveFile(conn, name, size); err != nil {
				log.Println("receiveFile error:", err)
			}
			continue
		}

		// Normal chat message
		fmt.Println("message:", message)
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
