package httpapi

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/audit"
	"github.com/oops-reader/oops-reader-manager/service/internal/importjob"
)

// ImportHandler handles HTTP requests for import jobs.
type ImportHandler struct {
	service  *importjob.Service
	tempRoot string
	auditSvc *audit.Service
}

// NewImportHandler creates a new import handler.
func NewImportHandler(service *importjob.Service, tempRoot string, auditSvc *audit.Service) *ImportHandler {
	return &ImportHandler{service: service, tempRoot: tempRoot, auditSvc: auditSvc}
}

// CreateJob handles POST /admin/catalog/import-jobs.
// It accepts a multipart file upload, saves it to a temp location,
// and creates a queued import job.
func (h *ImportHandler) CreateJob(c *gin.Context) {
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

	// Validate extension.
	ext := filepath.Ext(file.Filename)
	if ext != ".epub" && ext != ".txt" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported format, only .epub and .txt are accepted"})
		return
	}

	// Create temp directory if needed.
	if err := os.MkdirAll(h.tempRoot, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare temp directory"})
		return
	}

	// Save to temp file with unique name.
	tempPath := filepath.Join(h.tempRoot, generateTempFilename(file.Filename))
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save uploaded file"})
		return
	}

	// Create the import job.
	job, err := h.service.CreateJob(c.Request.Context(), importjob.CreateJobInput{
		AdminUsername:    claims.Username,
		OriginalFilename: file.Filename,
		TempPath:         tempPath,
	})
	if err != nil {
		// Clean up temp file on error.
		_ = os.Remove(tempPath)

		if errors.Is(err, importjob.ErrUnsupportedFormat) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, importjob.ErrDuplicateBook) {
			c.JSON(http.StatusConflict, gin.H{"error": "book with this content already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create import job"})
		return
	}

	// Audit.
	_ = h.auditSvc.Record(c.Request.Context(), audit.Entry{
		AdminUsername: claims.Username,
		Action:        "book_import_requested",
		ResourceType:  "import_job",
		ResourceID:    job.JobID,
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})

	c.JSON(http.StatusAccepted, gin.H{
		"data": gin.H{
			"job_id": job.JobID,
			"status": string(job.Status),
			"stage":  string(job.Stage),
		},
	})
}

// GetJob handles GET /admin/catalog/import-jobs/:job_id.
func (h *ImportHandler) GetJob(c *gin.Context) {
	jobID := c.Param("job_id")
	job, err := h.service.GetJob(c.Request.Context(), jobID)
	if err != nil {
		if errors.Is(err, importjob.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "import job not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get import job"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": jobJSON(job)})
}

// ListJobs handles GET /admin/catalog/import-jobs.
func (h *ImportHandler) ListJobs(c *gin.Context) {
	page, pageSize := pageParams(c)
	status := importjob.JobStatus(c.Query("status"))
	adminUsername := c.Query("admin_username")

	jobs, total, err := h.service.ListJobs(c.Request.Context(), status, adminUsername, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list import jobs"})
		return
	}

	result := make([]gin.H, len(jobs))
	for i, j := range jobs {
		result[i] = jobJSON(&j)
	}

	c.JSON(http.StatusOK, gin.H{
		"data": result,
		"pagination": gin.H{
			"page":      page,
			"page_size": pageSize,
			"total":     total,
		},
	})
}

// RetryJob handles POST /admin/catalog/import-jobs/:job_id/retry.
func (h *ImportHandler) RetryJob(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}

	jobID := c.Param("job_id")
	if err := h.service.RetryJob(c.Request.Context(), jobID); err != nil {
		if errors.Is(err, importjob.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "import job not found"})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	_ = h.auditSvc.Record(c.Request.Context(), audit.Entry{
		AdminUsername: claims.Username,
		Action:        "book_import_retried",
		ResourceType:  "import_job",
		ResourceID:    jobID,
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"job_id": jobID, "status": "queued"}})
}

// CancelJob handles POST /admin/catalog/import-jobs/:job_id/cancel.
func (h *ImportHandler) CancelJob(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}

	jobID := c.Param("job_id")
	if err := h.service.CancelJob(c.Request.Context(), jobID); err != nil {
		if errors.Is(err, importjob.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "import job not found"})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	_ = h.auditSvc.Record(c.Request.Context(), audit.Entry{
		AdminUsername: claims.Username,
		Action:        "book_import_canceled",
		ResourceType:  "import_job",
		ResourceID:    jobID,
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"job_id": jobID, "status": "canceled"}})
}

// jobJSON converts a Job to a JSON-serializable map.
func jobJSON(job *importjob.Job) gin.H {
	result := gin.H{
		"job_id":            job.JobID,
		"admin_username":    job.AdminUsername,
		"original_filename": job.OriginalFilename,
		"format":            job.Format,
		"content_sha1":      job.ContentSHA1,
		"file_size":         job.FileSize,
		"status":            string(job.Status),
		"stage":             string(job.Stage),
		"attempt_count":     job.AttemptCount,
		"max_attempts":      job.MaxAttempts,
		"created_at":        job.CreatedAt,
		"updated_at":        job.UpdatedAt,
	}
	if job.ProgressPercent != nil {
		result["progress_percent"] = *job.ProgressPercent
	}
	if job.BookKey != nil {
		result["book_key"] = *job.BookKey
	}
	if job.ErrorCode != nil {
		result["error_code"] = *job.ErrorCode
	}
	if job.ErrorMessage != nil {
		result["error_message"] = *job.ErrorMessage
	}
	if job.StartedAt != nil {
		result["started_at"] = *job.StartedAt
	}
	if job.FinishedAt != nil {
		result["finished_at"] = *job.FinishedAt
	}
	return result
}

// generateTempFilename creates a unique temp filename to avoid collisions.
func generateTempFilename(original string) string {
	ext := filepath.Ext(original)
	return "upload-" + generateJobID() + ext
}

// generateJobID is a simple ID generator for temp filenames.
func generateJobID() string {
	return importjob.GenerateJobID()
}
