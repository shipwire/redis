package main

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type argLexer struct {
	argText           string
	args              chan string
	width, start, pos int
	err               error
}

func (l *argLexer) getargs() []string {
	args := make([]string, 0)
	for arg := range l.args {
		args = append(args, arg)
	}
	return args
}

type stateFn func(a *argLexer) stateFn

func newLexer(argText string) *argLexer {
	a := &argLexer{argText: argText, args: make(chan string, 1)}
	go a.run()

	return a
}

func (a *argLexer) run() {
	for state := lexArg; state != nil; {
		state = state(a)
	}
	close(a.args)
}

func lexArg(l *argLexer) stateFn {
	l.skipSpace()
	if l.peek() == eof {
		return nil
	}
	if l.accept(`"`) {
		return lexQuotedArg(l)
	}
	l.acceptRunNonSpace()
	l.emit()
	return lexArg
}

func lexQuotedArg(a *argLexer) stateFn {
	a.acceptNonQuote()
	if !a.accept(`"`) {
		a.err = errors.New("Insanity. There should be a quote here")
		return nil
	}
	if err := a.emitQuoted(); err != nil {
		a.err = err
		return nil
	}
	if a.next() == eof {
		return nil
	}
	return lexArg
}

// emit gets the current token., sends it on the token. channel
// and prepares for lexing the next token.
func (l *argLexer) emit() {
	i := l.argText[l.start:l.pos]
	l.args <- i
	l.start = l.pos
}

func (l *argLexer) emitQuoted() error {
	i, err := strconv.Unquote(l.argText[l.start:l.pos])
	if err != nil {
		return err
	}
	l.args <- i
	l.start = l.pos
	return nil
}

// nextItem returns the next token. from the input.
func (l *argLexer) nextArg() string {
	return <-l.args
}

// peek returns but does not consume the next rune in the input.
func (l *argLexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Can only be called once per call of next.
func (l *argLexer) backup() {
	l.pos -= l.width
}

func (l *argLexer) accept(valid string) bool {
	if strings.IndexRune(valid, l.next()) >= 0 {
		return true
	}
	l.backup()
	return false
}

func (l *argLexer) acceptRunNonSpace() {
	for r := l.next(); !unicode.IsSpace(r) && r != eof; r = l.next() {
	}
	l.backup()
}

func (l *argLexer) acceptNonQuote() {
	preceedingSlashes := 0
	for r := l.next(); (r != '"' || preceedingSlashes%2 == 1) && r != eof; r = l.next() {
		if r == '\\' {
			preceedingSlashes += 1
		} else {
			preceedingSlashes = 0
		}
	}
	l.backup()
}

// acceptRun consumes a run of runes from the valid set.
func (l *argLexer) acceptRun(valid string) {
	for strings.IndexRune(valid, l.next()) >= 0 {
	}
	l.backup()
}

func (l *argLexer) next() rune {
	if int(l.pos) >= len(l.argText) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.argText[l.pos:])
	l.width = w
	l.pos += l.width
	return r
}

const eof = -1

func (l *argLexer) skipSpace() {
	r := l.next()
	for unicode.IsSpace(r) {
		r = l.next()
	}
	l.backup()
	l.ignore()
}

// ignore skips over the pending input before this point.
func (l *argLexer) ignore() {
	l.start = l.pos
}
