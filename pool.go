package pool

import (
	"github.com/nats-io/nats.go"
)

// ConnPool implements pool of *nats.Conn of a bounded channel
type ConnPool struct {
	pool    chan *nats.Conn
	url     string
	options []nats.Option
}

// create a new ConnPool bounded to the given poolSize,
// with specify the URL string to connect to natsd on url.
// option is used for nats#Connect when creating pool connections
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

// Get() returns *nats.Conn, if connection is available,
// or makes a new connection and returns a value if not.
// if *nats.Conn is not connected in pool, make new connection in the same way.
func (p *ConnPool) Get() (*nats.Conn, error) {
	var nc *nats.Conn
	var err error
	select {
	case nc = <-p.pool:
		// reuse exists pool
		if nc.IsConnected() != true {
			// Close to be sure
			nc.Close()

			// disconnected conn, create new *nats.Conn
			nc, err = p.connect()
		}
	default:
		// create *nats.Conn
		nc, err = p.connect()
	}
	return nc, err
}

// Put() puts *nats.Conn back into the pool.
// there is no need to do Close() ahead of time,
// ConnPool will automatically do a Close() if it cannot be returned to the pool.
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

// returns the number of items currently pooled
func (p *ConnPool) Len() int {
	return len(p.pool)
}

// returns the number of pool capacity
func (p *ConnPool) Cap() int {
	return cap(p.pool)
}
