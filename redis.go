// Redis implements basic connections and pooling to redis servers.
package redis

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"bytes"

	"bitbucket.org/shipwire/redis/resp"
)

// Conn represents an open connection to a redis server.
type Conn struct {
	net.Conn
	whence       *Pool
	openCommands int
	commandLock  *sync.Mutex
	reply        *resp.RESP
}

// Dial connects to the redis server.
func Dial(network, address string) (*Conn, error) {
	c, err := net.Dial(network, address)
	conn := &Conn{Conn: c, commandLock: &sync.Mutex{}}
	conn.reply = resp.New(c)
	return conn, err
}

// DialTimeout acts like Dial but takes a timeout. The timeout includes name resolution, if required.
func DialTimeout(network, address string, timeout time.Duration) (*Conn, error) {
	c, err := net.DialTimeout(network, address, timeout)
	conn := &Conn{Conn: c, commandLock: &sync.Mutex{}}
	conn.reply = resp.New(c)
	return conn, err
}

// RawCmd sends a raw command to the redis server
func (c *Conn) RawCmd(command string, args ...string) error {
	cmd, err := c.Command(command, len(args))
	if err != nil {
		return err
	}
	defer cmd.Close()

	for _, arg := range args {
		_, err := cmd.WriteArgumentString(arg)
		if err != nil {
			c.Destroy()
			return err
		}
	}
	return nil
}

// Command initializes a command with the given number of arguments. The connection
// only allows one open command at a time and will block callers to prevent jumbled queues.
func (c *Conn) Command(command string, args int) (*Cmd, error) {
	c.commandLock.Lock()
	c.openCommands += 1

	fmt.Fprintf(c, "*%d\r\n", args+1)
	cmd := &Cmd{c, c}
	cmd.WriteArgumentString(command)
	return cmd, nil
}

// Close either releases the connection back into the pool from whence it came, or, it
// actually destroys the connection.
func (c *Conn) Close() error {
	if c.whence != nil {
		return c.whence.put(c)
	} else {
		return c.Destroy()
	}
}

// Destory always destroys the connection.
func (c *Conn) Destroy() error {
	return c.Conn.Close()
}

// Resp reads a RESP from the connection
func (c *Conn) Resp() *resp.RESP {
	defer func() {
		c.openCommands -= 1
	}()
	return c.reply
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
