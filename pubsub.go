package redis

import (
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/shipwire/redis/resp"
)

type sub struct {
	conn              *Conn
	done              <-chan struct{}
	subscriptions     map[string]*subscription
	subscriptionsLock *sync.RWMutex
}

type subscription struct {
	subscribers map[chan<- *resp.RESP]struct{}
	channel     string
	done        <-chan struct{}
}

// Subscribe registers a channel of RESP values to a redis pubsub channel.
func (p *Pool) Subscribe(channel string, ch chan<- *resp.RESP) error {
	sub, err := p.getSub()
	if err != nil {
		return err
	}

	sub.subscriptionsLock.Lock()
	defer sub.subscriptionsLock.Unlock()

	s, ok := p.sub.subscriptions[channel]
	if !ok {
		s = &subscription{
			make(map[chan<- *resp.RESP]struct{}),
			channel,
			make(chan struct{}),
		}
	}

	s.subscribers[ch] = struct{}{}
	sub.subscriptions[channel] = s

	return sub.conn.RawCmd("SUBSCRIBE", channel)
}

// Subscribe listens on c for published messages on the a channel. This method will either return
// an error right away or block while sending received messages on the messges channel until it
// receives a signal on the done channel. This connection should not be reused for another purpose.
func (c *Conn) Subscribe(channel string, messages chan<- *resp.RESP) error {
	c.whence = nil
	err := c.RawCmd("SUBSCRIBE", channel)
	if err != nil {
		return err
	}

	if c.sub == nil {
		c.sub = &sub{
			conn:              c,
			subscriptions:     map[string]*subscription{},
			subscriptionsLock: &sync.RWMutex{},
		}
		c.sub.watch()
	}
	c.sub.subscriptionsLock.Lock()
	defer c.sub.subscriptionsLock.Unlock()

	s, ok := c.sub.subscriptions[channel]
	if !ok {
		s = &subscription{
			make(map[chan<- *resp.RESP]struct{}),
			channel,
			make(chan struct{}),
		}
	}

	s.subscribers[messages] = struct{}{}
	c.sub.subscriptions[channel] = s

	return nil
}

// Unsubscribe unregisters a channel from receiving messages on a redis channel.
// After unsubscribe returns, it is guaranteed that ch will receive no more messages.
func (c *Conn) Unsubscribe(channel string, ch chan<- *resp.RESP) {
	if c.sub == nil {
		return
	}

	c.sub.subscriptionsLock.Lock()
	defer c.sub.subscriptionsLock.Unlock()

	s, ok := c.sub.subscriptions[channel]
	if ok {
		delete(s.subscribers, ch)
	}
}

// Unsubscribe unregisters a channel of RESP values from a redis pubsub channel. If
// and after Unsubscribe returns with no error, it is guaranteed that ch will receive
// no more messages.
func (p *Pool) Unsubscribe(channel string, ch chan<- *resp.RESP) error {
	sub, err := p.getSub()
	if err != nil {
		return err
	}

	sub.subscriptionsLock.Lock()
	defer sub.subscriptionsLock.Unlock()

	s, ok := p.sub.subscriptions[channel]
	if ok {
		delete(s.subscribers, ch)
	}

	return nil
}

func (p *Pool) getSub() (*sub, error) {
	if p.sub == nil {
		conn, err := p.Conn()
		if err != nil {
			return nil, err
		}
		p.sub = &sub{
			conn:              conn,
			done:              make(chan struct{}),
			subscriptions:     make(map[string]*subscription),
			subscriptionsLock: &sync.RWMutex{},
		}
	}
	return p.sub, nil
}

func (p *sub) watch() {
	go func() {
		for {
			select {
			case <-p.done:
				return
			default:
				p.read()
			}
		}
	}()
}

func (p *sub) read() {
	switch p.conn.Resp().Type() {
	case resp.UnknownType: // do nothing, we haven't read yet
	case resp.ErrorType:
		p.conn.Resp().Error()
	case resp.InvalidType:
		panic("redis pubsub: invalid message received")
	case resp.ArrayType:
		arr, _ := p.conn.Resp().Array()
		p.receive(arr)
	default:
		r := p.conn.Resp()
		panic(fmt.Sprint("redis pubsub: wrong type message received. Got:", r.Type(), r))
	}
}

func (p *sub) receive(r *resp.Array) {
	first := r.Next()

	rd, err := first.BulkString()
	if err != nil {
		fmt.Println("Reading message type didn't work:", err)
		return
	}

	b := make([]byte, 11)
	n, _ := rd.Read(b)
	b = b[:n]

	chelem := r.Next()
	chStr, err := chelem.BulkString()
	if err != nil {
		fmt.Println("Couldn't get channel data:", err)
		return
	}

	switch string(b) {
	case "subscribe", "unsubscribe":
		fmt.Sprint(r.Next())
		return
	}

	chbytes, err := ioutil.ReadAll(chStr)
	if err != nil {
		fmt.Println("Couldn't get channel data:", err)
		return
	}
	channel := string(chbytes)

	message := r.Next()

	p.subscriptionsLock.RLock()
	defer p.subscriptionsLock.RUnlock()

	subscription := p.subscriptions[channel]
	for ch := range subscription.subscribers {
		ch <- message
	}

}
