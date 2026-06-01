package httpapi

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/audit"
	"github.com/oops-reader/oops-reader-manager/service/internal/catalog"
)

type CatalogHandler struct {
	service  *catalog.Service
	tempRoot string
	auditSvc *audit.Service
}

func NewCatalogHandler(service *catalog.Service, tempRoot string, auditSvc *audit.Service) *CatalogHandler {
	return &CatalogHandler{service: service, tempRoot: tempRoot, auditSvc: auditSvc}
}

func (h *CatalogHandler) Upload(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	if err := os.MkdirAll(h.tempRoot, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tempPath := filepath.Join(h.tempRoot, file.Filename+".upload")
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	book, err := h.service.ImportUploadedFile(c.Request.Context(), catalog.UploadInput{
		AdminUsername:    claims.Username,
		OriginalFilename: file.Filename,
		TempPath:         tempPath,
	})
	if err != nil {
		_ = os.Remove(tempPath)
		status := http.StatusInternalServerError
		if errors.Is(err, catalog.ErrUnsupportedFormat) || errors.Is(err, catalog.ErrDuplicateBook) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": bookJSON(book)})
}

func (h *CatalogHandler) List(c *gin.Context) {
	page, pageSize := pageParams(c)
	status := catalog.BookStatus(c.Query("status"))
	books, total, err := h.service.Store().List(c.Request.Context(), c.Query("q"), status, pageSize, (page-1)*pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result := make([]gin.H, len(books))
	for i, b := range books {
		result[i] = bookJSON(b)
	}
	c.JSON(http.StatusOK, gin.H{"data": result, "pagination": gin.H{"page": page, "page_size": pageSize, "total": total}})
}

func (h *CatalogHandler) Get(c *gin.Context) {
	book, err := h.service.Store().FindByKey(c.Request.Context(), c.Param("book_key"))
	if err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "book not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": bookJSON(*book)})
}

func (h *CatalogHandler) Cover(c *gin.Context) {
	bookKey := c.Param("book_key")
	cover, _, err := h.service.GetCover(c.Request.Context(), bookKey)
	if err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "cover not found"})
			return
		}
		if errors.Is(err, catalog.ErrUnsupportedFormat) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cover not available for this format"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, cover.MediaType, cover.Data)
}

type updateBookRequest struct {
	Title       *string `json:"title"`
	Author      *string `json:"author"`
	Description *string `json:"description"`
	Language    *string `json:"language"`
}

func (h *CatalogHandler) Update(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	bookKey := c.Param("book_key")
	existing, err := h.service.Store().FindByKey(c.Request.Context(), bookKey)
	if err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "book not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var req updateBookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	before := *existing
	if req.Title != nil {
		existing.Title = *req.Title
	}
	if req.Author != nil {
		existing.Author = *req.Author
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Language != nil {
		existing.Language = *req.Language
	}
	existing.UpdatedBy = claims.Username
	if err := h.service.Store().Update(c.Request.Context(), *existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.auditSvc.Record(c.Request.Context(), audit.Entry{
		AdminUsername: claims.Username,
		Action:        "update",
		ResourceType:  "book",
		ResourceID:    bookKey,
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
	_ = before // before/after JSON 可在后续增强中序列化
	c.JSON(http.StatusOK, gin.H{"data": bookJSON(*existing)})
}

type updateBookStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func (h *CatalogHandler) UpdateStatus(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	bookKey := c.Param("book_key")
	var req updateBookStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	newStatus := catalog.BookStatus(req.Status)
	if newStatus != catalog.StatusActive && newStatus != catalog.StatusHidden && newStatus != catalog.StatusDeleted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status, must be active, hidden, or deleted"})
		return
	}
	existing, err := h.service.Store().FindByKey(c.Request.Context(), bookKey)
	if err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "book not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	oldStatus := existing.Status
	if err := h.service.Store().UpdateStatus(c.Request.Context(), bookKey, newStatus, claims.Username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.auditSvc.Record(c.Request.Context(), audit.Entry{
		AdminUsername: claims.Username,
		Action:        "status_change",
		ResourceType:  "book",
		ResourceID:    bookKey,
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
	_ = oldStatus
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"book_key": bookKey, "status": string(newStatus)}})
}

func bookJSON(book catalog.Book) gin.H {
	return gin.H{
		"book_key":        book.BookKey,
		"title":           book.Title,
		"author":          book.Author,
		"description":     book.Description,
		"format":          book.Format,
		"filename":        book.Filename,
		"storage_path":    book.StoragePath,
		"cover_storage_path": book.CoverStoragePath,
		"file_size":       book.FileSize,
		"content_sha1":    book.ContentSHA1,
		"language":        book.Language,
		"chapter_count":   book.ChapterCount,
		"word_count":      nullableWordCount(book.WordCount),
		"status":          string(book.Status),
		"source":          book.Source,
		"uploaded_at":     book.UploadedAt,
		"published_at":    book.PublishedAt,
		"updated_by":      book.UpdatedBy,
	}
}

func nullableWordCount(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
