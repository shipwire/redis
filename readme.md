# redis
--
    import "github.com/shipwire/redis"

Package redis implements basic connections and pooling to redis servers.

This package operates with streams of data (io.Reader). As necessry the package
will cache data locally before read by clients, for example when reading
successive elements of an array before consuming each element's contents.

## Usage

#### type Cmd

```go
type Cmd struct {
	io.Writer
}
```

Cmd is a command that is currently being written to a connection.

#### func (Cmd) Close

```go
func (c Cmd) Close() error
```
Close closes the command with a CRLF.

#### func (Cmd) WriteArgument

```go
func (c Cmd) WriteArgument(r io.Reader) (int, error)
```
WriteArgument is a shortcut method to write a reader to a command. If at all
possible, WriteArgumentLength should be used instead.

#### func (Cmd) WriteArgumentLength

```go
func (c Cmd) WriteArgumentLength(r io.Reader, l int64) (written int, err error)
```
WriteArgumentLength copies a reader as an argument to a command. It expects the
reader to be of the given length.

#### func (Cmd) WriteArgumentString

```go
func (c Cmd) WriteArgumentString(arg string) (int, error)
```
WriteArgumentString writes a static string to the connection as a command
argument.

#### type Conn

```go
type Conn struct {
}
```

Conn represents an open connection to a redis server.

#### func  Dial

```go
func Dial(network, address string) (*Conn, error)
```
Dial connects to the redis server.

#### func  DialTimeout

```go
func DialTimeout(network, address string, timeout time.Duration) (*Conn, error)
```
DialTimeout acts like Dial but takes a timeout. The timeout includes name
resolution, if required.

#### func (*Conn) Close

```go
func (c *Conn) Close() error
```
Close either releases the connection back into the pool from whence it came, or,
it actually destroys the connection.

#### func (*Conn) Command

```go
func (c *Conn) Command(command string, args int) (*Cmd, error)
```
Command initializes a command with the given number of arguments. The connection
only allows one open command at a time and will block callers to prevent jumbled
queues.

#### func (*Conn) Destroy

```go
func (c *Conn) Destroy() error
```
Destroy always destroys the connection.

#### func (*Conn) RawCmd

```go
func (c *Conn) RawCmd(command string, args ...string) error
```
RawCmd sends a raw command to the redis server

#### func (*Conn) Resp

```go
func (c *Conn) Resp() *resp.RESP
```
Resp reads a RESP from the connection

#### func (*Conn) Subscribe

```go
func (c *Conn) Subscribe(channel string, messages chan<- *resp.RESP) error
```
Subscribe listens on c for published messages on the a channel. This method will
either return an error right away or block while sending received messages on
the messges channel until it receives a signal on the done channel. This
connection should not be reused for another purpose.

#### func (*Conn) Unsubscribe

```go
func (c *Conn) Unsubscribe(channel string, ch chan<- *resp.RESP)
```
Unsubscribe unregisters a channel from receiving messages on a redis channel.
After unsubscribe returns, it is guaranteed that ch will receive no more
messages.

#### type Pool

```go
type Pool struct {
	Network, Server string

	// Password indicates that new connections should initially authenticate
	// with the given password. Password is ignored if empty.
	Password string

	// MaxIdle indicates how many idle connections should be kept. Zero indicates
	// all idle connections should be preserved.
	MaxIdle     int
	ConnTimeout time.Duration
}
```

Pool maintains a collection of idle Redis connections.

#### func  NewPool

```go
func NewPool() *Pool
```
NewPool intializes a connection pool with default settings.

#### func (*Pool) Conn

```go
func (p *Pool) Conn() (*Conn, error)
```
Conn attempts to get or create a connection, depending on if there are any idle
connections.

#### func (*Pool) Subscribe

```go
func (p *Pool) Subscribe(channel string, ch chan<- *resp.RESP) error
```
Subscribe registers a channel of RESP values to a redis pubsub channel.

#### func (*Pool) Unsubscribe

```go
func (p *Pool) Unsubscribe(channel string, ch chan<- *resp.RESP) error
```
Unsubscribe unregisters a channel of RESP values from a redis pubsub channel. If
and after Unsubscribe returns with no error, it is guaranteed that ch will
receive no more messages.

# resp
--
    import "github.com/shipwire/redis/resp"

Package resp implements the REdis Serialization Protocol with the particular aim
to communicate with Redis. See http://redis.io/topics/protocol for more
information.

## Usage

```go
var (
	ErrInvalidResponse = errors.New("invalid response")
	ErrInvalidType     = errors.New("wrong redis type requested")
)
```
RESP errors

#### type Array

```go
type Array struct {
}
```

Array contains a sequence of RESP items.

#### func (*Array) Len

```go
func (r *Array) Len() int
```
Len returns the total number of items in the Array.

#### func (*Array) Next

```go
func (r *Array) Next() *RESP
```
Next returns the next RESP item in the Array.

#### func (*Array) String

```go
func (r *Array) String() string
```
String returns a string representation of r. It consumes all of r's elements.

#### type RESP

```go
type RESP struct {
}
```

RESP reads successive values in REdis Serialization Protocol. Values may be read
by the method for their correct type. While type can be checked as many times as
the caller wishes, each value may only be read once.

#### func  New

```go
func New(r io.Reader) *RESP
```
New creates a new RESP value from the given reader.

#### func (*RESP) Array

```go
func (r *RESP) Array() (*Array, error)
```
Array returns a channel on which callers can receive successive elements of the
RESP array.

#### func (*RESP) BulkString

```go
func (r *RESP) BulkString() (io.Reader, error)
```
BulkString returns the value of a RESP as a reader.

#### func (*RESP) Error

```go
func (r *RESP) Error() error
```
Error returns the value of a RESP as an error

#### func (*RESP) Int

```go
func (r *RESP) Int() (int64, error)
```
Int returns the value of a RESP as an integer.

#### func (*RESP) SimpleString

```go
func (r *RESP) SimpleString() (string, error)
```
SimpleString returns the value of a RESP as a simple string

#### func (*RESP) String

```go
func (r *RESP) String() string
```
String returns a string representation of r. It consumes the content.

#### func (*RESP) Type

```go
func (r *RESP) Type() RedisType
```
Type determines the redis type of a RESP.

#### type RedisType

```go
type RedisType int
```

RedisType represents one of the five types RESP values may take or it is unknown
or invalid.

```go
const (
	UnknownType RedisType = iota
	InvalidType
	SimpleStringType
	ErrorType
	IntegerType
	BulkStringType
	ArrayType
	NullType
)
```
RESP types

#### func (RedisType) String

```go
func (r RedisType) String() string
```
String returns a string representation of r.
