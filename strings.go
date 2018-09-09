package fastws

var (
	originString      = []byte("Origin")
	connectionString  = []byte("Connection")
	upgradeString     = []byte("Upgrade")
	wsString          = []byte("websocket")
	commaString       = []byte(",")
	wsHeaderVersion   = []byte("Sec-WebSocket-Version")
	wsHeaderKey       = []byte("Sec-WebSocket-Key")
	wsHeaderProtocol  = []byte("Sec-Websocket-Protocol")
	wsHeaderAccept    = []byte("Sec-Websocket-Accept")
	uidKey            = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
	supportedVersions = [][]byte{ // must be slice for future implementations
		[]byte("13"),
	}
)
