// Redis implements basic connections and pooling to redis servers.
package redis

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"bytes"

	"bitbucket.org/shipwire/redis/resp"
)

var CommandInProgress = errors.New("command in progress")

// Conn represents an open connection to a redis server.
type Conn struct {
	net.Conn
	whence       *Pool
	openCommands int
	commandLock  *sync.Mutex
}

// Dial connects to the redis server.
func Dial(network, address string) (*Conn, error) {
	c, err := net.Dial(network, address)
	return &Conn{Conn: c}, err
}

// DialTimeout acts like Dial but takes a timeout. The timeout includes name resolution, if required.
func DialTimeout(network, address string, timeout time.Duration) (*Conn, error) {
	c, err := net.DialTimeout(network, address, timeout)
	return &Conn{Conn: c}, err
}

// RawCmd sends a raw command to the redis server
func (c *Conn) RawCmd(command string, args ...string) error {
	c.commandLock.Lock()
	defer c.commandLock.Unlock()

	c.openCommands += 1

	cmd, err := c.Command(command, len(args))
	if err != nil {
		return err
	}

	for _, arg := range args {
		cmd.WriteArgumentString(arg)
	}
	return cmd.Close()
}

// Command initializes a command with the given number of arguments. The connection
// only allows one open command at a time and will block callers to prevent jumbled queues.
func (c *Conn) Command(command string, args int) (*Cmd, error) {
	c.commandLock.Lock()

	fmt.Fprintf(c, "*%d\r\n$%d\r\n", args, len(command))
	return &Cmd{c, c}, nil
}

// Resp reads a RESP from the connection
func (c *Conn) Resp() *resp.RESP {
	defer func() {
		c.openCommands -= 1
	}()
	return resp.New(c)
}

// Cmd is a command that is currently being written to a connection.
type Cmd struct {
	io.Writer
	conn *Conn
}

// WriteArgumentString writes a static string to the connection as a command argument.
func (c Cmd) WriteArgumentString(arg string) (int, error) {
	return fmt.Fprintf(c, "$%d\r\n%s\r\n", len(arg), arg)
}

// WriteArgument is a shortcut method to write a reader to a command. If at all possible,
// WriteArgumentLength should be used instead.
func (c Cmd) WriteArgument(r io.Reader) (int, error) {
	buf := &bytes.Buffer{}
	io.Copy(buf, r)
	return fmt.Fprintf(c, "$%d\r\n%s\r\n", buf.Len(), buf.String())
}

// WriteArgumentLength copies a reader as an argument to a command. It expects the reader
// to be of the given length.
func (c Cmd) WriteArgumentLength(r io.Reader, l int64) (written int, err error) {
	r = io.LimitReader(r, l)
	w, err := fmt.Fprintf(c, "$%d\r\n", l)
	if err != nil {
		return
	}
	written += w
	ww, err := io.Copy(c, r)
	if err != nil {
		return
	}
	written += int(ww)
	w, err = fmt.Fprint(c, "\r\n")
	if err != nil {
		return
	}
	written += w
	return
}

// Close closes the command with a CRLF.
func (c Cmd) Close() error {
	defer c.conn.commandLock.Unlock()
	_, err := io.WriteString(c, "\r\n")
	return err
}
