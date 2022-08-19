package pool

import (
	"errors"
	"strings"
	"sync"

	"github.com/nats-io/nats.go"
)

// ConnPool implements pool of *nats.EncodedConn of a bounded channel
type ConnPool struct {
	mutex   *sync.RWMutex
	pool    *sync.Pool
	url     string
	options []nats.Option
}

func NewPool(servers []string, user, pass string) *ConnPool {
	return New(strings.Join(servers, ","), nats.UserInfo(user, pass))
}

// New create new ConnPool bounded to the given poolSize,
// with specify the URL string to connect to natsd on url.
// option is used for nats#Connect when creating pool connections
func New(url string, options ...nats.Option) *ConnPool {
	return &ConnPool{
		mutex: new(sync.RWMutex),
		pool: &sync.Pool{
			New: func() any {
				connect, err := nats.Connect(url, options...)
				if err != nil {
					return nil
				}

				ec, err := nats.NewEncodedConn(connect, nats.GOB_ENCODER)
				if err != nil {
					return nil
				}

				return ec
			},
		},
		url:     url,
		options: options,
	}
}

var errGetPoolConnection = errors.New("get conn from pool err")

// Get returns *nats.EncodedConn, if connection is available,
func (p *ConnPool) Get() (*nats.EncodedConn, error) {
	conn, ok := p.pool.Get().(*nats.EncodedConn)
	if !ok {
		return nil, errGetPoolConnection
	}

	return conn, nil
}

// Put puts *nats.EncodedConn back into the pool.
func (p *ConnPool) Put(ec *nats.EncodedConn) error {
	if ec.Conn.IsConnected() {
		if err := ec.Flush(); err != nil {
			return err
		}
	}

	p.pool.Put(ec)
	return nil
}

func (p *ConnPool) Publish(topic string, v interface{}) (err error) {
	conn, err := p.Get()
	if err != nil {
		return err
	}

	defer p.Put(conn)
	if err = conn.Publish(topic, v); err != nil {
		return err
	}
	return nil
}

