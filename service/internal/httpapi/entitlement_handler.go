package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/audit"
	"github.com/oops-reader/oops-reader-manager/service/internal/entitlementadmin"
)

type EntitlementHandler struct {
	store    entitlementadmin.Store
	auditSvc *audit.Service
}

func NewEntitlementHandler(store entitlementadmin.Store, auditSvc *audit.Service) *EntitlementHandler {
	return &EntitlementHandler{store: store, auditSvc: auditSvc}
}

func (h *EntitlementHandler) ListByUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	entitlements, err := h.store.ListByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entitlements})
}

type createEntitlementRequest struct {
	EntitlementKey string  `json:"entitlement_key" binding:"required"`
	Source         string  `json:"source"`
	StartsAt       string  `json:"starts_at" binding:"required"`
	ExpiresAt      *string `json:"expires_at"`
}

func (h *EntitlementHandler) Create(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	var req createEntitlementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid starts_at format, use RFC3339"})
		return
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expires_at format, use RFC3339"})
			return
		}
		expiresAt = &t
	}
	source := req.Source
	if source == "" {
		source = "admin"
	}
	id, err := h.store.Create(c.Request.Context(), entitlementadmin.Entitlement{
		UserID:         userID,
		EntitlementKey: req.EntitlementKey,
		Source:         source,
		StartsAt:       startsAt,
		ExpiresAt:      expiresAt,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.auditSvc.Record(c.Request.Context(), audit.Entry{
		AdminUsername: claims.Username,
		Action:        "create",
		ResourceType:  "entitlement",
		ResourceID:    strconv.FormatInt(id, 10),
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": id}})
}

func (h *EntitlementHandler) Revoke(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entitlement id"})
		return
	}
	if err := h.store.Revoke(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.auditSvc.Record(c.Request.Context(), audit.Entry{
		AdminUsername: claims.Username,
		Action:        "revoke",
		ResourceType:  "entitlement",
		ResourceID:    strconv.FormatInt(id, 10),
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "status": "revoked"}})
}

type extendEntitlementRequest struct {
	ExpiresAt string `json:"expires_at" binding:"required"`
}

func (h *EntitlementHandler) Extend(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entitlement id"})
		return
	}
	var req extendEntitlementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	newExpiry, err := time.Parse(time.RFC3339, req.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expires_at format, use RFC3339"})
		return
	}
	if err := h.store.Extend(c.Request.Context(), id, newExpiry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.auditSvc.Record(c.Request.Context(), audit.Entry{
		AdminUsername: claims.Username,
		Action:        "extend",
		ResourceType:  "entitlement",
		ResourceID:    strconv.FormatInt(id, 10),
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "expires_at": newExpiry}})
}
