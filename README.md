# `nats-pool`

[![Apache License](https://img.shields.io/github/license/octu0/nats-pool)](https://github.com/octu0/nats-pool/blob/master/LICENSE)
[![GoDoc](https://godoc.org/github.com/octu0/nats-pool?status.svg)](https://godoc.org/github.com/octu0/nats-pool)
[![Go Report Card](https://goreportcard.com/badge/github.com/octu0/nats-pool)](https://goreportcard.com/report/github.com/octu0/nats-pool)
[![Releases](https://img.shields.io/github/v/release/octu0/nats-pool)](https://github.com/octu0/nats-pool)

`nats-pool` connection pooling for [nats.go](https://github.com/nats-io/nats.go)

## Installation

```bash
go get github.com/octu0/nats-pool
```

## Example

```go
import (
	"github.com/nats-io/nats.go"
	"github.com/octu0/nats-pool"
)

var (
	// 100 connections pool
	connPool = pool.New(100, "nats://localhost:4222",
		nats.NoEcho(),
		nats.Name("client/1.0"),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			...
		}),
	)
)

func main() {
	nc, err := connPool.Get()
	if err != nil {
		panic(err)
	}
	// return buffer to pool
	defer connPool.Put(nc)
	:
	nc.Subscribe("subject.a.b.c", func(msg *nats.Msg) {
		...
	})
	nc.Publish("foo.bar", []byte("hello world"))
}
```

## Benchmark

Here are the benchmark results for a simple case with multiple PubSub.

```
$ go test -bench=. -benchmem
goos: darwin
goarch: amd64
pkg: github.com/octu0/nats-pool
BenchmarkSimpleConnPubSub/NoPool-8         	    5000	    261422 ns/op	  124696 B/op	     177 allocs/op
BenchmarkSimpleConnPubSub/UsePool-8        	   35050	     29524 ns/op	    4656 B/op	      50 allocs/op
PASS
ok  	github.com/octu0/nats-pool	3.829s
```

## License

Apache 2.0, see LICENSE file for details.
