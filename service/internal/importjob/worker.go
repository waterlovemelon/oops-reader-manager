package importjob

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/oops-reader/oops-reader-manager/service/internal/audit"
	"github.com/oops-reader/oops-reader-manager/service/internal/catalog"
	"go.uber.org/zap"
)

// WorkerConfig configures the import worker.
type WorkerConfig struct {
	Enabled      bool
	Concurrency  int
	PollInterval time.Duration
	JobTimeout   time.Duration
	MaxAttempts  int
}

// DefaultWorkerConfig returns sensible defaults.
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		Enabled:      true,
		Concurrency:  2,
		PollInterval: 2 * time.Second,
		JobTimeout:   5 * time.Minute,
		MaxAttempts:  3,
	}
}

// Worker processes import jobs in the background.
type Worker struct {
	cfg       WorkerConfig
	store     Store
	importSvc *catalog.Service
	storage   *catalog.LocalStorage
	auditSvc  *audit.Service
	logger    *zap.Logger
}

// NewWorker creates a new import worker.
func NewWorker(cfg WorkerConfig, store Store, importSvc *catalog.Service, storage *catalog.LocalStorage, auditSvc *audit.Service, logger *zap.Logger) *Worker {
	return &Worker{
		cfg:       cfg,
		store:     store,
		importSvc: importSvc,
		storage:   storage,
		auditSvc:  auditSvc,
		logger:    logger,
	}
}

// Start launches the worker goroutines. It blocks until the context is canceled.
func (w *Worker) Start(ctx context.Context) {
	if !w.cfg.Enabled {
		w.logger.Info("import worker disabled")
		return
	}

	concurrency := w.cfg.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}

	w.logger.Info("starting import worker", zap.Int("concurrency", concurrency), zap.Duration("poll_interval", w.cfg.PollInterval))

	for i := 0; i < concurrency; i++ {
		go w.loop(ctx, i)
	}
}

func (w *Worker) loop(ctx context.Context, workerID int) {
	logger := w.logger.With(zap.Int("worker_id", workerID))
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("import worker stopping")
			return
		case <-ticker.C:
			job, err := w.store.ClaimNext(ctx)
			if err != nil {
				logger.Error("failed to claim job", zap.Error(err))
				continue
			}
			if job == nil {
				continue
			}
			logger.Info("claimed job", zap.String("job_id", job.JobID), zap.String("format", job.Format))
			w.processJob(ctx, logger, job)
		}
	}
}

func (w *Worker) processJob(ctx context.Context, logger *zap.Logger, job *Job) {
	jobCtx, cancel := context.WithTimeout(ctx, w.cfg.JobTimeout)
	defer cancel()

	// Build the full temp path.
	tempPath := filepath.Join(w.storage.TempRoot(), job.TempPath)

	// Track start time for duration logging.
	start := time.Now()

	// Use the existing catalog service to do the actual import.
	book, err := w.importSvc.ImportUploadedFile(jobCtx, catalog.UploadInput{
		AdminUsername:    job.AdminUsername,
		OriginalFilename: job.OriginalFilename,
		TempPath:         tempPath,
	})

	duration := time.Since(start)

	if err != nil {
		w.handleFailure(jobCtx, logger, job, err, duration)
		return
	}

	// Success: update job with the created book key.
	if err := w.store.UpdateSuccess(jobCtx, job.JobID, book.BookKey); err != nil {
		logger.Error("failed to mark job succeeded", zap.String("job_id", job.JobID), zap.Error(err))
		return
	}

	// Audit.
	_ = w.auditSvc.Record(jobCtx, audit.Entry{
		AdminUsername: job.AdminUsername,
		Action:        "book_import_succeeded",
		ResourceType:  "import_job",
		ResourceID:    job.JobID,
	})

	logger.Info("import job succeeded",
		zap.String("job_id", job.JobID),
		zap.String("book_key", book.BookKey),
		zap.Duration("duration", duration),
	)
}

func (w *Worker) handleFailure(ctx context.Context, logger *zap.Logger, job *Job, err error, duration time.Duration) {
	errCode, errMsg, internalErr := classifyError(err)

	logger.Warn("import job failed",
		zap.String("job_id", job.JobID),
		zap.String("error_code", errCode),
		zap.String("error_message", errMsg),
		zap.String("internal_error", internalErr),
		zap.Int("attempt", job.AttemptCount),
		zap.Duration("duration", duration),
	)

	// Check if we should retry.
	if isRetryable(errCode) && job.AttemptCount < job.MaxAttempts {
		logger.Info("retrying job", zap.String("job_id", job.JobID), zap.Int("next_attempt", job.AttemptCount+1))
		if retryErr := w.store.UpdateStatus(ctx, job.JobID, StatusQueued, StageUploaded); retryErr != nil {
			logger.Error("failed to reset job for retry", zap.String("job_id", job.JobID), zap.Error(retryErr))
		}
		return
	}

	// Terminal failure.
	if failErr := w.store.UpdateFailure(ctx, job.JobID, errCode, errMsg, internalErr); failErr != nil {
		logger.Error("failed to mark job failed", zap.String("job_id", job.JobID), zap.Error(failErr))
		return
	}

	// Audit.
	_ = w.auditSvc.Record(ctx, audit.Entry{
		AdminUsername: job.AdminUsername,
		Action:        "book_import_failed",
		ResourceType:  "import_job",
		ResourceID:    job.JobID,
	})

	// Clean up temp file on terminal failure.
	_ = os.Remove(tempPathForJob(job))
}

// classifyError maps an error to a stable error code, user-facing message, and internal diagnostic.
func classifyError(err error) (code, msg, internal string) {
	errStr := err.Error()

	switch {
	case errors.Is(err, catalog.ErrUnsupportedFormat):
		return ErrCodeUnsupportedFormat, "不支持的文件格式", errStr
	case errors.Is(err, catalog.ErrDuplicateBook):
		return ErrCodeDuplicateBook, "书籍已存在（重复上传）", errStr
	case isTempFileMissing(err):
		return ErrCodeTempFileMissing, "临时文件丢失，请重新上传", errStr
	case isStorageError(err):
		return ErrCodeStorageError, "存储写入失败", errStr
	default:
		return ErrCodeInternalError, "导入失败，请稍后重试", errStr
	}
}

func isRetryable(errCode string) bool {
	switch errCode {
	case ErrCodeStorageError, ErrCodeDatabaseError, ErrCodeJobTimeout:
		return true
	default:
		return false
	}
}

func isTempFileMissing(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

func isStorageError(err error) bool {
	// Check for common filesystem errors.
	return errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrExist)
}

func tempPathForJob(job *Job) string {
	// This is a best-effort path; the actual path depends on the temp root.
	return job.TempPath
}
