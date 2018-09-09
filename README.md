# Fastws

Websocket library for [fasthttp](https://github.com/valyala/fasthttp).

See [examples](https://github.com/dgrr/fastws/blob/master/examples) to see how to use it.

# Why another websocket package?

Another websocket packages does not allow concurrent Read/Write operations
and a low level access to websocket packat crafting.

Following the fasthttp philosophy this library tries to avoid extra-allocations
while providing concurrent access to Read/Write operations and stable API to be used
in production.
