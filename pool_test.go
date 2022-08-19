package pool

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func testStartNatsd(port int) (*server.Server, error) {
	opts := &server.Options{
		Host:            "127.0.0.1",
		Port:            port,
		ClientAdvertise: "127.0.0.1",
		HTTPPort:        -1,
		Cluster:         server.ClusterOpts{Port: -1},
		NoLog:           true,
		NoSigs:          true,
		Debug:           false,
		Trace:           false,
		MaxPayload:      512,
		PingInterval:    1 * time.Second,
		MaxPingsOut:     10,
		WriteDeadline:   2 * time.Second,
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}
	go ns.Start()

	if ns.ReadyForConnections(3*time.Second) != true {
		return nil, fmt.Errorf("natsd server startup failure")
	}
	return ns, nil
}

func TestNew(t *testing.T) {
	ns, err := testStartNatsd(-1)
	if err != nil {
		panic(err)
	}
	defer ns.Shutdown()

	url := fmt.Sprintf("nats://%s", ns.Addr().String())
	t.Run("default", func(tt *testing.T) {
		p := New(url)
		_, err := p.Get()
		if err != nil {
			tt.Errorf("no error to Get(): %s", err.Error())
		}
	})
	t.Run("withOption", func(tt *testing.T) {
		p := New(url,
			nats.Name("hello world"),
			nats.NoEcho(),
		)
		_, err := p.Get()
		if err != nil {
			tt.Errorf("no error to Get() w/ options: %s", err.Error())
		}
	})
}

func TestGetPut(t *testing.T) {
	ns, err := testStartNatsd(-1)
	if err != nil {
		panic(err)
	}
	defer ns.Shutdown()

	url := fmt.Sprintf("nats://%s", ns.Addr().String())
	t.Run("getput", func(tt *testing.T) {
		p := New(url)
		nc1, err := p.Get()
		if err != nil {
			tt.Errorf(err.Error())
		}

		if nc1.Conn.IsConnected() != true {
			tt.Errorf("connected only")
		}

		if err := p.Put(nc1); err != nil {
			tt.Errorf("put ok / free cap")
		}
	})
}

func TestPool(t *testing.T) {
	t.Run("get/conn10", func(tt *testing.T) {
		ns, err := testStartNatsd(-1)
		if err != nil {
			panic(err)
		}
		defer ns.Shutdown()

		url := fmt.Sprintf("nats://%s", ns.Addr().String())
		p := New(url)
		n := make([]*nats.EncodedConn, 10)
		for i := 0; i < 10; i += 1 {
			nc, err := p.Get()
			if err != nil {
				tt.Errorf(err.Error())
			}
			n[i] = nc
		}

		if ns.NumClients() != 10 {
			tt.Errorf("server client 10: %d", ns.NumClients())
		}
	})
}

func TestClosedConn(t *testing.T) {
	ns, err := testStartNatsd(-1)
	if err != nil {
		panic(err)
	}
	defer ns.Shutdown()

	url := fmt.Sprintf("nats://%s", ns.Addr().String())
	p := New(url)
	nc, err := p.Get()
	if err != nil {
		t.Errorf(err.Error())
	}

	nc.Close()
	if err := p.Put(nc); err != nil {
		t.Errorf(err.Error())
	}

	nc1, err1 := p.Get()
	if err1 != nil {
		t.Errorf(err1.Error())
	}
	if !nc1.Conn.IsConnected() {
		t.Errorf("new conn is connected")
	}
}

func TestDisconnectAll(t *testing.T) {
	t.Run("concurrency", func(tt *testing.T) {
		// all no error
		ns, err := testStartNatsd(-1)
		if err != nil {
			panic(err)
		}
		defer ns.Shutdown()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		url := fmt.Sprintf("nats://%s", ns.Addr().String())
		p := New(url)
		boot := new(sync.WaitGroup)
		for i := 0; i < 100; i += 1 {
			boot.Add(1)
			go func(c context.Context, cp *ConnPool, b *sync.WaitGroup) {
				b.Done()
				for {
					select {
					case <-c.Done():
						return
					default:
						nc, err := cp.Get()
						if err != nil {
							tt.Errorf(err.Error())
						}
						cp.Put(nc)
						time.Sleep(10 * time.Millisecond) // save high load for ci
					}
				}
			}(ctx, p, boot)
		}
		boot.Wait()
	})
}
