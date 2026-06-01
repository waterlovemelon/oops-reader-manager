package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/dashboard"
)

type DashboardHandler struct {
	store dashboard.Store
}

func NewDashboardHandler(store dashboard.Store) *DashboardHandler {
	return &DashboardHandler{store: store}
}

func (h *DashboardHandler) Summary(c *gin.Context) {
	summary, err := h.store.Summary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": summary})
}
