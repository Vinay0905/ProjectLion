# Line-by-Line Explanation of `file.go`

This file handles sending and receiving files over the network connection. It implements safety checks (like ensuring file sizes aren't too large) and ensures files are saved to the correct location.

---

### Code Breakdown

#### Line 1: Package Declaration
```go
package protocol
```
* **Explanation:** Tells Go that this file is part of the `protocol` package.

---

#### Lines 3–9: Importing Packages
```go
import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)
```
* **Explanation:**
  * `"errors"`: Used to create custom error messages (e.g., when a file is too large).
  * `"fmt"`: Used to format string messages.
  * `"io"`: Input/Output utilities, used here to copy bytes from files to the network, or from the network to local files.
  * `"os"`: Operating System tools. Used to open, read, and write actual files on your computer's hard drive.
  * `"path/filepath"`: Used to work with file paths safely on different operating systems (Windows uses `\`, macOS/Linux uses `/`).

---

#### Lines 11–12: The `SendFile` Function Declaration
```go
// SendFile reads a file from path and writes it to w using the LION protocol framing.
func SendFile(w io.Writer, path string) error {
```
* **Explanation:** This function takes a destination to write to (`w`, like a network connection) and a file path (`path`, like `"my_picture.png"`) on your computer, and sends it.

---

#### Lines 13–17: Opening the file
```go
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
```
* **Explanation:**
  * `os.Open(path)`: Tries to open the file. If it doesn't exist, we get an error.
  * `if err != nil { return err }`: If opening the file failed, stop and return the error.
  * `defer file.Close()`: The keyword `defer` tells Go to close the file *automatically* when this function finishes, no matter how it exits. This is important to free up computer resources.

---

#### Lines 19–22: Getting file metadata
```go
	info, err := file.Stat()
	if err != nil {
		return err
	}
```
* **Explanation:**
  * `file.Stat()`: Gets info about the file (like its size and actual filename).
  * If getting info fails, we return the error.

---

#### Lines 24–27: Getting the clean name and checking if empty
```go
	name := filepath.Base(info.Name())
	if info.Size() == 0 {
		return fmt.Errorf("cannot send empty file %q", path)
	}
```
* **Explanation:**
  * `filepath.Base(...)`: Extracts just the filename from a full path (e.g., converting `/Users/mast/doc.txt` to just `doc.txt`).
  * `info.Size() == 0`: If the file has 0 bytes (it's empty), we stop and return a formatted error, because sending empty files is not allowed.

---

#### Lines 29–31: Sending the File Metadata Header
```go
	if _, err := fmt.Fprintf(w, "FILE_META %s|%d\n", name, info.Size()); err != nil {
		return err
	}
```
* **Explanation:** We write the metadata to the network: `FILE_META <filename>|<filesize>\n`. This warns the receiver that a file named `<filename>` of size `<filesize>` is coming next.

---

#### Lines 33–35: Sending the File Data Header
```go
	if _, err := fmt.Fprintf(w, "FILE_DATA %d\n", info.Size()); err != nil {
		return err
	}
```
* **Explanation:** We write a secondary header `FILE_DATA <size>\n` right before the actual raw bytes start.

---

#### Lines 37–38: Copying the file data to the network connection
```go
	_, err = io.Copy(w, file)
	return err
}
```
* **Explanation:**
  * `io.Copy(w, file)`: Reads all the bytes from the open local `file` and writes them directly to the connection `w`.
  * We return whatever error `io.Copy` might produce. If it succeeds, it returns `nil`.

---

---

#### Lines 44: The `ReceiveFile` Function Declaration
```go
func ReceiveFile(reader io.Reader, receivedName string, size int64) error {
```
* **Explanation:** This function reads incoming file bytes from the network connection `reader`, and writes them to a file named `receivedName` in the `downloads/` directory.

---

#### Lines 45–47: Checking the file size limit
```go
	if size <= 0 || size > 50<<20 {
		return errors.New("file size must be between 1 byte and 50 MB")
	}
```
* **Explanation:**
  * `50 << 20` is a bitwise operation that evaluates to exactly 50 Megabytes ($50 \times 1024 \times 1024$ bytes).
  * We make sure the file is not empty (size $\le 0$) and not larger than 50 MB. If it is, we reject it immediately with an error.

---

#### Lines 49–53: Helper function to discard extra data
```go
	// Helper to consume and discard the payload bytes in case of error.
	// This keeps the TCP buffer synchronized for subsequent commands.
	discardPayload := func() {
		_, _ = io.CopyN(io.Discard, reader, size)
	}
```
* **Explanation:**
  * This defines an inline helper function `discardPayload`.
  * If we encounter an error halfway through (for example, the file name is unsafe or already exists), we can't just stop reading. The sender is already sending the file bytes over the wire. If we stop reading, the leftover bytes will get mixed up with the next chat messages.
  * `io.CopyN(io.Discard, reader, size)`: Read the expected number of bytes and throw them away (`io.Discard` is like a black hole) to clear the pipe.

---

#### Lines 55–59: Verifying the filename is safe
```go
	safeName := filepath.Base(receivedName)
	if safeName != receivedName || safeName == "." {
		discardPayload()
		return errors.New("unsafe filename")
	}
```
* **Explanation:**
  * A security check! We make sure the user isn't trying to send a file named `../../etc/passwd` or something that could overwrite important files on your computer.
  * `filepath.Base` extracts just the ending name. If that doesn't match the original, it means the name contains folders or paths, which we reject as `"unsafe filename"`.
  * Before returning the error, we call `discardPayload()` to empty the incoming data from the connection.

---

#### Lines 61–64: Creating the downloads directory
```go
	if err := os.MkdirAll("downloads", 0755); err != nil {
		discardPayload()
		return err
	}
```
* **Explanation:**
  * `os.MkdirAll("downloads", 0755)`: Creates a folder named `downloads/` if it doesn't already exist.
  * `0755` is the file permission code (readable/executable by everyone, writeable only by the owner).

---

#### Lines 66–70: Checking if file already exists
```go
	dst := filepath.Join("downloads", safeName)
	if _, err := os.Stat(dst); err == nil {
		discardPayload()
		return errors.New("file already exists")
	}
```
* **Explanation:**
  * `filepath.Join("downloads", safeName)`: Creates the final save path, e.g., `"downloads/photo.jpg"`.
  * `os.Stat(dst)`: Checks if the file already exists. If `err == nil`, it means the file exists. To prevent overwriting, we discard the payload and return `"file already exists"`.

---

#### Lines 72–77: Creating the new local file
```go
	out, err := os.Create(dst)
	if err != nil {
		discardPayload()
		return err
	}
	defer out.Close()
```
* **Explanation:**
  * `os.Create(dst)`: Creates the new file on disk.
  * `defer out.Close()`: Ensures the file is closed when this function finishes.

---

#### Lines 79–80: Copying bytes from network to local file
```go
	_, err = io.CopyN(out, reader, size)
	return err
}
```
* **Explanation:**
  * `io.CopyN(out, reader, size)`: Read exactly `size` bytes from the network connection `reader` and write them directly into our local file `out`.
  * This writes the actual contents of the file onto your computer. We return any error that happens here.
