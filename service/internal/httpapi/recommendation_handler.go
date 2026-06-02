package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/audit"
	"github.com/oops-reader/oops-reader-manager/service/internal/recommendation"
)

type RecommendationHandler struct {
	service  *recommendation.Service
	auditSvc *audit.Service
}

func NewRecommendationHandler(service *recommendation.Service, auditSvc *audit.Service) *RecommendationHandler {
	return &RecommendationHandler{service: service, auditSvc: auditSvc}
}

func (h *RecommendationHandler) List(c *gin.Context) {
	page, pageSize := pageParams(c)
	status := recommendation.Status(c.Query("status"))
	recs, total, err := h.service.Store().List(c.Request.Context(), recommendation.ListInput{
		Query:  c.Query("q"),
		Status: status,
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result := make([]gin.H, len(recs))
	for i, r := range recs {
		result[i] = recommendationJSON(r)
	}
	c.JSON(http.StatusOK, gin.H{"data": result, "pagination": gin.H{"page": page, "page_size": pageSize, "total": total}})
}

func (h *RecommendationHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	rec, err := h.service.Store().FindByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, recommendation.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "recommendation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": recommendationJSON(*rec)})
}

type createRecommendationRequest struct {
	BookKey            string `json:"book_key" binding:"required"`
	Comment            string `json:"comment" binding:"required"`
	ScheduledPublishAt string `json:"scheduled_publish_at"`
}

func (h *RecommendationHandler) Create(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	var req createRecommendationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	input := recommendation.CreateInput{
		BookKey:   req.BookKey,
		Comment:   req.Comment,
		CreatedBy: claims.Username,
	}
	if req.ScheduledPublishAt != "" {
		t, err := time.Parse(time.RFC3339, req.ScheduledPublishAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scheduled_publish_at, use RFC3339"})
			return
		}
		input.ScheduledPublishAt = t
	}
	rec, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, recommendation.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.auditSvc.Record(c.Request.Context(), audit.Entry{
		AdminUsername: claims.Username,
		Action:        "create",
		ResourceType:  "recommendation",
		ResourceID:    strconv.FormatInt(rec.ID, 10),
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
	c.JSON(http.StatusCreated, gin.H{"data": recommendationJSON(rec)})
}

type updateRecommendationRequest struct {
	Comment            *string `json:"comment"`
	ScheduledPublishAt *string `json:"scheduled_publish_at"`
}

func (h *RecommendationHandler) Update(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req updateRecommendationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	input := recommendation.UpdateInput{
		ID:        id,
		Comment:   req.Comment,
		UpdatedBy: claims.Username,
	}
	if req.ScheduledPublishAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ScheduledPublishAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scheduled_publish_at, use RFC3339"})
			return
		}
		input.ScheduledPublishAt = &t
	}
	if err := h.service.Update(c.Request.Context(), input); err != nil {
		if errors.Is(err, recommendation.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, recommendation.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "recommendation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.auditSvc.Record(c.Request.Context(), audit.Entry{
		AdminUsername: claims.Username,
		Action:        "update",
		ResourceType:  "recommendation",
		ResourceID:    strconv.FormatInt(id, 10),
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
	rec, err := h.service.Store().FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": recommendationJSON(*rec)})
}

func (h *RecommendationHandler) Delete(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.service.SoftDelete(c.Request.Context(), id, claims.Username); err != nil {
		if errors.Is(err, recommendation.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "recommendation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.auditSvc.Record(c.Request.Context(), audit.Entry{
		AdminUsername: claims.Username,
		Action:        "delete",
		ResourceType:  "recommendation",
		ResourceID:    strconv.FormatInt(id, 10),
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "status": "deleted"}})
}

func recommendationJSON(rec recommendation.Recommendation) gin.H {
	return gin.H{
		"id":                   rec.ID,
		"book_key":             rec.BookKey,
		"comment":              rec.Comment,
		"status":               string(rec.Status),
		"scheduled_publish_at": rec.ScheduledPublishAt,
		"publish_state":        string(rec.PublishState(time.Now())),
		"created_by":           rec.CreatedBy,
		"updated_by":           rec.UpdatedBy,
		"created_at":           rec.CreatedAt,
		"updated_at":           rec.UpdatedAt,
	}
}
