// Resp implements the REdis Serialization Protocol with the particular aim to communicate with
// Redis. See http://redis.io/topics/protocol for more information.
package resp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"

	"bitbucket.org/shipwire/forkedReader"
)

// RedisType represents one of the five types RESP values may take or it is unknown or invalid.
type RedisType int

const (
	Unknown RedisType = iota
	Invalid
	SimpleString
	Error
	Integer
	BulkString
	Array
	Null
)

var redisTypeMap map[string]RedisType = map[string]RedisType{
	"+": SimpleString,
	"-": Error,
	":": Integer,
	"$": BulkString,
	"*": Array,
}

var (
	InvalidResponse error = errors.New("invalid response")
	InvalidType           = errors.New("wrong redis type requested")
)

// RESP reads successive values in REdis Serialization Protocol. Values may be read
// by the method for their correct type. While type can be checked as many times as the caller
// wishes, each value may only be read once.
type RESP struct {
	r         io.Reader
	length    int64
	redisType RedisType
}

// New creates a new RESP value from the given reader.
func New(r io.Reader) *RESP {
	return &RESP{r: r}
}

// Type determines the redis type of a RESP.
func (r *RESP) Type() RedisType {
	if r.redisType == Unknown {
		firstByte := make([]byte, 1)
		n, err := io.ReadFull(r.r, firstByte)
		if err != nil {
			return Invalid
		}
		if n != 1 {
			return Unknown
		}

		t := redisTypeMap[string(firstByte)]
		switch t {
		case BulkString, Array:
			r.length, err = extractLength(r.r)
			if r.length == -1 {
				return Null
			}
			fallthrough
		case SimpleString, Integer, Error:
			r.redisType = t
		default:
			r.redisType = Invalid
		}
	}
	return r.redisType
}

func (r *RESP) resetType() {
	r.redisType = Unknown
}

func extractLength(r io.Reader) (i int64, err error) {
	_, err = fmt.Fscanf(r, "%d\r\n", &i)
	return
}

// SimpleString returns the value of a RESP as a simple string
func (r *RESP) SimpleString() (string, error) {
	if r.Type() != SimpleString {
		return "", InvalidType
	}
	defer r.resetType()

	scanner := bufio.NewScanner(r.r)
	if !scanner.Scan() {
		return "", scanner.Err()
	}
	return scanner.Text(), nil
}

// Error returns the value of a RESP as an error
func (r *RESP) Error() error {
	if r.Type() != Error {
		return InvalidType
	}
	defer r.resetType()

	scanner := bufio.NewScanner(r.r)
	if !scanner.Scan() {
		return scanner.Err()
	}
	return errors.New(scanner.Text())
}

// Int returns the value of a RESP as an integer.
func (r *RESP) Int() (int64, error) {
	if r.Type() != Integer {
		return 0, InvalidType
	}
	defer r.resetType()

	scanner := bufio.NewScanner(r.r)
	if !scanner.Scan() {
		return 0, scanner.Err()
	}
	return strconv.ParseInt(scanner.Text(), 10, 64)
}

// BulkString returns the value of a RESP as a reader.
func (r *RESP) BulkString() (io.Reader, error) {
	if r.Type() != BulkString {
		return nil, InvalidType
	}
	defer r.resetType()

	head, tail := fork.ForkReader(r.r, int(r.length+2))
	r.r = tail
	return io.LimitReader(head, r.length), nil
}

// Array returns a channel on which callers can receive successive
// elements of the RESP array.
func (r *RESP) Array() (*RESPArray, error) {
	if r.Type() != Array {
		return nil, InvalidType
	}
	defer r.resetType()

	elements := make(chan *RESP)
	array := &RESPArray{
		length: int(r.length),
		c:      elements,
	}

	go func() {
		for err := (error)(nil); err == nil; {
			elem := New(r.r)
			elements <- elem
		}
		close(elements)
	}()

	return array, nil
}

func (r *RESP) String() string {
	switch r.Type() {
	case SimpleString:
		s, err := r.SimpleString()
		if err != nil {
			return err.Error()
		}
		return s
	case Error:
		return r.Error().Error()
	case Integer:
		i, err := r.Int()
		if err != nil {
			return err.Error()
		}
		return fmt.Sprint(i)
	case BulkString:
		b, err := r.BulkString()
		if err != nil {
			return err.Error()
		}
		s, err := ioutil.ReadAll(b)
		if err != nil {
			return err.Error()
		}
		return string(s)
	case Array:
		a, err := r.Array()
		if err != nil {
			return err.Error()
		}
		return a.String()
	case Null:
		return "NULL"
	default:
		return "Invalid RESP format"
	}
}

// RESPArray contains a sequence of RESP items.
type RESPArray struct {
	length int
	c      <-chan *RESP
	cached []*RESP
}

// Next returns the next RESP item in the RESPArray.
func (r *RESPArray) Next() *RESP {
	if len(r.cached) > 0 {
		ret := r.cached[0]
		r.cached = r.cached[1:]
		return ret
	}
	return <-r.c
}

// Cache reads the next RESP item and stores it in memory so subsequent items
// may also be read.
func (r *RESPArray) Cache() {
	if elem := <-r.c; elem != nil {
		r.cached = append(r.cached, elem)
	}
}

// Len returns the total number of items in the RESPArray.
func (r *RESPArray) Len() int {
	return r.length
}

func (r *RESPArray) String() string {
	buf := bytes.NewBufferString("[")
	buf.WriteString(r.Next().String())

	first := false
	for elem := r.Next(); elem != nil; r.Next() {
		if !first {
			buf.WriteString(",")
		}
		buf.WriteString(elem.String())
	}

	buf.WriteString("]")
	return buf.String()
}
