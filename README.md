# Fastws

Websocket library for [fasthttp](https://github.com/valyala/fasthttp).

See [examples](https://github.com/dgrr/fastws/blob/master/examples) to see how to use it.

# Why another websocket package?

Other websocket packages does not allow concurrent Read/Write operations
and does not provide low level access to websocket packet crafting.

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

| Features | [fastws](https://github.com/dgrr/fastws) | [Gorilla](https://github.com/savsgio/websocket)|
| --------------------------------------- |:--------------:| -----:|
| Passes Autobahn Test Suite              | On development | Yes |
| Receive fragmented message              | Yes            | Yes  |
| Send close message                      | Yes            | Yes |
| Send pings and receive pongs            | Yes            | Yes |
| Get the type of a received data message | Yes            | Yes |
| Compression Extensions                  | On development | Experimental |
| Read message using io.Reader            | On development | Yes |
| Write message using io.WriteCloser      | On development | Yes |

# Benchmarks

Fastws:

```
$ go test -v -bench=Fast -benchmem -benchtime=10s
Benchmark1000FastClientsPer10Messages-2         50000000               317 ns/op               0 B/op          0 allocs/op
Benchmark1000FastClientsPer100Messages-2        500000000               36.7 ns/op             0 B/op          0 allocs/op
Benchmark1000FastClientsPer1000Messages-2       5000000000               3.63 ns/op            0 B/op          0 allocs/op
Benchmark100FastMsgsPerConn-2                   500000000               38.2 ns/op             0 B/op          0 allocs/op
Benchmark1000FastMsgsPerConn-2                  5000000000               3.67 ns/op            0 B/op          0 allocs/op
Benchmark10000FastMsgsPerConn-2                 10000000000              0.47 ns/op            0 B/op          0 allocs/op
Benchmark100000FastMsgsPerConn-2                10000000000              0.03 ns/op            0 B/op          0 allocs/op
```

Gorilla:
```
$ go test -v -bench=Gorilla -benchmem -benchtime=10s
Benchmark1000GorillaClientsPer10Messages-2      20000000               813 ns/op              86 B/op          1 allocs/op
Benchmark1000GorillaClientsPer100Messages-2     200000000               75.4 ns/op             8 B/op          0 allocs/op
Benchmark1000GorillaClientsPer1000Messages-2    2000000000               7.54 ns/op            0 B/op          0 allocs/op
Benchmark100GorillaMsgsPerConn-2                200000000               85.0 ns/op             8 B/op          0 allocs/op
Benchmark1000GorillaMsgsPerConn-2               2000000000               8.12 ns/op            0 B/op          0 allocs/op
Benchmark10000GorillaMsgsPerConn-2              10000000000              0.84 ns/op            0 B/op          0 allocs/op
Benchmark100000GorillaMsgsPerConn-2             10000000000              0.08 ns/op            0 B/op          0 allocs/op
```
