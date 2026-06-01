package importjob

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MySQLStore implements Store against a MySQL database.
type MySQLStore struct {
	db *sql.DB
}

// NewMySQLStore creates a new MySQL-backed import job store.
func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) Create(ctx context.Context, job Job) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO catalog_import_jobs
			(job_id, admin_username, original_filename, format, temp_path,
			 content_sha1, file_size, status, stage, attempt_count, max_attempts, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.JobID, job.AdminUsername, job.OriginalFilename, job.Format, job.TempPath,
		job.ContentSHA1, job.FileSize, string(job.Status), string(job.Stage),
		job.AttemptCount, job.MaxAttempts, job.CreatedAt,
	)
	return err
}

func (s *MySQLStore) FindByID(ctx context.Context, jobID string) (*Job, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT job_id, admin_username, original_filename, format, temp_path,
				content_sha1, file_size, status, stage, progress_percent,
				attempt_count, max_attempts, book_key, error_code, error_message,
				internal_error, created_at, started_at, finished_at, updated_at
		 FROM catalog_import_jobs WHERE job_id = ?`, jobID)
	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (s *MySQLStore) List(ctx context.Context, status JobStatus, adminUsername string, limit, offset int) ([]Job, int, error) {
	if limit < 1 {
		limit = 20
	}
	where := "1=1"
	args := []any{}
	if status != "" {
		where += " AND status = ?"
		args = append(args, string(status))
	}
	if adminUsername != "" {
		where += " AND admin_username = ?"
		args = append(args, adminUsername)
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM catalog_import_jobs WHERE %s", where)
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT job_id, admin_username, original_filename, format, temp_path,
				content_sha1, file_size, status, stage, progress_percent,
				attempt_count, max_attempts, book_key, error_code, error_message,
				internal_error, created_at, started_at, finished_at, updated_at
		 FROM catalog_import_jobs WHERE %s ORDER BY created_at DESC LIMIT ? OFFSET ?`, where)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		job, err := scanJobRows(rows)
		if err != nil {
			return nil, 0, err
		}
		jobs = append(jobs, *job)
	}
	return jobs, total, rows.Err()
}

func (s *MySQLStore) ClaimNext(ctx context.Context) (*Job, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx,
		`SELECT job_id, admin_username, original_filename, format, temp_path,
				content_sha1, file_size, status, stage, progress_percent,
				attempt_count, max_attempts, book_key, error_code, error_message,
				internal_error, created_at, started_at, finished_at, updated_at
		 FROM catalog_import_jobs
		 WHERE status = 'queued'
		 ORDER BY created_at ASC
		 LIMIT 1
		 FOR UPDATE SKIP LOCKED`)
	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	now := time.Now()
	_, err = tx.ExecContext(ctx,
		`UPDATE catalog_import_jobs
		 SET status = 'processing', stage = 'hashing', started_at = ?, attempt_count = attempt_count + 1, updated_at = NOW()
		 WHERE job_id = ?`,
		now, job.JobID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	job.Status = StatusProcessing
	job.Stage = StageHashing
	job.StartedAt = &now
	job.AttemptCount++
	return job, nil
}

func (s *MySQLStore) UpdateStatus(ctx context.Context, jobID string, status JobStatus, stage Stage) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE catalog_import_jobs SET status = ?, stage = ?, updated_at = NOW() WHERE job_id = ?`,
		string(status), string(stage), jobID)
	return err
}

func (s *MySQLStore) UpdateForProcessing(ctx context.Context, jobID string, attemptCount int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE catalog_import_jobs
		 SET status = 'processing', stage = 'hashing', started_at = NOW(), attempt_count = ?, updated_at = NOW()
		 WHERE job_id = ?`,
		attemptCount, jobID)
	return err
}

func (s *MySQLStore) UpdateSuccess(ctx context.Context, jobID string, bookKey string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE catalog_import_jobs
		 SET status = 'succeeded', stage = 'finished', book_key = ?, finished_at = NOW(), updated_at = NOW()
		 WHERE job_id = ?`,
		bookKey, jobID)
	return err
}

func (s *MySQLStore) UpdateFailure(ctx context.Context, jobID string, errorCode, errorMsg, internalErr string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE catalog_import_jobs
		 SET status = 'failed', error_code = ?, error_message = ?, internal_error = ?, finished_at = NOW(), updated_at = NOW()
		 WHERE job_id = ?`,
		errorCode, errorMsg, internalErr, jobID)
	return err
}

func (s *MySQLStore) UpdateProgress(ctx context.Context, jobID string, stage Stage, progressPercent int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE catalog_import_jobs SET stage = ?, progress_percent = ?, updated_at = NOW() WHERE job_id = ?`,
		string(stage), progressPercent, jobID)
	return err
}

func (s *MySQLStore) ResetForRetry(ctx context.Context, jobID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE catalog_import_jobs
		 SET status = 'queued', stage = 'uploaded', error_code = NULL, error_message = NULL,
		     internal_error = NULL, started_at = NULL, finished_at = NULL, updated_at = NOW()
		 WHERE job_id = ? AND status = 'failed'`,
		jobID)
	return err
}

func (s *MySQLStore) Cancel(ctx context.Context, jobID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE catalog_import_jobs
		 SET status = 'canceled', finished_at = NOW(), updated_at = NOW()
		 WHERE job_id = ? AND status = 'queued'`,
		jobID)
	return err
}

// scanner is an interface satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanJob(s scanner) (*Job, error) {
	var j Job
	var status, stage string
	var progressPercent sql.NullInt64
	var bookKey, errorCode, errorMsg, internalErr sql.NullString
	var startedAt, finishedAt sql.NullTime

	err := s.Scan(
		&j.JobID, &j.AdminUsername, &j.OriginalFilename, &j.Format, &j.TempPath,
		&j.ContentSHA1, &j.FileSize, &status, &stage, &progressPercent,
		&j.AttemptCount, &j.MaxAttempts, &bookKey, &errorCode, &errorMsg,
		&internalErr, &j.CreatedAt, &startedAt, &finishedAt, &j.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	j.Status = JobStatus(status)
	j.Stage = Stage(stage)
	if progressPercent.Valid {
		v := int(progressPercent.Int64)
		j.ProgressPercent = &v
	}
	if bookKey.Valid {
		j.BookKey = &bookKey.String
	}
	if errorCode.Valid {
		j.ErrorCode = &errorCode.String
	}
	if errorMsg.Valid {
		j.ErrorMessage = &errorMsg.String
	}
	if internalErr.Valid {
		j.InternalError = &internalErr.String
	}
	if startedAt.Valid {
		j.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		j.FinishedAt = &finishedAt.Time
	}
	return &j, nil
}

func scanJobRows(rows *sql.Rows) (*Job, error) {
	return scanJob(rows)
}
