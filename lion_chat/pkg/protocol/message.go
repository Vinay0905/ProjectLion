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
