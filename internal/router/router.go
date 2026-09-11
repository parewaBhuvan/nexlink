package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/parewaBhuvan/nexlink/internal/auth"
	"github.com/parewaBhuvan/nexlink/internal/middleware"
	"github.com/parewaBhuvan/nexlink/internal/ws"
)

func NewRouter(authService *auth.Service, hub *ws.Hub , wsService *ws.Service) *gin.Engine {
	r := gin.Default()

	// Public routes — no auth required
	r.POST("/auth/request-otp", authService.RequestOTPHandler)
	r.POST("/auth/verify-otp", authService.VerifyOTPHandler)
	
	r.GET("/ws", ws.UpgradeHandler(hub, wsService, authService))
	// Protected routes — require valid bearer token
	
	protected := r.Group("/api")
	protected.Use(middleware.AuthRequired(authService))
	{
		protected.GET("/ping", pingHandler)
		protected.PATCH("/users/me", func(c *gin.Context) {
			userID, ok := middleware.GetUserID(c)
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "user_id missing from context"})
				return
			}
			authService.UpdateUsernameHandler(c, userID)
		})
		protected.POST("/ws/ticket", func(c *gin.Context) {
			userID, ok := middleware.GetUserID(c)
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "user_id missing from context"})
				return
			}
			authService.CreateWSTicketHandler(c, userID)
		})
	}

	return r
}
func pingHandler(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(500, gin.H{"error": "user_id missing from context"})
		return
	}
	c.JSON(200, gin.H{"user_id": userID})
}
