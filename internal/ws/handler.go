package ws

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/parewaBhuvan/nexlink/internal/auth"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: restrict to actual frontend origin before production
	},
}

func UpgradeHandler(hub *Hub, svc *Service, authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticket := c.Query("ticket")
		if ticket == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing ticket"})
			return
		}

		userID, err := authService.ConsumeWSTicket(c.Request.Context(), ticket)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired ticket"})
			return
		}

		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("websocket upgrade failed for user %s: %v", userID, err)
			return
		}

		conn := NewConnection(hub, ws, userID, svc)
		hub.register <- conn

		go conn.WritePump()
		go conn.ReadPump()
	}
}

const (
	defaultMessageLimit = 50
	maxMessageLimit     = 100
)

func (s *Service) GetMessagesHandler(c *gin.Context, userID string) {
	conversationID := c.Param("conversation_id")

	isParticipant, err := s.IsParticipant(c.Request.Context(), conversationID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if !isParticipant {
		c.JSON(http.StatusForbidden, gin.H{"error": "you are not a participant in this conversation"})
		return
	}

	limit := defaultMessageLimit
	if limitStr := c.Query("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		if parsedLimit > maxMessageLimit {
			parsedLimit = maxMessageLimit
		}
		limit = parsedLimit
	}

	var before *time.Time
	if beforeStr := c.Query("before"); beforeStr != "" {
		unixSeconds, err := strconv.ParseInt(beforeStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid before cursor, expected unix timestamp"})
			return
		}
		parsedBefore := time.Unix(unixSeconds, 0)
		before = &parsedBefore
	}

	messages, err := s.GetMessages(c.Request.Context(), conversationID, before, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	var nextCursor *int64
	if len(messages) > 0 {
		unixTime := messages[len(messages)-1].CreatedAt.Unix()
		nextCursor = &unixTime
	}

	c.JSON(http.StatusOK, gin.H{
		"messages":    messages,
		"next_cursor": nextCursor,
	})
}
