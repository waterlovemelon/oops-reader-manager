package importjob

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oops-reader/oops-reader-manager/service/internal/catalog"
)

// Sentinel errors for import job operations.
var (
	ErrUnsupportedFormat = errors.New("unsupported book format")
	ErrDuplicateBook     = errors.New("duplicate catalog book")
	ErrUploadSaveFailed  = errors.New("upload save failed")
	ErrDatabaseError     = errors.New("database error")
)

// Service provides business logic for import job management.
type Service struct {
	store     Store
	bookStore catalog.Store
	storage   *catalog.LocalStorage
}

// NewService creates a new import job service.
func NewService(store Store, bookStore catalog.Store, storage *catalog.LocalStorage) *Service {
	return &Service{
		store:     store,
		bookStore: bookStore,
		storage:   storage,
	}
}

// CreateJobInput contains the parameters for creating an import job.
type CreateJobInput struct {
	AdminUsername    string
	OriginalFilename string
	TempPath         string
}

// CreateJob validates the upload, checks for duplicates, and creates a queued import job.
func (s *Service) CreateJob(ctx context.Context, input CreateJobInput) (*Job, error) {
	// Detect format from extension.
	ext := strings.ToLower(filepath.Ext(input.OriginalFilename))
	format := strings.TrimPrefix(ext, ".")
	if format != "epub" && format != "txt" {
		return nil, fmt.Errorf("%w: .%s", ErrUnsupportedFormat, format)
	}

	// Compute SHA1 and file size.
	sha, fileSize, err := fileSHA1(input.TempPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUploadSaveFailed, err)
	}

	// Check for duplicate in existing books.
	existing, err := s.bookStore.FindBySHA1(ctx, sha)
	if err != nil && !errors.Is(err, catalog.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, ErrDuplicateBook
	}

	job := Job{
		JobID:            generateJobID(),
		AdminUsername:     input.AdminUsername,
		OriginalFilename: input.OriginalFilename,
		Format:           format,
		TempPath:         filepath.Base(input.TempPath), // store relative to temp root
		ContentSHA1:      sha,
		FileSize:         fileSize,
		Status:           StatusQueued,
		Stage:            StageUploaded,
		AttemptCount:     0,
		MaxAttempts:      3,
		CreatedAt:        time.Now(),
	}

	if err := s.store.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}
	return &job, nil
}

// GetJob retrieves a job by ID.
func (s *Service) GetJob(ctx context.Context, jobID string) (*Job, error) {
	return s.store.FindByID(ctx, jobID)
}

// ListJobs retrieves jobs with optional filters.
func (s *Service) ListJobs(ctx context.Context, status JobStatus, adminUsername string, page, pageSize int) ([]Job, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.store.List(ctx, status, adminUsername, pageSize, offset)
}

// RetryJob resets a failed job back to queued state.
func (s *Service) RetryJob(ctx context.Context, jobID string) error {
	job, err := s.store.FindByID(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status != StatusFailed {
		return fmt.Errorf("cannot retry job with status %s", job.Status)
	}
	return s.store.ResetForRetry(ctx, jobID)
}

// CancelJob cancels a queued job.
func (s *Service) CancelJob(ctx context.Context, jobID string) error {
	job, err := s.store.FindByID(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status != StatusQueued {
		return fmt.Errorf("cannot cancel job with status %s", job.Status)
	}
	return s.store.Cancel(ctx, jobID)
}

// Store exposes the underlying store for the worker.
func (s *Service) Store() Store {
	return s.store
}

// fileSHA1 computes the SHA1 hash and size of a file.
func fileSHA1(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	h := sha1.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

// GenerateJobID creates a unique job ID. Exported for use by other packages.
func GenerateJobID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		time.Now().UnixNano()&0xffffffff,
		b[0:2], b[2:4], b[4:6], b[6:12])
}

// generateJobID is an alias for internal use.
var generateJobID = GenerateJobID
