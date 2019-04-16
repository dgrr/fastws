package fastws

var (
	wsString            = []byte("ws")
	wssString           = []byte("wss")
	originString        = []byte("Origin")
	connectionString    = []byte("Connection")
	upgradeString       = []byte("Upgrade")
	websocketString     = []byte("WebSocket")
	commaString         = []byte(",")
	wsHeaderVersion     = []byte("Sec-WebSocket-Version")
	wsHeaderKey         = []byte("Sec-WebSocket-Key")
	wsHeaderProtocol    = []byte("Sec-Websocket-Protocol")
	wsHeaderAccept      = []byte("Sec-Websocket-Accept")
	wsHeaderExtensions  = []byte("Sec-WebSocket-Extensions")
	permessageDeflate   = []byte("permessage-deflate")
	serverNoCtxTakeover = []byte("server_no_context_takeover")
	clientNoCtxTakeover = []byte("client_no_context_takeover")
	uidKey              = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
	supportedVersions   = [][]byte{ // must be slice for future implementations
		[]byte("13"),
	}
)
