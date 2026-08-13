package websocket

// Client represents an individual connected WebSocket client.
type Client struct {
	ID   string
	Send chan LogMessage
}

// NewClient instantiates a new Client.
func NewClient(id string) *Client {
	return &Client{
		ID:   id,
		Send: make(chan LogMessage, 128),
	}
}
