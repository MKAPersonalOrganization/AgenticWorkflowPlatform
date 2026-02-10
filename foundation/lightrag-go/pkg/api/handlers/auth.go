// Package handlers provides HTTP handlers for the LightRAG API.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lightrag/lightrag-go/pkg/api/models"
	"github.com/lightrag/lightrag-go/pkg/auth"
	"github.com/lightrag/lightrag-go/pkg/config"
)

// AuthHandler handles authentication requests.
type AuthHandler struct {
	cfg         *config.Config
	authHandler *auth.AuthHandler
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(cfg *config.Config, authHandler *auth.AuthHandler) *AuthHandler {
	return &AuthHandler{
		cfg:         cfg,
		authHandler: authHandler,
	}
}

// Login handles login requests.
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest

	// Support both form and JSON
	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Detail: "Invalid request: " + err.Error(),
			})
			return
		}
	} else {
		// Form data (OAuth2 standard)
		req.Username = c.PostForm("username")
		req.Password = c.PostForm("password")

		if req.Username == "" || req.Password == "" {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Detail: "Username and password are required",
			})
			return
		}
	}

	// Validate credentials
	if !h.authHandler.ValidateCredentials(req.Username, req.Password) {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Detail: "Invalid username or password",
		})
		return
	}

	// Create token
	token, err := h.authHandler.CreateToken(req.Username, "user", nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Detail: "Failed to create token: " + err.Error(),
		})
		return
	}

	// Calculate expiration in seconds
	expiresIn := int(h.cfg.TokenExpireHours * 3600)

	c.JSON(http.StatusOK, models.LoginResponse{
		AccessToken: token,
		TokenType:   "bearer",
		ExpiresIn:   expiresIn,
	})
}

// GuestLogin handles guest login requests.
func (h *AuthHandler) GuestLogin(c *gin.Context) {
	// Create guest token
	token, err := h.authHandler.CreateToken("guest", "guest", nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Detail: "Failed to create guest token: " + err.Error(),
		})
		return
	}

	// Calculate expiration in seconds
	expiresIn := int(h.cfg.GuestTokenExpireHours * 3600)

	c.JSON(http.StatusOK, models.LoginResponse{
		AccessToken: token,
		TokenType:   "bearer",
		ExpiresIn:   expiresIn,
	})
}

// Me returns the current user information.
func (h *AuthHandler) Me(c *gin.Context) {
	username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Detail: "Not authenticated",
		})
		return
	}

	role, _ := c.Get("role")
	tokenInfo, _ := c.Get("token_info")

	response := gin.H{
		"username": username,
		"role":     role,
	}

	if info, ok := tokenInfo.(*auth.TokenInfo); ok {
		response["metadata"] = info.Metadata
		response["expires_at"] = info.Exp
	}

	c.JSON(http.StatusOK, response)
}

// RefreshToken handles token refresh requests.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	tokenInfo, exists := c.Get("token_info")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Detail: "Not authenticated",
		})
		return
	}

	info, ok := tokenInfo.(*auth.TokenInfo)
	if !ok {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Detail: "Invalid token info",
		})
		return
	}

	// Create new token
	token, err := h.authHandler.CreateToken(info.Username, info.Role, nil, info.Metadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Detail: "Failed to refresh token: " + err.Error(),
		})
		return
	}

	// Calculate expiration in seconds
	var expiresIn int
	if info.Role == "guest" {
		expiresIn = int(h.cfg.GuestTokenExpireHours * 3600)
	} else {
		expiresIn = int(h.cfg.TokenExpireHours * 3600)
	}

	c.JSON(http.StatusOK, models.LoginResponse{
		AccessToken: token,
		TokenType:   "bearer",
		ExpiresIn:   expiresIn,
	})
}
