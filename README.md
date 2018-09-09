# Fastws

Websocket library for [fasthttp](https://github.com/valyala/fasthttp).

See [examples](https://github.com/dgrr/fastws/blob/master/examples) to see how to use it.

# Why another websocket package?

Other websocket packages does not allow concurrent Read/Write operations
and a does not provide low level access to websocket packet crafting.

Following the fasthttp philosophy this library tries to avoid extra-allocations
while providing concurrent access to Read/Write operations and stable API to be used
in production allowing low level access to the websocket frames.
