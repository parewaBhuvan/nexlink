package router

import (
	"github.com/gin-gonic/gin"

	"github.com/parewaBhuvan/nexlink/internal/auth"
)

func NewRouter(authService *auth.Service) *gin.Engine {
	r := gin.Default()

	r.POST("/auth/request-otp", authService.RequestOTPHandler)
	r.POST("/auth/verify-otp", authService.VerifyOTPHandler)

	return r
}