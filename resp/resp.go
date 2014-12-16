// Package resp implements the REdis Serialization Protocol with the particular aim to communicate with
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

	"github.com/shipwire/swutil/swio"
)

// RedisType represents one of the five types RESP values may take or it is unknown or invalid.
type RedisType int

// RESP types
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

var redisTypeMap = map[string]RedisType{
	"+": SimpleStringType,
	"-": ErrorType,
	":": IntegerType,
	"$": BulkStringType,
	"*": ArrayType,
}

// String returns a string representation of r.
func (r RedisType) String() string {
	switch r {
	case SimpleStringType:
		return "simple string"
	case ErrorType:
		return "error"
	case IntegerType:
		return "integer"
	case BulkStringType:
		return "bulk string"
	case ArrayType:
		return "array"
	case NullType:
		return "null"
	default:
		return "invalid or unknown"
	}
}

// RESP errors
var (
	ErrInvalidResponse = errors.New("invalid response")
	ErrInvalidType     = errors.New("wrong redis type requested")
)

// RESP reads successive values in REdis Serialization Protocol. Values may be read
// by the method for their correct type. While type can be checked as many times as the caller
// wishes, each value may only be read once.
type RESP struct {
	r            io.Reader
	length       int64
	redisType    RedisType
	lastWasArray *Array
}

// New creates a new RESP value from the given reader.
func New(r io.Reader) *RESP {
	return &RESP{r: r}
}

// Type determines the redis type of a RESP.
func (r *RESP) Type() RedisType {
	if r.lastWasArray != nil {
		r.lastWasArray.cache()
		r.lastWasArray = nil
	}

	if r.redisType == UnknownType {
		firstByte := make([]byte, 1)
		n, err := r.r.Read(firstByte)
		if err != nil && err != io.EOF {
			return InvalidType
		}
		if n != 1 {
			return UnknownType
		}

		t := redisTypeMap[string(firstByte)]
		switch t {
		case BulkStringType, ArrayType:
			r.length, err = extractLength(r.r)
			if r.length == -1 {
				return NullType
			}
			fallthrough
		case SimpleStringType, IntegerType, ErrorType:
			r.redisType = t
		default:
			r.redisType = InvalidType
		}
	}
	return r.redisType
}

func (r *RESP) resetType() {
	r.redisType = UnknownType
}

func extractLength(r io.Reader) (i int64, err error) {
	_, err = fmt.Fscanf(r, "%d\r\n", &i)
	return
}

// SimpleString returns the value of a RESP as a simple string
func (r *RESP) SimpleString() (string, error) {
	if r.Type() != SimpleStringType {
		return "", ErrInvalidType
	}
	defer r.resetType()

	buf := &bytes.Buffer{}
	b := bufio.NewReader(r.r)

	expectNL := false
	for {
		ru, _, err := b.ReadRune()
		if err == io.EOF {
			return buf.String(), io.ErrUnexpectedEOF
		}
		if err != nil {
			return "", err
		}
		if ru == '\n' && expectNL {
			s := buf.String()
			// get buffered bytes back
			r.r = io.MultiReader(b, r.r)
			return s, nil
		}
		if expectNL {
			buf.WriteRune('\r')
			expectNL = false
		}
		if ru == '\r' {
			expectNL = true
			continue
		}
		buf.WriteRune(ru)
	}

	return buf.String(), io.ErrUnexpectedEOF
}

// Error returns the value of a RESP as an error
func (r *RESP) Error() error {
	if r.Type() != ErrorType {
		return ErrInvalidType
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
	if r.Type() != IntegerType {
		return 0, ErrInvalidType
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
	if r.Type() != BulkStringType {
		return nil, ErrInvalidType
	}
	defer r.resetType()

	head, tail := swio.ForkReader(r.r, int(r.length+2))
	r.r = tail
	return fastForwardReaderTail(head, r.length, 2), nil
}

// Array returns a channel on which callers can receive successive
// elements of the RESP array.
func (r *RESP) Array() (*Array, error) {
	if r.Type() != ArrayType {
		return nil, ErrInvalidType
	}

	elements := make(chan *RESP, 1)
	array := &Array{
		length: int(r.length),
		c:      elements,
	}

	r.resetType()
	go func(l int) {
		for readElems := 0; readElems < l; readElems++ {
			elem := New(r.r)
			elem.lastWasArray = r.lastWasArray
			switch elem.Type() {
			case SimpleStringType:
				s, _ := elem.SimpleString()
				r.r = elem.r
				er := bytes.NewBufferString(s)
				er.WriteString("\r\n")
				elem.r = er
				elem.redisType = SimpleStringType
			case ErrorType:
				e := elem.Error()
				elem.r = bytes.NewBufferString(e.Error())
				elem.redisType = ErrorType
			case IntegerType:
				i, _ := elem.Int()
				elem.r = bytes.NewBufferString(fmt.Sprint(i))
				elem.redisType = IntegerType
			case NullType:
				elem.redisType = NullType
			case BulkStringType:
				bs, _ := elem.BulkString()
				r.r = elem.r
				elem.r = io.MultiReader(bs, bytes.NewBufferString("\r\n"))
				elem.redisType = BulkStringType
			case ArrayType:
				r.lastWasArray = array
			}
			elements <- elem
		}
		close(elements)
	}(int(r.length))

	return array, nil
}

// String returns a string representation of r. It consumes the content.
func (r *RESP) String() string {
	switch r.Type() {
	case SimpleStringType:
		s, err := r.SimpleString()
		if err != nil {
			return err.Error()
		}
		return s
	case ErrorType:
		return r.Error().Error()
	case IntegerType:
		i, err := r.Int()
		if err != nil {
			return err.Error()
		}
		return fmt.Sprint(i)
	case BulkStringType:
		b, err := r.BulkString()
		if err != nil {
			return err.Error()
		}
		s, err := ioutil.ReadAll(b)
		if err != nil {
			return err.Error()
		}
		return string(s)
	case ArrayType:
		a, err := r.Array()
		if err != nil {
			return err.Error()
		}
		return a.String()
	case NullType:
		return "NULL"
	default:
		return "Invalid RESP format"
	}
}

// Array contains a sequence of RESP items.
type Array struct {
	length int
	c      <-chan *RESP
	cached []*RESP
}

// Next returns the next RESP item in the Array.
func (r *Array) Next() *RESP {
	if len(r.cached) > 0 {
		ret := r.cached[0]
		r.cached = r.cached[1:]
		return ret
	}
	elem := <-r.c
	return elem
}

// cache reads the all remaining elements and stores them in memory so subsequent items
// may also be read.
func (r *Array) cache() {
	for elem := range r.c {
		r.cached = append(r.cached, elem)
	}
}

// Len returns the total number of items in the Array.
func (r *Array) Len() int {
	return r.length
}

// String returns a string representation of r. It consumes all of r's elements.
func (r *Array) String() string {
	buf := bytes.NewBufferString("[")

	for i := 0; i < r.Len(); i++ {
		if i != 0 {
			buf.WriteString(",")
		}
		elem := r.Next()
		buf.WriteString(elem.String())
	}

	buf.WriteString("]")
	return buf.String()
}
