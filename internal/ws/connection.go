package ws

import (
	"context"
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

const sendBufferSize = 256

type Connection struct {
	UserID string
	Hub    *Hub
	Send   chan []byte
	WS     *websocket.Conn
	Svc    *Service
}
type IncomingMessage struct {
	ConversationID string `json:"conversation_id"`
	Content        string `json:"content"`
}

func NewConnection(hub *Hub, ws *websocket.Conn, userId string, svc *Service) *Connection {
	return &Connection{
		UserID: userId,
		Hub:    hub, 
		Send:   make(chan []byte, sendBufferSize),
		WS:     ws,
		Svc:    svc,
	}
}

func (c *Connection) ReadPump() {
	defer func() {
		c.Hub.unregister <- c
		c.WS.Close()
	}()

	for {
		_, data, err := c.WS.ReadMessage()
		if err != nil {
			log.Printf("read error for user %s: %v", c.UserID, err)
			break
		}

		var msg IncomingMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("invalid message from user %s: %v", c.UserID, err)
			continue
		}

		ctx := context.Background()

		participantIDs, err := c.Svc.GetParticipants(ctx, msg.ConversationID)
		if err != nil {
			log.Printf("failed to get participants for conversation %s: %v", msg.ConversationID, err)
			continue
		}

		if _, err := c.Svc.SaveMessage(ctx, msg.ConversationID, c.UserID, msg.Content); err != nil {
			log.Printf("failed to save message: %v", err)
			continue
		}

		c.Hub.broadcast <- &OutboundMessage{
			RecipientIDs: participantIDs,
			Payload:      data,
		}
	}
}

func (c *Connection) WritePump() {
	defer c.WS.Close()

	for msg := range c.Send {
		if err := c.WS.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Printf("write error for user %s: %v", c.UserID, err)
			return
		}
	}
}
