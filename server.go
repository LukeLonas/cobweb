package cobweb

import (
	"encoding/json"
	"log"
	"net/http"
	"net/rpc"
	"sync"

	"github.com/gorilla/websocket"
)

// Global RPC server and subscriptions
var (
	server        = rpc.NewServer()
	subscriptions = &Subscriptions{subs: make(map[string]map[*Client]struct{})}
	upgrader      = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func init() {
	// Register built-in System service for subscriptions
	server.RegisterName("System", new(System))
}

// Register registers a service's methods for RPC handling.
func Register(service interface{}) error {
	return server.Register(service)
}

// NewHandler returns an http.Handler for WebSocket connections.
func NewHandler() http.Handler {
	return &handler{}
}

type handler struct{}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	log.Println("WebSocket connection established")
	client := &Client{
		conn:          conn,
		subscriptions: sync.Map{},
		send:          make(chan []byte, 256),
	}
	go client.writePump()
	codec := &WSCodec{client: client}
	server.ServeCodec(codec)
	// Clean up subscriptions on disconnect
	client.subscriptions.Range(func(key, value interface{}) bool {
		topic := key.(string)
		subscriptions.Remove(topic, client)
		return true
	})
}

// Publish sends a message to all clients subscribed to a topic.
func Publish(topic string, message interface{}) error {
	//data, err := json.Marshal(message)
	//if err != nil {
	//	return err
	//}
	publishMsg := map[string]interface{}{
		"type":  "publish",
		"topic": topic,
		"data":  message,
	}
	msg, err := json.Marshal(publishMsg)
	if err != nil {
		return err
	}
	for _, client := range subscriptions.GetClients(topic) {
		select {
		case client.send <- msg:
		default:
			// Log dropped message in production
		}
	}
	return nil
}

// System provides built-in RPC methods for subscription management.
type System struct{}

// SubscribeReq holds subscription request data.
type SubscribeReq struct {
	Client *Client // Injected by codec
	Topic  string
}

// Subscribe adds a client to a topic's subscribers.
func (s *System) Subscribe(req SubscribeReq, reply *string) error {
	subscriptions.Add(req.Topic, req.Client)
	req.Client.subscriptions.Store(req.Topic, struct{}{})
	*reply = "ok"
	return nil
}

// Unsubscribe removes a client from a topic's subscribers.
func (s *System) Unsubscribe(req SubscribeReq, reply *string) error {
	subscriptions.Remove(req.Topic, req.Client)
	req.Client.subscriptions.Delete(req.Topic)
	*reply = "ok"
	return nil
}
