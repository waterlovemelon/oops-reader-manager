package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/audit"
)

type AuditHandler struct {
	store audit.Store
}

func NewAuditHandler(store audit.Store) *AuditHandler {
	return &AuditHandler{store: store}
}

func (h *AuditHandler) List(c *gin.Context) {
	page, pageSize := pageParams(c)
	input := audit.ListInput{
		AdminUsername: c.Query("admin"),
		ResourceType:  c.Query("resource_type"),
		StartTime:     c.Query("start"),
		EndTime:       c.Query("end"),
		Limit:         pageSize,
		Offset:        (page - 1) * pageSize,
	}
	entries, total, err := h.store.List(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result := make([]gin.H, len(entries))
	for i, e := range entries {
		result[i] = gin.H{
			"id":            e.ID,
			"admin_username": e.AdminUsername,
			"action":        e.Action,
			"resource_type": e.ResourceType,
			"resource_id":   e.ResourceID,
			"before_json":   string(e.BeforeJSON),
			"after_json":    string(e.AfterJSON),
			"ip_address":    e.IPAddress,
			"user_agent":    e.UserAgent,
			"created_at":    e.CreatedAt,
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": result, "pagination": gin.H{"page": page, "page_size": pageSize, "total": total}})
}
