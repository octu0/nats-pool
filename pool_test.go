package pool

import (
	"context"
	"fmt"
	natsd "github.com/nats-io/nats-server/server"
	nats "github.com/nats-io/nats.go"
	"strconv"
	"sync"
	"testing"
	"time"
)

func testStartNatsd(port int) (*natsd.Server, error) {
	opts := &natsd.Options{
		Host:            "127.0.0.1",
		Port:            port,
		ClientAdvertise: "127.0.0.1",
		HTTPPort:        -1,
		Cluster:         natsd.ClusterOpts{Port: -1},
		NoLog:           true,
		NoSigs:          true,
		Debug:           false,
		Trace:           false,
		MaxPayload:      512,
		PingInterval:    1 * time.Second,
		MaxPingsOut:     10,
		WriteDeadline:   2 * time.Second,
	}
	ns, err := natsd.NewServer(opts)
	if err != nil {
		return nil, err
	}
	go ns.Start()

	if ns.ReadyForConnections(3*time.Second) != true {
		return nil, fmt.Errorf("natsd startup failure")
	}
	return ns, nil
}

func BenchmarkSimpleConnPubSub(b *testing.B) {
	test := func(wg *sync.WaitGroup, nc *nats.Conn, id string, tb *testing.B, end func(nc *nats.Conn)) {
		defer wg.Done()
		defer end(nc)

		done := make(chan struct{})
		subj := "hello.workd" + id
		sub, err := nc.Subscribe(subj, func(msg *nats.Msg) {
			if string(msg.Data) != "foobar" {
				tb.Errorf("msg.Data not != foobar : %s", string(msg.Data))
			}
			done <- struct{}{}
		})
		if err != nil {
			tb.Errorf(err.Error())
			return
		}

		go nc.Publish(subj, []byte("foobar"))
		<-done
		sub.Unsubscribe()
	}
	b.Run("NoPool", func(tb *testing.B) {
		ns, err := testStartNatsd(-1)
		if err != nil {
			panic(err)
		}

		wg := new(sync.WaitGroup)
		url := fmt.Sprintf("nats://%s", ns.Addr().String())
		for i := 0; i < tb.N; i += 1 {
			nc, err := nats.Connect(url)
			if err != nil {
				tb.Errorf(err.Error())
			}
			wg.Add(1)
			go test(wg, nc, strconv.Itoa(i), tb, func(_nc *nats.Conn) {
				_nc.Close()
			})
		}
		wg.Wait()
	})
	b.Run("UsePool", func(tb *testing.B) {
		ns, err := testStartNatsd(-1)
		if err != nil {
			panic(err)
		}

		wg := new(sync.WaitGroup)
		url := fmt.Sprintf("nats://%s", ns.Addr().String())
		p := New(100, url)
		for i := 0; i < tb.N; i += 1 {
			nc, err := p.Get()
			if err != nil {
				tb.Errorf(err.Error())
			}
			wg.Add(1)
			go test(wg, nc, strconv.Itoa(i), tb, func(_nc *nats.Conn) {
				p.Put(_nc)
			})
		}
		wg.Wait()
	})
}

func TestNew(t *testing.T) {
	ns, err := testStartNatsd(-1)
	if err != nil {
		panic(err)
	}
	defer ns.Shutdown()

	url := fmt.Sprintf("nats://%s", ns.Addr().String())
	t.Run("default", func(tt *testing.T) {
		p := New(10, url)
		nc, err := p.Get()
		if err != nil {
			tt.Errorf("no error to Get(): %s", err.Error())
		}
		if nc.Opts.Name != "" {
			tt.Errorf("using default option: %s", nc.Opts.Name)
		}
		if p.Len() != 0 {
			tt.Errorf("initial len = 0")
		}
		if p.Cap() != 10 {
			tt.Errorf("initial cap = 10")
		}
	})
	t.Run("withOption", func(tt *testing.T) {
		p := New(10, url,
			nats.Name("hello world"),
			nats.NoEcho(),
		)
		nc, err := p.Get()
		if err != nil {
			tt.Errorf("no error to Get() w/ options: %s", err.Error())
		}
		if nc.Opts.Name != "hello world" {
			tt.Errorf("option w/ name: %s", nc.Opts.Name)
		}
		if nc.Opts.NoEcho != true {
			tt.Errorf("option w/ NoEcho: %v", nc.Opts.NoEcho)
		}
		if p.Len() != 0 {
			tt.Errorf("initial len = 0")
		}
		if p.Cap() != 10 {
			tt.Errorf("initial cap = 10")
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
		p := New(10, url)
		nc1, err1 := p.Get()
		if err1 != nil {
			tt.Errorf(err1.Error())
		}
		if nc1.IsConnected() != true {
			tt.Errorf("connected only")
		}
		ok1, err2 := p.Put(nc1)
		if ok1 != true {
			tt.Errorf("put ok / free cap")
		}
		if err2 != nil {
			tt.Errorf(err2.Error())
		}
		if p.Len() != 1 {
			tt.Errorf("p.Len() == 1: %d", p.Len())
		}
	})
	t.Run("put/maxcap", func(tt *testing.T) {
		p := New(10, url)
		n := make([]*nats.Conn, 10)
		for i := 0; i < 10; i += 1 {
			nc, err := p.Get()
			if err != nil {
				tt.Errorf(err.Error())
			}
			n[i] = nc
		}
		for _, nc := range n {
			ok, err := p.Put(nc)
			if ok != true {
				tt.Errorf("put ok / free cap")
			}
			if err != nil {
				tt.Errorf(err.Error())
			}
		}
		if p.Len() != 10 {
			tt.Errorf("len == 10")
		}

		nc1, err1 := p.connect()
		if err1 != nil {
			tt.Errorf(err1.Error())
		}
		ok, err2 := p.Put(nc1)
		if ok {
			tt.Errorf("discard / max cap")
		}
		if err2 != nil {
			tt.Errorf(err2.Error())
		}
		if nc1.IsConnected() {
			tt.Errorf("closed")
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
		p := New(10, url)
		n := make([]*nats.Conn, 10)
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
	t.Run("getput/conn10", func(tt *testing.T) {
		ns, err := testStartNatsd(-1)
		if err != nil {
			panic(err)
		}
		defer ns.Shutdown()

		url := fmt.Sprintf("nats://%s", ns.Addr().String())
		p := New(10, url)
		n := make([]*nats.Conn, 10)
		for i := 0; i < 10; i += 1 {
			nc, err := p.Get()
			if err != nil {
				tt.Errorf(err.Error())
			}
			n[i] = nc
		}

		for _, nc := range n {
			p.Put(nc)
		}

		if ns.NumClients() != 10 {
			tt.Errorf("server client 10: %d", ns.NumClients())
		}

		nc, err := p.Get()
		if err != nil {
			tt.Errorf(err.Error())
		}

		if ns.NumClients() != 10 {
			tt.Errorf("server client 10: %d", ns.NumClients())
		}

		p.Put(nc)

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
	p := New(10, url)
	nc, err := p.Get()
	if err != nil {
		t.Errorf(err.Error())
	}
	nc.Close()
	ok, err := p.Put(nc)
	if err != nil {
		t.Errorf(err.Error())
	}
	if ok != true {
		t.Errorf("free cap")
	}
	nc1, err1 := p.Get()
	if err1 != nil {
		t.Errorf(err1.Error())
	}
	if nc1.IsConnected() != true {
		t.Errorf("new conn is connected")
	}
}

func TestLeakSubs(t *testing.T) {
	ns, err := testStartNatsd(-1)
	if err != nil {
		panic(err)
	}
	defer ns.Shutdown()

	url := fmt.Sprintf("nats://%s", ns.Addr().String())
	p := New(10, url)
	nc, err := p.Get()
	if err != nil {
		t.Errorf(err.Error())
	}
	sub, err := nc.Subscribe("foo.bar", func(msg *nats.Msg) {
		if "hello world" != string(msg.Data) {
			t.Errorf("data == 'hello world': %v", msg.Data)
		}
	})
	if err != nil {
		t.Errorf(err.Error())
	}
	if nc.NumSubscriptions() != 1 {
		t.Errorf("sub 1")
	}

	nc.Publish("foo.bar", []byte("hello world"))

	p.Put(nc)

	if nc.NumSubscriptions() != 1 {
		t.Errorf("leaked sub")
	}
	if sub.IsValid() != true {
		t.Errorf("released but keep connection")
	}
	sub.Unsubscribe()
	if nc.NumSubscriptions() != 0 {
		t.Errorf("unscribed")
	}
}

func TestDisconnectAll(t *testing.T) {
	t.Run("newclient", func(tt *testing.T) {
		ns, err := testStartNatsd(-1)
		if err != nil {
			panic(err)
		}
		defer ns.Shutdown()

		url := fmt.Sprintf("nats://%s", ns.Addr().String())
		p := New(10, url)

		ncs := make([]*nats.Conn, 10)
		for i := 0; i < 10; i += 1 {
			nc, err := p.Get()
			if err != nil {
				tt.Errorf(err.Error())
			}
			ncs[i] = nc
		}

		lastClientId := uint64(0)
		for _, nc := range ncs {
			id, err := nc.GetClientID()
			if err != nil {
				tt.Errorf(err.Error())
			}
			lastClientId = id
			p.Put(nc)
		}

		nc1, err := p.Get()
		if err != nil {
			t.Errorf(err.Error())
		}
		id1, err := nc1.GetClientID()
		if err != nil {
			t.Errorf(err.Error())
		}
		if lastClientId < id1 {
			tt.Errorf("use exists conn")
		}
		p.Put(nc1)

		p.DisconnectAll()

		nc2, err := p.Get()
		if err != nil {
			tt.Errorf(err.Error())
		}
		id2, err := nc2.GetClientID()
		if err != nil {
			tt.Errorf(err.Error())
		}
		p.Put(nc2)

		if id2 <= lastClientId {
			tt.Errorf("new client")
		}
	})
	t.Run("concurrency", func(tt *testing.T) {
		// all no error

		ns, err := testStartNatsd(-1)
		if err != nil {
			panic(err)
		}
		defer ns.Shutdown()

		url := fmt.Sprintf("nats://%s", ns.Addr().String())
		p := New(100, url)
		boot := new(sync.WaitGroup)
		ctx, cancel := context.WithCancel(context.Background())
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

		time.Sleep(time.Second) // warmimng-up workers
		for i := 0; i < 100; i += 1 {
			p.DisconnectAll()
			time.Sleep(50 * time.Millisecond) // save high load for ci
		}
		cancel()
	})
}
