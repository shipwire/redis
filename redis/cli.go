package main

import (
	"flag"
	"fmt"
	"io"
	"os"
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

	fmt.Println("Connection established")

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

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
		fmt.Println(reply)
	}
}

func scanCommand(conn *redis.Conn, cmdText string) *resp.RESP {
	cmdName, args := extractCommand(cmdText)
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
