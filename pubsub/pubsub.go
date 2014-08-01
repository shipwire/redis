package pubsub

import "bitbucket.org/shipwire/redis"

type Publisher struct {
	pool *redis.Pool
}

type Subscription struct {
	conn *redis.Conn
}

func Subscribe(channel string, conn *redis.Conn) *Subscription {
}

func (s *Subscription) Next() *RESP {
}

func (s *Subscription) WaitForNext() *RESP {
}
