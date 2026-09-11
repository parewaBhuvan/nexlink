package ws

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/parewaBhuvan/nexlink/internal/auth"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: restrict to your actual frontend origin before production
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