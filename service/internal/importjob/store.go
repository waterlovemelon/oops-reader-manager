package importjob

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a job is not found.
var ErrNotFound = errors.New("import job not found")

// Store defines the persistence interface for import jobs.
type Store interface {
	// Create inserts a new import job.
	Create(ctx context.Context, job Job) error

	// FindByID retrieves a job by its ID.
	FindByID(ctx context.Context, jobID string) (*Job, error)

	// List retrieves jobs with optional status and admin filters, with pagination.
	// Pass empty string for status/adminUsername to skip those filters.
	List(ctx context.Context, status JobStatus, adminUsername string, limit, offset int) ([]Job, int, error)

	// ClaimNext atomically claims the next queued job for processing.
	// Uses SELECT ... FOR UPDATE SKIP LOCKED to prevent double-claiming.
	// Returns nil if no queued jobs are available.
	ClaimNext(ctx context.Context) (*Job, error)

	// UpdateStatus updates the job status and stage.
	UpdateStatus(ctx context.Context, jobID string, status JobStatus, stage Stage) error

	// UpdateForProcessing marks a job as processing with start time and attempt count.
	UpdateForProcessing(ctx context.Context, jobID string, attemptCount int) error

	// UpdateSuccess marks a job as succeeded with the created book key.
	UpdateSuccess(ctx context.Context, jobID string, bookKey string) error

	// UpdateFailure marks a job as failed with error details.
	UpdateFailure(ctx context.Context, jobID string, errorCode, errorMsg, internalErr string) error

	// UpdateProgress updates the stage and progress percentage.
	UpdateProgress(ctx context.Context, jobID string, stage Stage, progressPercent int) error

	// ResetForRetry resets a failed job back to queued state for retry.
	ResetForRetry(ctx context.Context, jobID string) error

	// Cancel cancels a queued job.
	Cancel(ctx context.Context, jobID string) error
}
