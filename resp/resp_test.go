package resp

import (
	"bytes"
	"errors"
	"io/ioutil"
	"strconv"
	"testing"
)

type test struct {
	name     string
	resp     *bytes.Buffer
	expected interface{}
	typ      RedisType
}

var tests = []test{
	{
		name:     "null bulk string",
		resp:     bytes.NewBufferString("$-1\r\n"),
		typ:      NullType,
		expected: nil,
	},
	{
		name:     "null array",
		resp:     bytes.NewBufferString("*-1\r\n"),
		typ:      NullType,
		expected: nil,
	},
}

func TestSimpleString(t *testing.T) {
	r := New(bytes.NewBufferString("+i'm a simple string\r\n"))
	expected := "i'm a simple string"

	if r.Type() != SimpleStringType {
		t.Fatalf("Expected type SimpleString, saw %s", r.Type())
	}

	s, err := r.SimpleString()
	if err != nil {
		t.Fatal(err)
	}
	if s != expected {
		t.Fatalf("Expected simple string '%s', saw '%s'", expected, s)
	}
}

func TestError(t *testing.T) {
	r := New(bytes.NewBufferString("-i'm an error\r\n"))
	expected := errors.New("i'm an error")

	if r.Type() != ErrorType {
		t.Fatalf("Expected type Error, saw %s", r.Type())
	}

	err := r.Error()
	if err.Error() != expected.Error() {
		t.Fatalf("Expected error '%s', saw '%s'", expected, err)
	}
}

func TestInteger(t *testing.T) {
	r := New(bytes.NewBufferString(":42"))
	var expected int64 = 42

	if r.Type() != IntegerType {
		t.Fatalf("Expected type Integer, saw %s", r.Type())
	}

	i, err := r.Int()
	if err != nil {
		t.Fatal(err)
	}
	if i != expected {
		t.Fatalf("Expected int '%d', saw '%d'", expected, i)
	}
}

func TestBulkString(t *testing.T) {
	r := New(bytes.NewBufferString("$17\r\ni'm a bulk string\r\n"))
	expected := bytes.NewBufferString("i'm a bulk string")

	if r.Type() != BulkStringType {
		t.Fatalf("Expected type BulkString, saw %s", r.Type())
	}

	b, err := r.BulkString()
	if err != nil {
		t.Fatal(err)
	}
	bb, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(bb) != expected.String() {
		t.Fatalf("Expected bulk string '%d', saw '%d'", expected.String(), string(bb))
	}
}

func TestArraySimple(t *testing.T) {
	r := New(bytes.NewBufferString("*1\r\n+foo\r\n"))

	if r.Type() != ArrayType {
		t.Fatalf("Expected type Array, saw %s", r.Type())
	}

	a, err := r.Array()
	if err != nil {
		t.Fatal(err)
	}

	if a.Len() != 1 {
		t.Fatalf("Expected 1 element, saw %d", a.Len())
	}

	r1 := a.Next()
	expectedSimpleString := "foo"

	if r1.Type() != SimpleStringType {
		t.Fatalf("Expected type %s, saw %s", SimpleStringType, r1.Type())
	}

	s, err := r1.SimpleString()
	if err != nil {
		t.Fatal(err)
	}
	if s != expectedSimpleString {
		t.Fatalf("Expected simple string '%s', saw '%s'", expectedSimpleString, s)
	}

}

func TestArrayBulk(t *testing.T) {
	r := New(bytes.NewBufferString("*1\r\n$17\r\ni'm a bulk string\r\n"))

	if r.Type() != ArrayType {
		t.Fatalf("Expected type Array, saw %s", r.Type())
	}

	a, err := r.Array()
	if err != nil {
		t.Fatal(err)
	}

	if a.Len() != 1 {
		t.Fatalf("Expected 1 elements, saw %d", a.Len())
	}

	r1 := a.Next()

	expectedBulkString := "i'm a bulk string"
	if r1.Type() != BulkStringType {
		t.Fatalf("Expected type BulkString, saw %s", r1.Type())
	}

	b, err := r1.BulkString()
	if err != nil {
		t.Fatal(err)
	}

	bb, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(bb) != expectedBulkString {
		t.Fatalf("Expected bulk string '%s', saw '%s'", expectedBulkString, string(bb))
	}

}

func TestArrayMany(t *testing.T) {
	r := New(bytes.NewBufferString("*2\r\n+simple\r\n$17\r\ni'm a bulk string\r\n"))

	if r.Type() != ArrayType {
		t.Fatalf("Expected type Array, saw %s", r.Type())
	}

	a, err := r.Array()
	if err != nil {
		t.Fatal(err)
	}

	if a.Len() != 2 {
		t.Fatalf("Expected 2 elements, saw %d", a.Len())
	}

	r1 := a.Next()

	// first elem
	expectedSimpleString := "simple"

	if r1.Type() != SimpleStringType {
		t.Fatalf("Expected type SimpleString, saw %s", r1.Type())
	}

	s, err := r1.SimpleString()
	if err != nil {
		t.Fatal(err)
	}
	if s != expectedSimpleString {
		t.Fatalf("Expected simple string '%s', saw '%s'", expectedSimpleString, s)
	}

	// second elem
	r2 := a.Next()
	expectedBulkString := "i'm a bulk string"
	if r2.Type() != BulkStringType {
		t.Fatalf("Expected type BulkString, saw %s", r2.Type())
	}

	b, err := r2.BulkString()
	if err != nil {
		t.Fatal(err)
	}

	bb, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(bb) != expectedBulkString {
		t.Fatalf("Expected bulk string '%s', saw '%s'", expectedBulkString, string(bb))
	}

}

func TestArrayManyCached(t *testing.T) {
	r := New(bytes.NewBufferString("*2\r\n$3\r\nfoo\r\n$17\r\ni'm a bulk string\r\n"))

	if r.Type() != ArrayType {
		t.Fatalf("Expected type Array, saw %s", r.Type())
	}

	a, err := r.Array()
	if err != nil {
		t.Fatal(err)
	}

	if a.Len() != 2 {
		t.Fatalf("Expected 2 elements, saw %d", a.Len())
	}

	r1 := a.Next()
	r2 := a.Next()

	// second elem, caching first
	expectedBulkString := "i'm a bulk string"
	if r2.Type() != BulkStringType {
		t.Fatalf("Expected type BulkString, saw %s", r2.Type())
	}

	b, err := r2.BulkString()
	if err != nil {
		t.Fatal(err)
	}

	bb, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(bb) != expectedBulkString {
		t.Fatalf("Expected bulk string '%s', saw '%s'", expectedBulkString, string(bb))
	}

	// first elem
	expectedCachedString := "foo"

	if r1.Type() != BulkStringType {
		t.Fatalf("Expected type BulkString, saw %s", r1.Type())
	}

	b, err = r1.BulkString()
	if err != nil {
		t.Fatal(err)
	}

	bb, err = ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(bb) != expectedCachedString {
		t.Fatalf("Expected bulk string '%s', saw %s", expectedCachedString, strconv.Quote(string(bb)))
	}
}
