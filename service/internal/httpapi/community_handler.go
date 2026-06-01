package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/audit"
	"github.com/oops-reader/oops-reader-manager/service/internal/communityadmin"
)

type CommunityHandler struct {
	store    communityadmin.Store
	auditSvc *audit.Service
}

func NewCommunityHandler(store communityadmin.Store, auditSvc *audit.Service) *CommunityHandler {
	return &CommunityHandler{store: store, auditSvc: auditSvc}
}

type updateStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func (h *CommunityHandler) ListThreads(c *gin.Context) {
	page, pageSize := pageParams(c)
	threads, total, err := h.store.ListThreads(c.Request.Context(), c.Query("q"), pageSize, (page-1)*pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": threads, "pagination": gin.H{"page": page, "page_size": pageSize, "total": total}})
}

func (h *CommunityHandler) UpdateThreadStatus(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	threadID := c.Param("id")
	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Status != "active" && req.Status != "hidden" && req.Status != "locked" && req.Status != "deleted" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	if err := h.store.UpdateThreadStatus(c.Request.Context(), threadID, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.auditSvc.Record(c.Request.Context(), audit.Entry{
		AdminUsername: claims.Username,
		Action:        "status_change",
		ResourceType:  "thread",
		ResourceID:    threadID,
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": threadID, "status": req.Status}})
}

func (h *CommunityHandler) ListComments(c *gin.Context) {
	page, pageSize := pageParams(c)
	comments, total, err := h.store.ListComments(c.Request.Context(), c.Query("thread_id"), pageSize, (page-1)*pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": comments, "pagination": gin.H{"page": page, "page_size": pageSize, "total": total}})
}

func (h *CommunityHandler) UpdateCommentStatus(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	commentID := c.Param("id")
	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Status != "active" && req.Status != "hidden" && req.Status != "deleted" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	if err := h.store.UpdateCommentStatus(c.Request.Context(), commentID, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.auditSvc.Record(c.Request.Context(), audit.Entry{
		AdminUsername: claims.Username,
		Action:        "status_change",
		ResourceType:  "comment",
		ResourceID:    commentID,
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": commentID, "status": req.Status}})
}

func pageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return page, pageSize
}
