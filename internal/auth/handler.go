package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type requestOTPBody struct {
	Mobile string `json:"mobile" binding:"required"`
}

func (s *Service) RequestOTPHandler(c *gin.Context) {
	var body requestOTPBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mobile is required"})
		return
	}

	err := s.RequestOTP(c.Request.Context(), body.Mobile)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidMobile):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCooldownActive):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		case errors.Is(err, ErrSendFailed):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OTP sent"})
}

type verifyOTPBody struct {
	Mobile string `json:"mobile" binding:"required"`
	OTP    string `json:"otp" binding:"required"`
}

func (s *Service) VerifyOTPHandler(c *gin.Context) {
	var body verifyOTPBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mobile and otp are required"})
		return
	}

	ctx := c.Request.Context()

	if err := s.VerifyOTP(ctx, body.Mobile, body.OTP); err != nil {
		switch {
		case errors.Is(err, ErrOTPExpiredOrNotFound):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrOTPMismatch):
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		case errors.Is(err, ErrTooManyAttempts):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected error"})
		}
		return
	}

	userID, err := s.GetOrCreateUser(ctx, body.Mobile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
		return
	}

	token, err := s.CreateSession(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

type UpdateUsernameRequest struct {
	UserName string `json:"user_name" binding:"required"`
}

func (s *Service) UpdateUsernameHandler(c *gin.Context, userID string) {
	var req UpdateUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	err := s.UpdateUsername(c.Request.Context(), userID, req.UserName)
	if err != nil {
		switch {
		case errors.Is(err, ErrUsernameTooShort):
			c.JSON(http.StatusBadRequest, gin.H{"error": ErrUsernameTooShort.Error()})
		case errors.Is(err, ErrUsernameTaken):
			c.JSON(http.StatusConflict, gin.H{"error": ErrUsernameTaken.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"user_name": req.UserName})
}
