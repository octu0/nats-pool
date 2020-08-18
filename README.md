# `nats-pool`

[![Apache License](https://img.shields.io/github/license/octu0/nats-pool)](https://github.com/octu0/nats-pool/blob/master/LICENSE)
[![GoDoc](https://godoc.org/github.com/octu0/nats-pool?status.svg)](https://godoc.org/github.com/octu0/nats-pool)
[![Go Report Card](https://goreportcard.com/badge/github.com/octu0/nats-pool)](https://goreportcard.com/report/github.com/octu0/nats-pool)
[![Releases](https://img.shields.io/github/v/release/octu0/nats-pool)](https://github.com/octu0/chanque/nats-pool)

`nats-pool` connection pooling for [nats.io](http://nats.io/)

## Installation

```bash
go get github.com/octu0/nats-pool
```

## Example

```go

import(
  "github.com/nats-io/nats.go"
  "github.com/octu0/nats-pool"
)

var(
  // 100 connection pool
  connPool = pool.New(100, "nats://localhost:4222",
    nats.NoEcho(),
    nats.Name("client/1.0"),
    nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error){
      ...
    }),
  })
)

func main() {
  conn, err := connPool.Get()
  if err != nil {
    panic(err)
  }
  :
  conn.Subscribe("subject.a.b.c", func(msg *nats.Msg) {
    ...
  })
  :
  // return buffer to pool
  connPool.Put(conn)
}
```

## License

Apache 2.0, see LICENSE file for details.
