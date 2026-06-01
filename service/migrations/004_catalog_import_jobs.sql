-- 004_catalog_import_jobs.sql
-- Async book import job queue for the manager service.

CREATE TABLE IF NOT EXISTS catalog_import_jobs (
    job_id          VARCHAR(36)   NOT NULL,
    admin_username  VARCHAR(100)  NOT NULL,
    original_filename VARCHAR(500) NOT NULL,
    format          VARCHAR(20)   NOT NULL,
    temp_path       VARCHAR(1000) NOT NULL,
    content_sha1    VARCHAR(40)   NOT NULL,
    file_size       BIGINT        NOT NULL DEFAULT 0,
    status          VARCHAR(20)   NOT NULL DEFAULT 'queued',
    stage           VARCHAR(50)   NOT NULL DEFAULT 'uploaded',
    progress_percent TINYINT      NULL,
    attempt_count   INT           NOT NULL DEFAULT 0,
    max_attempts    INT           NOT NULL DEFAULT 3,
    book_key        VARCHAR(200)  NULL,
    error_code      VARCHAR(50)   NULL,
    error_message   VARCHAR(500)  NULL,
    internal_error  TEXT          NULL,
    created_at      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at      DATETIME      NULL,
    finished_at     DATETIME      NULL,
    updated_at      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (job_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX idx_import_jobs_status_created ON catalog_import_jobs (status, created_at);
CREATE INDEX idx_import_jobs_sha1 ON catalog_import_jobs (content_sha1);
CREATE INDEX idx_import_jobs_admin_created ON catalog_import_jobs (admin_username, created_at);
