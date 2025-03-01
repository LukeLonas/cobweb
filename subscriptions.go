package cobweb

import (
	"sync"
)

// Subscriptions manages topic-to-client mappings.
type Subscriptions struct {
	mu   sync.Mutex
	subs map[string]map[*Client]struct{}
}

// Add subscribes a client to a topic.
func (s *Subscriptions) Add(topic string, client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subs[topic]; !ok {
		s.subs[topic] = make(map[*Client]struct{})
	}
	s.subs[topic][client] = struct{}{}
}

// Remove unsubscribes a client from a topic.
func (s *Subscriptions) Remove(topic string, client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if clients, ok := s.subs[topic]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(s.subs, topic)
		}
	}
}

// GetClients returns all clients subscribed to a topic.
func (s *Subscriptions) GetClients(topic string) []*Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	clients, ok := s.subs[topic]
	if !ok {
		return nil
	}
	result := make([]*Client, 0, len(clients))
	for client := range clients {
		result = append(result, client)
	}
	return result
}
