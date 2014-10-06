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

var messages = make(chan *resp.RESP, 10)
var subscriptionMode = false

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
	cmds := readLines()
	for {
		io.WriteString(os.Stdout, "redis> ")
		select {
		case cmd := <-cmds:
			reply := scanCommand(conn, cmd)
			if reply != nil {
				fmt.Println("Reply:", reply)
			}
		case msg := <-messages:
			fmt.Println()
			fmt.Println("Published message:", msg)
		}
	}
}

func readLines() <-chan string {
	ch := make(chan string)
	go func() {
		r := bufio.NewReader(os.Stdin)
		for {
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
			ch <- string(line)
		}
	}()

	return ch
}

func scanCommand(conn *redis.Conn, cmdText string) *resp.RESP {
	cmdName, args := extractCommand(cmdText)
	switch strings.ToLower(cmdName) {
	case "exit", "quit":
		os.Exit(0)
	case "":
	case "subscribe", "psubscribe":
		return subscribe(conn, args)
	case "unsubscribe", "punsubscribe":
		return unsubscribe(conn, args)
	default:
		if subscriptionMode {
			fmt.Println("ERR only (P)SUBSCRIBE / (P)UNSUBSCRIBE / QUIT allowed in this context")
			return nil
		}

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

func subscribe(conn *redis.Conn, channels []string) *resp.RESP {
	subscriptionMode = true
	err := conn.Subscribe(strings.Join(channels, " "), messages)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return nil
}

func unsubscribe(conn *redis.Conn, channels []string) *resp.RESP {
	conn.Unsubscribe(strings.Join(channels, " "), messages)
	return nil
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
