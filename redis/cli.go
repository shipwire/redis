// Command redis is a CLI for redis.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"bufio"

	"bitbucket.org/shipwire/redis"
	"bitbucket.org/shipwire/redis/resp"
)

func main() {
	network := flag.String("network", "tcp", "The type of network to connect on")
	host := flag.String("host", "127.0.0.1:6379", "The location to connect to")
	timeout := flag.Int("timeout", 30, "The timeout to connect, in seconds. Pass 0 for no timeout.")

	flag.Parse()

	var conn *redis.Conn
	var err error
	if *timeout == 0 {
		conn, err = redis.Dial(*network, *host)
	} else {
		conn, err = redis.DialTimeout(*network, *host, time.Duration(*timeout)*time.Second)
	}

	killChan := make(chan os.Signal, 1)
	signal.Notify(killChan, os.Kill, os.Interrupt)
	go func() {
		<-killChan
		conn.Close()
		fmt.Println()
		os.Exit(0)
	}()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Connection established")

	for {
		scanCommands(conn)
	}

}

func scanCommands(conn *redis.Conn) {
	r := bufio.NewReader(os.Stdin)
	for {
		io.WriteString(os.Stdout, "redis> ")
		line, isPrefix, err := r.ReadLine()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		for isPrefix {
			var more []byte
			more, isPrefix, err = r.ReadLine()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			line = append(line, more...)
		}
		reply := scanCommand(conn, string(line))
		if reply != nil {
			fmt.Println(reply)
		}
	}
}

func scanCommand(conn *redis.Conn, cmdText string) *resp.RESP {
	cmdName, args := extractCommand(cmdText)
	switch strings.ToLower(cmdName) {
	case "exit", "quit":
		os.Exit(0)
	case "":
	case "subscribe", "psubscribe":
		subscribe(conn, args)
		return nil
	default:
		cmd, _ := conn.Command(cmdName, len(args))
		for _, arg := range args {
			cmd.WriteArgumentString(arg)
		}
		err := cmd.Close()
		if err != nil {
			fmt.Println(err)
		}
		return conn.Resp()
	}
	return nil
}

func subscribe(conn *redis.Conn, channels []string) {
	messages := make(chan *resp.RESP, 10)
	done := make(chan struct{}, 0)

	err := conn.Subscribe(strings.Join(channels, " "), messages, done)
	if err != nil {
		fmt.Println(err)
		return
	}

	for {
		r := <-messages
		fmt.Println(r)
	}
}

func extractCommand(cmdText string) (string, []string) {
	split := strings.SplitN(cmdText, " ", 2)
	switch len(split) {
	case 0:
		return "", nil
	case 1:
		return split[0], nil
	}

	return split[0], extractArgs(split[1])
}

func extractArgs(argText string) []string {

	a := newLexer(argText)
	return a.getargs()
}
