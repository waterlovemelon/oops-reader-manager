package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/audit"
	"github.com/oops-reader/oops-reader-manager/service/internal/usersadmin"
)

type UsersHandler struct {
	store    usersadmin.Store
	auditSvc *audit.Service
}

func NewUsersHandler(store usersadmin.Store, auditSvc *audit.Service) *UsersHandler {
	return &UsersHandler{store: store, auditSvc: auditSvc}
}

func (h *UsersHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	users, total, err := h.store.List(c.Request.Context(), c.Query("q"), pageSize, (page-1)*pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users, "pagination": gin.H{"page": page, "page_size": pageSize, "total": total}})
}

type updateUserStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func (h *UsersHandler) UpdateStatus(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	userID := c.Param("id")
	var req updateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Status != "active" && req.Status != "frozen" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status, must be active or frozen"})
		return
	}
	if err := h.store.UpdateStatus(c.Request.Context(), userID, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.auditSvc.Record(c.Request.Context(), audit.Entry{
		AdminUsername: claims.Username,
		Action:        "status_change",
		ResourceType:  "user",
		ResourceID:    userID,
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": userID, "status": req.Status}})
}
