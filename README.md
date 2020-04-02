# Fastws

Websocket library for [fasthttp](https://github.com/valyala/fasthttp).

Checkout [examples](https://github.com/dgrr/fastws/blob/master/examples) to get inspiration.

# Why another websocket package?

Other websocket packages does not allow concurrent Read/Write operations
and does not provide low level access to websocket packet crafting.

To see a example of what this package CAN do that others NOT see [this](https://github.com/dgrr/fastws/blob/master/examples/concurrent_server.go) example.

Following the fasthttp philosophy this library tries to avoid extra-allocations
while providing concurrent access to Read/Write operations and stable API to be used
in production allowing low level access to the websocket frames.

# How it works? (Server)

Okay. It's easy. You have an
[Upgrader](https://godoc.org/github.com/dgrr/fastws#Upgrader)
which is used to upgrade your connection.
You must specify the
[Handler](https://godoc.org/github.com/dgrr/fastws#Upgrader.Handler)
to handle the request.

If you just want a websocket connection and don't want to be
a websocket expert you can just
use [Upgrade](https://godoc.org/github.com/dgrr/fastws#Upgrade) function parsing the
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
[closed](https://github.com/dgrr/fastws/blob/master/upgrader.go#L80)
by fastws when you exit your handler but you can close
your connection if you wanna send
[close](https://godoc.org/github.com/dgrr/fastws#Conn.Close) message to the peer.

If you want low level usage of the library your can use
[Frame](https://godoc.org/github.com/dgrr/fastws#Frame) structure
to handle frame by frame in a websocket connection.
Also you can use
[Conn.ReadFrame](https://godoc.org/github.com/dgrr/fastws#Conn.ReadFrame) or
[Conn.NextFrame](https://godoc.org/github.com/dgrr/fastws#Conn.NextFrame) to read
frame by frame from your connection peer.

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

All of this functions are safe-concurrent. Ready to be used from different goroutines.

# How it works? (Client)

Just call [Dial](https://godoc.org/github.com/dgrr/fastws#Dial).

```go
conn, err := fastws.Dial("ws://localhost:8080/ws")
if err != nil {
  fmt.Println(err)
}
conn.WriteString("Hello")
```

# fastws vs gorilla.

| Features | [fastws](https://github.com/dgrr/fastws) | [Gorilla](https://github.com/savsgio/websocket)| [Nhooyr](https://github.com/nhooyr/websocket)
| --------------------------------------- |:--------------:| ------------:| ---------------:|
| Concurrent R/W                          | Yes            | No           | No. Only writes |
| Passes Autobahn Test Suite              | Mostly         | Yes          | Yes             |
| Receive fragmented message              | Yes            | Yes          | Yes             |
| Send close message                      | Yes            | Yes          | Yes             |
| Send pings and receive pongs            | Yes            | Yes          | Yes             |
| Get the type of a received data message | Yes            | Yes          | Yes             |
| Compression Extensions                  | On development | Experimental | Yes             |
| Read message using io.Reader            | Non planned    | Yes          | No              |
| Write message using io.WriteCloser      | Non planned    | Yes          | No              |

# Benchmarks

Fastws:

```
$ go test -bench=Fast -benchmem -benchtime=10s
enchmark1000FastClientsPer10Messages-8         226068555               54.1 ns/op             0 B/op          0 allocs/op
Benchmark1000FastClientsPer100Messages-8        1000000000               5.91 ns/op            0 B/op          0 allocs/op
Benchmark1000FastClientsPer1000Messages-8       1000000000               0.576 ns/op           0 B/op          0 allocs/op
Benchmark100FastMsgsPerConn-8                   1000000000               8.34 ns/op            0 B/op          0 allocs/op
Benchmark1000FastMsgsPerConn-8                  1000000000               0.893 ns/op           0 B/op          0 allocs/op
Benchmark10000FastMsgsPerConn-8                 1000000000               0.0954 ns/op          0 B/op          0 allocs/op
Benchmark100000FastMsgsPerConn-8                1000000000               0.0197 ns/op          0 B/op          0 allocs/op
```

Gorilla:
```
$ go test -bench=Gorilla -benchmem -benchtime=10s
Benchmark1000GorillaClientsPer10Messages-8      128621386         97.5 ns/op        86 B/op        1 allocs/op
Benchmark1000GorillaClientsPer100Messages-8     1000000000          11.0 ns/op         8 B/op        0 allocs/op
Benchmark1000GorillaClientsPer1000Messages-8    1000000000           1.12 ns/op        0 B/op        0 allocs/op
Benchmark100GorillaMsgsPerConn-8                849490059         14.0 ns/op         8 B/op        0 allocs/op
Benchmark1000GorillaMsgsPerConn-8               1000000000           1.42 ns/op        0 B/op        0 allocs/op
Benchmark10000GorillaMsgsPerConn-8              1000000000           0.143 ns/op         0 B/op        0 allocs/op
Benchmark100000GorillaMsgsPerConn-8             1000000000           0.0252 ns/op        0 B/op        0 allocs/op
```

Nhooyr:
```
$ go test -bench=Nhooyr -benchmem -benchtime=10s
Benchmark1000NhooyrClientsPer10Messages-8       121254158        114 ns/op        87 B/op        1 allocs/op
Benchmark1000NhooyrClientsPer100Messages-8      1000000000          11.1 ns/op         8 B/op        0 allocs/op
Benchmark1000NhooyrClientsPer1000Messages-8     1000000000           1.19 ns/op        0 B/op        0 allocs/op
Benchmark100NhooyrMsgsPerConn-8                 845071632         15.1 ns/op         8 B/op        0 allocs/op
Benchmark1000NhooyrMsgsPerConn-8                1000000000           1.47 ns/op        0 B/op        0 allocs/op
Benchmark10000NhooyrMsgsPerConn-8               1000000000           0.157 ns/op         0 B/op        0 allocs/op
Benchmark100000NhooyrMsgsPerConn-8              1000000000           0.0251 ns/op        0 B/op        0 allocs/op
```
