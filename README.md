# fastws

[![Build Status](https://travis-ci.com/dgrr/fastws.svg?branch=master)](https://travis-ci.com/dgrr/fastws)
[![codecov](https://codecov.io/gh/dgrr/fastws/branch/master/graph/badge.svg)](https://codecov.io/gh/dgrr/fastws)

Websocket library for [fasthttp](https://github.com/valyala/fasthttp). And now for net/http too.

Checkout [examples](https://github.com/dgrr/fastws/blob/master/examples) to inspire yourself.

# Why another websocket package?

Other websocket packages does not allow concurrent Read/Write operations
and does not provide low level access to websocket packet crafting.

Following the fasthttp philosophy this library tries to avoid extra-allocations
while providing concurrent access to Read/Write operations and stable API to be used
in production allowing low level access to the websocket frames.

To see an example of what this package CAN do that others DONT checkout [this](https://github.com/dgrr/fastws/blob/master/examples/concurrent_server.go)
or [this](https://github.com/dgrr/fastws/blob/master/examples/broadcast.go) examples.

# How it works? (Server)

Okay. It's easy. You have an
[Upgrader](https://godoc.org/github.com/dgrr/fastws#Upgrader)
which is used to upgrade your connection.
You must specify the
[Handler](https://godoc.org/github.com/dgrr/fastws#Upgrader.Handler)
to handle the request.

If you just want a websocket connection and don't want to be
a websocket expert you can just
use [Upgrade](https://godoc.org/github.com/dgrr/fastws#Upgrade) function passing the
handler.

```go
func main() {
  fasthttp.ListenAndServe(":8080", fastws.Upgrade(wsHandler))
}

func wsHandler(conn *Conn) {
  fmt.Fprintf(conn, "Hello world")
}
```

After this point you can handle your awesome websocket connection.
The connection is automatically
[closed](https://github.com/dgrr/fastws/blob/master/upgrader.go#L137)
by fastws when you exit your handler. YES! You are able to close
your connection if you want to send a
[close](https://godoc.org/github.com/dgrr/fastws#Conn.Close) message to the peer.

If you are looking for a low level usage of the library you can use the
[Frame](https://godoc.org/github.com/dgrr/fastws#Frame) structure
to handle frame by frame in a websocket connection.
Also you can use
[Conn.ReadFrame](https://godoc.org/github.com/dgrr/fastws#Conn.ReadFrame) or
[Conn.NextFrame](https://godoc.org/github.com/dgrr/fastws#Conn.NextFrame) to read
frame by frame from your peers.

```go
func main() {
  fasthttp.ListenAndServe(":8080", fastws.Upgrade(wsHandler))
}

func wsHandler(conn *Conn) {
  fmt.Fprintf(conn, "Hello world")

  fr, err := conn.NextFrame()
  if err != nil {
    panic(err)
  }

  fmt.Printf("Received: %s\n", fr.Payload())

  ReleaseFrame(fr)
}
```

All of the [Conn](https://godoc.org/github.com/dgrr/fastws#Conn)
functions are safe-concurrent. Ready to be used from different goroutines.
If you want to deal with the frames you can either use
[Conn](https://godoc.org/github.com/dgrr/fastws#Conn) or your own net.Conn,
but remember to manage concurrency when using a net.Conn

# How it works? (Client)

Just call [Dial](https://godoc.org/github.com/dgrr/fastws#Dial).

```go
conn, err := fastws.Dial("ws://localhost:8080/ws")
if err != nil {
  fmt.Println(err)
}
conn.WriteString("Hello")
```

# fastws vs gorilla vs nhooyr vs gobwas

| Features | [fastws](https://github.com/dgrr/fastws) | [Gorilla](https://github.com/savsgio/websocket)| [Nhooyr](https://github.com/nhooyr/websocket) | [gowabs](https://github.com/gobwas/ws) |
| --- | --- | --- | --- | --- |
| Concurrent R/W                          | Yes            | No           | No. Only writes | No           |
| Passes Autobahn Test Suite              | Mostly         | Yes          | Yes             | Mostly       |    
| Receive fragmented message              | Yes            | Yes          | Yes             | Yes          |
| Send close message                      | Yes            | Yes          | Yes             | Yes          |
| Send pings and receive pongs            | Yes            | Yes          | Yes             | Yes          |
| Get the type of a received data message | Yes            | Yes          | Yes             | Yes          |
| Compression Extensions                  | On development | Experimental | Yes             | No (?)       |
| Read message using io.Reader            | Non planned    | Yes          | No              | No (?)       |
| Write message using io.WriteCloser      | Non planned    | Yes          | No              | No (?)       |

# Benchmarks: fastws vs gorilla vs nhooyr vs gobwas

Fastws:
```
$ go test -bench=Fast -benchmem -benchtime=10s
Benchmark1000FastClientsPer10Messages-8          225367248    52.6 ns/op       0 B/op   0 allocs/op
Benchmark1000FastClientsPer100Messages-8        1000000000     5.48 ns/op      0 B/op   0 allocs/op
Benchmark1000FastClientsPer1000Messages-8       1000000000     0.593 ns/op     0 B/op   0 allocs/op
Benchmark100FastMsgsPerConn-8                   1000000000     7.38 ns/op      0 B/op   0 allocs/op
Benchmark1000FastMsgsPerConn-8                  1000000000     0.743 ns/op     0 B/op   0 allocs/op
Benchmark10000FastMsgsPerConn-8                 1000000000     0.0895 ns/op    0 B/op   0 allocs/op
Benchmark100000FastMsgsPerConn-8                1000000000     0.0186 ns/op    0 B/op   0 allocs/op
```

Gorilla:
```
$ go test -bench=Gorilla -benchmem -benchtime=10s
Benchmark1000GorillaClientsPer10Messages-8       128621386    97.5 ns/op      86 B/op   1 allocs/op
Benchmark1000GorillaClientsPer100Messages-8     1000000000    11.0 ns/op       8 B/op   0 allocs/op
Benchmark1000GorillaClientsPer1000Messages-8    1000000000     1.12 ns/op      0 B/op   0 allocs/op
Benchmark100GorillaMsgsPerConn-8                 849490059    14.0 ns/op       8 B/op   0 allocs/op
Benchmark1000GorillaMsgsPerConn-8               1000000000     1.42 ns/op      0 B/op   0 allocs/op
Benchmark10000GorillaMsgsPerConn-8              1000000000     0.143 ns/op     0 B/op   0 allocs/op
Benchmark100000GorillaMsgsPerConn-8             1000000000     0.0252 ns/op    0 B/op   0 allocs/op
```

Nhooyr:
```
$ go test -bench=Nhooyr -benchmem -benchtime=10s
Benchmark1000NhooyrClientsPer10Messages-8        121254158   114 ns/op        87 B/op   1 allocs/op
Benchmark1000NhooyrClientsPer100Messages-8      1000000000    11.1 ns/op       8 B/op   0 allocs/op
Benchmark1000NhooyrClientsPer1000Messages-8     1000000000     1.19 ns/op      0 B/op   0 allocs/op
Benchmark100NhooyrMsgsPerConn-8                  845071632    15.1 ns/op       8 B/op   0 allocs/op
Benchmark1000NhooyrMsgsPerConn-8                1000000000     1.47 ns/op      0 B/op   0 allocs/op
Benchmark10000NhooyrMsgsPerConn-8               1000000000     0.157 ns/op     0 B/op   0 allocs/op
Benchmark100000NhooyrMsgsPerConn-8              1000000000     0.0251 ns/op    0 B/op   0 allocs/op
```

Gobwas:
```
$ go test -bench=Gobwas -benchmem -benchtime=10s
Benchmark1000GobwasClientsPer10Messages-8         98497042   106 ns/op        86 B/op   1 allocs/op
Benchmark1000GobwasClientsPer100Messages-8      1000000000    13.4 ns/op       8 B/op   0 allocs/op
Benchmark1000GobwasClientsPer1000Messages-8     1000000000     1.19 ns/op      0 B/op   0 allocs/op
Benchmark100GobwasMsgsPerConn-8                  833576667    14.6 ns/op       8 B/op   0 allocs/op
Benchmark1000GobwasMsgsPerConn-8                1000000000     1.46 ns/op      0 B/op   0 allocs/op
Benchmark10000GobwasMsgsPerConn-8               1000000000     0.156 ns/op     0 B/op   0 allocs/op
Benchmark100000GobwasMsgsPerConn-8              1000000000     0.0262 ns/op    0 B/op   0 allocs/op
```

# Stress tests

The following stress test were performed without timeouts:

Executing `tcpkali --ws -c 100 -m 'hello world!!13212312!' -r 10k localhost:8081` the tests shows the following:

Fastws:
```
Total data sent:     267.4 MiB (280416485 bytes)
Total data received: 229.2 MiB (240330760 bytes)
Bandwidth per channel: 4.164⇅ Mbps (520.5 kBps)
Aggregate bandwidth: 192.172↓, 224.225↑ Mbps
Packet rate estimate: 153966.7↓, 47866.8↑ (1↓, 1↑ TCP MSS/op)
Test duration: 10.0048 s.
```

Gorilla:
```
Total data sent:     267.6 MiB (280594916 bytes)
Total data received: 165.8 MiB (173883303 bytes)
Bandwidth per channel: 3.632⇅ Mbps (454.0 kBps)
Aggregate bandwidth: 138.973↓, 224.260↑ Mbps
Packet rate estimate: 215158.9↓, 74635.5↑ (1↓, 1↑ TCP MSS/op)
Test duration: 10.0096 s.
```

Nhooyr: (Don't know why is that low)
```
Total data sent:     234.3 MiB (245645988 bytes)
Total data received: 67.7 MiB (70944682 bytes)
Bandwidth per channel: 2.532⇅ Mbps (316.5 kBps)
Aggregate bandwidth: 56.740↓, 196.461↑ Mbps
Packet rate estimate: 92483.9↓, 50538.6↑ (1↓, 1↑ TCP MSS/op)
Test duration: 10.0028 s
```

Gobwas:
```
Total data sent:     267.6 MiB (280591457 bytes)
Total data received: 169.5 MiB (177693000 bytes)
Bandwidth per channel: 3.664⇅ Mbps (458.0 kBps)
Aggregate bandwidth: 142.080↓, 224.356↑ Mbps
Packet rate estimate: 189499.0↓, 66535.5↑ (1↓, 1↑ TCP MSS/op)
Test duration: 10.0052 s.
```

The source files are in [this](https://github.com/dgrr/fastws/tree/master/stress-tests/) folder.
