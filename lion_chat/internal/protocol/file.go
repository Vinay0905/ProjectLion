package protocol

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SendFile reads a file from path and writes it to w using the LION protocol framing.
func SendFile(w io.Writer, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	name := filepath.Base(info.Name())
	if info.Size() == 0 {
		return fmt.Errorf("cannot send empty file %q", path)
	}

	if _, err := fmt.Fprintf(w, "FILE_META %s|%d\n", name, info.Size()); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "FILE_DATA %d\n", info.Size()); err != nil {
		return err
	}

	_, err = io.Copy(w, file)
	return err
}

// ReceiveFile reads size bytes from reader and saves them under receivedName in downloads/.
// If an error is encountered after size validation, it safely discards size bytes to keep
// the TCP socket connection synchronized.
func ReceiveFile(reader io.Reader, receivedName string, size int64) error {
	if size <= 0 || size > 50<<20 {
		return errors.New("file size must be between 1 byte and 50 MB")
	}

	// Helper to consume and discard the payload bytes in case of error.
	// This keeps the TCP buffer synchronized for subsequent commands.
	discardPayload := func() {
		_, _ = io.CopyN(io.Discard, reader, size)
	}

	safeName := filepath.Base(receivedName)
	if safeName != receivedName || safeName == "." {
		discardPayload()
		return errors.New("unsafe filename")
	}

	if err := os.MkdirAll("downloads", 0755); err != nil {
		discardPayload()
		return err
	}

	dst := filepath.Join("downloads", safeName)
	if _, err := os.Stat(dst); err == nil {
		discardPayload()
		return errors.New("file already exists")
	}

	out, err := os.Create(dst)
	if err != nil {
		discardPayload()
		return err
	}
	defer out.Close()

	_, err = io.CopyN(out, reader, size)
	return err
}
