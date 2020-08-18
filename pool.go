package pool

import (
	"github.com/nats-io/nats.go"
)

type ConnPool struct {
	pool    chan *nats.Conn
	url     string
	options []nats.Option
}

func New(poolSize int, url string, options ...nats.Option) *ConnPool {
	return &ConnPool{
		pool:    make(chan *nats.Conn, poolSize),
		url:     url,
		options: options,
	}
}

func (p *ConnPool) connect() (*nats.Conn, error) {
	return nats.Connect(p.url, p.options...)
}

func (p *ConnPool) Get() (*nats.Conn, error) {
	var nc *nats.Conn
	var err error
	select {
	case nc = <-p.pool:
		// reuse exists pool
		if nc.IsConnected() != true {
			// disconnected conn to create new *nats.Conn
			nc, err = p.connect()
		}
	default:
		// create *nats.Conn
		nc, err = p.connect()
	}
	return nc, err
}

func (p *ConnPool) Put(nc *nats.Conn) (bool, error) {
	var err error
	if nc.IsConnected() {
		err = nc.Flush()
	}

	select {
	case p.pool <- nc:
		// free capacity
		return true, err
	default:
		// full capacity, discard & disconnect
		nc.Close()
		return false, err
	}
}

func (p *ConnPool) Len() int {
	return len(p.pool)
}

func (p *ConnPool) Cap() int {
	return cap(p.pool)
}
