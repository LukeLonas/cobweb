package cobweb

import (
	"encoding/json"
	"errors"
	"net/rpc"
	"sync"

	"github.com/gorilla/websocket"
)

// Client represents a connected WebSocket client.
type Client struct {
	conn          *websocket.Conn
	subscriptions sync.Map // map[string]struct{}
	send          chan []byte
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

// WSCodec implements rpc.ServerCodec for WebSocket connections.
type WSCodec struct {
	client        *Client
	params        json.RawMessage
	serviceMethod string // Store the service method for use in ReadRequestBody
}

// ReadRequestHeader reads the JSON RPC request header.
func (c *WSCodec) ReadRequestHeader(req *rpc.Request) error {
	_, msg, err := c.client.conn.ReadMessage()
	if err != nil {
		return err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(msg, &m); err != nil {
		return err
	}
	method, ok := m["method"].(string)
	if !ok {
		return errors.New("invalid method")
	}
	req.ServiceMethod = method
	c.serviceMethod = method // Store it for ReadRequestBody
	if id, ok := m["id"]; ok {
		req.Seq = uint64(id.(float64)) // Assumes numeric ID
	} else {
		req.Seq = 0 // Notification
	}
	c.params = nil
	if params, ok := m["params"]; ok {
		p, err := json.Marshal(params)
		if err != nil {
			return err
		}
		c.params = p
	}
	return nil
}

// ReadRequestBody reads and populates the request body.
func (c *WSCodec) ReadRequestBody(body interface{}) error {
	if c.params == nil {
		return nil
	}
	if c.serviceMethod == "System.Subscribe" || c.serviceMethod == "System.Unsubscribe" {
		var topic string
		if err := json.Unmarshal(c.params, &topic); err != nil {
			return err
		}
		if req, ok := body.(*SubscribeReq); ok {
			req.Client = c.client
			req.Topic = topic
			return nil
		}
		return errors.New("invalid request type")
	}
	// Handle generic RPC methods with any number of parameters
	return json.Unmarshal(c.params, body)
}

// WriteResponse writes the JSON RPC response.
func (c *WSCodec) WriteResponse(resp *rpc.Response, body interface{}) error {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      resp.Seq,
	}
	if resp.Error != "" {
		response["error"] = resp.Error
	} else {
		response["result"] = body
	}
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}
	c.client.send <- data
	return nil
}

// Close closes the WebSocket connection.
func (c *WSCodec) Close() error {
	close(c.client.send)
	return c.client.conn.Close()
}
