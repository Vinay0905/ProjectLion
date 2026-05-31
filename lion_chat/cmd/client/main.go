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
)

func main() {
	conn, err := net.Dial("tcp", "192.168.29.203:9000")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	fmt.Println("connected; type a message and press Return")
	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			fmt.Println("\npeer:", scanner.Text())
			fmt.Print("> ")

		}
		if err := scanner.Err(); err != nil {
			log.Println("read error from server:", err)
		}
	}()
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		line := input.Text()

		// if user types: /send path/to/file
		if len(line) > 6 && line[:6] == "/send " {
			path := line[6:]
			if err := sendFile(conn, path); err != nil {
				log.Println("sendFile error:", err)
			}
			continue
		}

		if _, err := fmt.Fprintln(conn, input.Text()); err != nil {
			log.Fatal(err)
		}
	}
	if err := input.Err(); err != nil {
		log.Println("read error from server:", err)
	}

}
func sendFile(conn net.Conn, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	// CHANGE 1: Check file.Stat before using info so an invalid file cannot panic.
	if err != nil {
		return err
	}
	name := filepath.Base(info.Name())
	// CHANGE 5: Reject empty files so a mistaken blank source file is obvious.
	if info.Size() == 0 {
		return fmt.Errorf("cannot send empty file %q", path)
	}
	sizeStr := strconv.FormatInt(info.Size(), 10)
	if _, err := fmt.Fprintf(conn, "FILE_META %s|%s\n", name, sizeStr); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(conn, "FILE_DATA %d\n", info.Size()); err != nil {
		return err
	}
	// fmt.Fprintf(conn, "FILE_DATA %d\n%s|%d", len(name)+1+len(strconv.FormatInt(info.Size(), 10)), name, info.Size())
	// fmt.Fprintf(conn, "FILE_DATA %d\n", info.Size())

	_, err = io.Copy(conn, file)
	return err

}
