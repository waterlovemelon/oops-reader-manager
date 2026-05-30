package httpapi

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/catalog"
)

type CatalogHandler struct {
	service  *catalog.Service
	tempRoot string
}

func NewCatalogHandler(service *catalog.Service, tempRoot string) *CatalogHandler {
	return &CatalogHandler{service: service, tempRoot: tempRoot}
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

func bookJSON(book catalog.Book) gin.H {
	return gin.H{
		"id":            book.BookKey,
		"title":         book.Title,
		"author":        book.Author,
		"format":        book.Format,
		"filename":      book.Filename,
		"storage_path":  book.StoragePath,
		"file_size":     book.FileSize,
		"content_sha1":  book.ContentSHA1,
		"chapter_count": book.ChapterCount,
		"status":        book.Status,
	}
}
