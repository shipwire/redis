package redis

import (
	"sync"
	"time"

	"bitbucket.org/shipwire/redis/resp"
)

// Pool maintains a collection of idle Redis connections.
type Pool struct {
	pool *sync.Pool
	idle int

	Network, Server string

	// Password indicates that new connections should initially authenticate
	// with the given password. Password is ignored if empty.
	Password string

	// MaxIdle indicates how many idle connections should be kept. Zero indicates
	// all idle connections should be preserved.
	MaxIdle     int
	ConnTimeout time.Duration

	sub *sub
}

// NewPool intializes a connection pool with default settings.
func NewPool() *Pool {
	return &Pool{
		pool: &sync.Pool{},
		idle: 0,
	}
}

func (p *Pool) put(c *Conn) error {
	if p.MaxIdle != 0 && p.idle >= p.MaxIdle {
		return c.Conn.Close()
	}
	p.pool.Put(c)
	p.idle += 1
	return nil
}

// Conn attempts to get or create a connection, depending on if there
// are any idle connections.
func (p *Pool) Conn() (*Conn, error) {
	client, ok := p.pool.Get().(*Conn)
	if client == nil || !ok {
		return p.newConn()
	}
	p.idle -= 1
	return client, nil
}

func (p *Pool) newConn() (c *Conn, err error) {
	if p.ConnTimeout != 0 {
		c, err = DialTimeout(p.Network, p.Server, p.ConnTimeout)
	} else {
		c, err = Dial(p.Network, p.Server)
	}

	if err != nil {
		return nil, err
	}

	if p.Password != "" {
		c.RawCmd("AUTH", p.Password)
		if r := c.Resp(); r.Type() == resp.Error {
			return nil, r.Error()
		}
	}

	c.whence = p
	return
}
