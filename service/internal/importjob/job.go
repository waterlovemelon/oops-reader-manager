package importjob

import "time"

// JobStatus represents the lifecycle state of an import job.
type JobStatus string

const (
	StatusQueued     JobStatus = "queued"
	StatusProcessing JobStatus = "processing"
	StatusSucceeded  JobStatus = "succeeded"
	StatusFailed     JobStatus = "failed"
	StatusCanceled   JobStatus = "canceled"
)

// Stage describes the current processing stage for UI display.
type Stage string

const (
	StageUploaded         Stage = "uploaded"
	StageHashing          Stage = "hashing"
	StageDuplicateCheck   Stage = "duplicate_check"
	StageParsingMetadata  Stage = "parsing_metadata"
	StageExtractingCover  Stage = "extracting_cover"
	StageSplittingChapters Stage = "splitting_chapters"
	StageWritingStorage   Stage = "writing_storage"
	StageCreatingBook     Stage = "creating_book"
	StageRecordingAudit   Stage = "recording_audit"
	StageFinished         Stage = "finished"
)

// Error codes for stable machine-readable error classification.
const (
	ErrCodeUnsupportedFormat  = "unsupported_format"
	ErrCodeFileTooLarge       = "file_too_large"
	ErrCodeUploadSaveFailed   = "upload_save_failed"
	ErrCodeTempFileMissing    = "temp_file_missing"
	ErrCodeContentHashMismatch = "content_hash_mismatch"
	ErrCodeDuplicateBook      = "duplicate_book"
	ErrCodeInvalidEpub        = "invalid_epub"
	ErrCodeInvalidTextEncoding = "invalid_text_encoding"
	ErrCodeEmptyContent       = "empty_content"
	ErrCodeCoverTooLarge      = "cover_too_large"
	ErrCodeStorageError       = "storage_error"
	ErrCodeDatabaseError      = "database_error"
	ErrCodeJobTimeout         = "job_timeout"
	ErrCodeInternalError      = "internal_error"
)

// Job represents an import job in the queue.
type Job struct {
	JobID            string
	AdminUsername     string
	OriginalFilename string
	Format           string
	TempPath         string
	ContentSHA1      string
	FileSize         int64
	Status           JobStatus
	Stage            Stage
	ProgressPercent  *int
	AttemptCount     int
	MaxAttempts      int
	BookKey          *string
	ErrorCode        *string
	ErrorMessage     *string
	InternalError    *string
	CreatedAt        time.Time
	StartedAt        *time.Time
	FinishedAt       *time.Time
	UpdatedAt        time.Time
}
