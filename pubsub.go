package redis

import "bitbucket.org/shipwire/redis/resp"

type Publisher struct {
	pool *Pool
}

type Subscription struct {
	conn        *Conn
	subscribers map[string][]chan<- *resp.RESP
	done        chan struct{}
}

func (p *Pool) Subscribe(channel string, ch chan<- *resp.RESP) (*Subscription, error) {
	if p.subscriptions == nil {
		conn, err := p.Conn()
		if err != nil {
			return nil, err
		}
		p.subscriptions = &Subscription{
			subscribers: make(map[string][]chan<- *resp.RESP),
			done:        make(chan struct{}),
			conn:        conn,
		}
	}

	err := p.subscriptions.conn.RawCmd("SUBSCRIBE", channel)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *Subscription) watch() {
	go func() {
		for {
			select {
			case <-s.done:
				return
			default:
				s.read()
			}
		}
	}()
}

func (s *Subscription) read() {
	switch s.conn.Resp().Type() {
	case resp.Unknown:
	case resp.Invalid:
		panic("redis pubsub: invalid message received")
	case resp.Array:
		s.receive(s.conn.Resp().Array())
	default:
		panic("redis pubsub: wrong type message received")
	}
}

func (s *Subscription) receive(r *resp.RESPArray) {
	first := r.Next()

	str, err := first.SimpleString()
	if err != nil {
		panic(err)
	}

	switch str {
	case "subscribe", "unsubscribe":
		return
	}

	channel, err := r.Next().SimpleString()
	if err != nil {
		panic(err)
	}

	message := r.Next()

	subscribers := s.subscribers[channel]
	for _, ch := range subscribers {
		ch <- message
	}

}
