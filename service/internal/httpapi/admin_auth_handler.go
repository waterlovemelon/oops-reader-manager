package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/adminauth"
)

type AdminAuthHandler struct {
	service *adminauth.Service
}

func NewAdminAuthHandler(service *adminauth.Service) *AdminAuthHandler {
	return &AdminAuthHandler{service: service}
}

type adminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AdminAuthHandler) Login(c *gin.Context) {
	var req adminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	token, err := h.service.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"access_token": token}})
}

func (h *AdminAuthHandler) Me(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"username": claims.Username}})
}

func AdminRequired(service *adminauth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		value := c.GetHeader("Authorization")
		token := strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
		if token == "" || token == value {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
			return
		}
		claims, err := service.Validate(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("admin_claims", claims)
		c.Next()
	}
}

func CurrentAdmin(c *gin.Context) (adminauth.Claims, bool) {
	value, ok := c.Get("admin_claims")
	if !ok {
		return adminauth.Claims{}, false
	}
	claims, ok := value.(adminauth.Claims)
	return claims, ok
}
