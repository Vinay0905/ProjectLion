package protocol

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

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

func ReceiveFile(reader io.Reader, receivedName string, size int64) error {
	safeName := filepath.Base(receivedName)
	if safeName != receivedName || safeName == "." {
		return errors.New("unsafe filename")
	}
	if size <= 0 || size > 50<<20 {
		return errors.New("file size must be between 1 byte and 50 MB")
	}
	if err := os.MkdirAll("downloads", 0755); err != nil {
		return err
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

	_, err = io.CopyN(out, reader, size)
	return err
}
