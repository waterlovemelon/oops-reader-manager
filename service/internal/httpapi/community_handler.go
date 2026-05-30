package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/communityadmin"
)

type CommunityHandler struct {
	store communityadmin.Store
}

func NewCommunityHandler(store communityadmin.Store) *CommunityHandler {
	return &CommunityHandler{store: store}
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
	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.store.UpdateThreadStatus(c.Request.Context(), c.Param("id"), req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": c.Param("id"), "status": req.Status}})
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
	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.store.UpdateCommentStatus(c.Request.Context(), c.Param("id"), req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": c.Param("id"), "status": req.Status}})
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
