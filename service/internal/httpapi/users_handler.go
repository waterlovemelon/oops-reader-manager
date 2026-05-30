package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/usersadmin"
)

type UsersHandler struct {
	store usersadmin.Store
}

func NewUsersHandler(store usersadmin.Store) *UsersHandler {
	return &UsersHandler{store: store}
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
