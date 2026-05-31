# Line-by-Line Explanation of `message.go`

This file is part of the custom communication protocol (called the **LION protocol**) used by our chat program. Its main job is to define helper functions to read and write messages over a network connection.

---

### Code Breakdown

#### Line 1: Package Declaration
```go
package protocol
```
* **Explanation:** Every Go file must start with a `package` declaration. It tells Go which folder/group this file belongs to. Here, it is part of the `protocol` package, which holds all the rules for how our client and server talk to each other.

---

#### Lines 3–8: Importing external packages
```go
import (
	"bufio"
	"fmt"
	"io"
	"strings"
)
```
* **Explanation:** In Go, if you want to use features built by other people or Go's standard library, you must "import" them.
  * `"bufio"` (Buffered I/O): Helps read data from the network efficiently, line-by-line.
  * `"fmt"` (Format): Used for formatting and printing text (like writing text to a connection).
  * `"io"` (Input/Output): Provides basic tools for reading and writing data stream by stream.
  * `"strings"` (Strings): Contains helpful functions for manipulating text (like removing extra spaces).

---

#### Lines 10–14: Defining the `Frame` Structure
```go
type Frame struct {
	Type    string
	Size    int64
	Payload io.Reader
}
```
* **Explanation:** A `struct` (short for structure) is like a blueprint or a custom container. It lets us group different pieces of information together. Here, we define a container named `Frame` that represents a structured message sent over the network:
  * `Type`: A text label describing what kind of data it is (for example, "TEXT" or "FILE_META").
  * `Size`: A large integer number representing the size of the data in bytes.
  * `Payload`: The actual content/body of the message itself. `io.Reader` means "anything we can read data from" (like a network connection or a file).

---

#### Lines 16–19: The `WriteText` Function
```go
func WriteText(w io.Writer, message string) error {
	_, err := fmt.Fprintf(w, "TEXT %d\n%s", len(message), message)
	return err
}
```
* **Explanation:** This function helps us send a plain text message.
  * `func WriteText(...)`: Declares a new function.
  * `w io.Writer`: The first parameter is where we want to write the data (like a network connection).
  * `message string`: The second parameter is the text we want to send.
  * `error`: The return type. It returns an error if something goes wrong (e.g., if the network connection breaks).
  * `fmt.Fprintf(...)`: Formats our message into a standard format: `TEXT <length>\n<message_content>`. For example, sending `"hello"` becomes `"TEXT 5\nhello"`.
  * `_, err := ...`: In Go, functions often return multiple values. `fmt.Fprintf` returns the number of bytes written (which we don't care about, so we use `_` to ignore it) and an error value (which we save in `err`).
  * `return err`: Sends the error back to whoever called this function. If there was no error, `err` will be `nil` (which means empty/null).

---

#### Lines 21–28: The `ReadHeader` Function
```go
func ReadHeader(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(line), nil
}
```
* **Explanation:** This function reads the very first line of any message coming from the network connection.
  * `func ReadHeader(...) (string, error)`: This function takes a network reader (`*bufio.Reader`) and returns two things: the header text (`string`) and an error if something fails.
  * `reader.ReadString('\n')`: Reads data from the network until it encounters a newline character (`\n`). This represents the header line.
  * `if err != nil { return "", err }`: In Go, we handle errors immediately. If reading from the network failed (for example, the other person disconnected), we stop and return the error.
  * `strings.TrimSpace(line)`: Cleans up the text by removing trailing newlines (`\n`, `\r`) or extra spaces at the ends.
  * `return ..., nil`: If everything went well, we return the cleaned-up header line and `nil` (meaning no error occurred).
