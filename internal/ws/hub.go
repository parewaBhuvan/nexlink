package ws

import "log"

type OutboundMessage struct {
	RecipientIDs []string
	Payload      []byte
}

type Hub struct {
	connections map[string]*Connection
	register    chan *Connection
	unregister  chan *Connection
	broadcast   chan *OutboundMessage
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[string]*Connection),
		register:    make(chan *Connection),
		unregister:  make(chan *Connection),
		broadcast:   make(chan *OutboundMessage),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.connections[conn.UserID] = conn

		case conn := <-h.unregister:
			if existing, ok := h.connections[conn.UserID]; ok && existing == conn {
				delete(h.connections, conn.UserID)
				close(existing.Send)
			}

		case msg := <-h.broadcast:
			// log.Printf("broadcasting to recipients: %v", msg.RecipientIDs)
			// log.Printf("currently registered connections: %v", h.connections)
			for _, uid := range msg.RecipientIDs {
				if conn, ok := h.connections[uid]; ok {
					select {
					case conn.Send <- msg.Payload:
					default:
						log.Printf("dropping message for user %s: send buffer full", uid)
						delete(h.connections, uid)
						close(conn.Send)
					}
				}
			}
		}
	}
}
