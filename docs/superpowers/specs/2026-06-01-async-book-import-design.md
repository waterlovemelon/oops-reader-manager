# Async Book Import Design

Date: 2026-06-01

## Context

The manager service currently accepts EPUB and TXT uploads through `POST /admin/catalog/books/upload`, saves the multipart file to a temporary path, parses it synchronously, moves the original file into catalog storage, and creates a `catalog_books` row with `status = draft`.

That works for small files, but it couples the HTTP request lifetime to parsing, filesystem writes, duplicate checks, and database writes. EPUB parsing and TXT chapter splitting can be slow or fail in ways that need better status reporting. Upload should become a short request that creates an import job, while the expensive import work runs in a background worker.

## Goals

- Keep upload requests fast and predictable.
- Track import progress and failure reasons.
- Support retrying failed imports without re-uploading the file when possible.
- Preserve current catalog ownership: manager service writes, reader backend reads.
- Keep uploaded books unpublished by default with `status = draft`.
- Keep the importer interface format-oriented so EPUB/TXT logic stays isolated.
- Make cleanup and audit behavior explicit.

## Non-Goals

- No distributed queue in the first version.
- No object storage migration in the first version.
- No automatic publishing after successful import.
- No reader-backend write endpoints.
- No full-text search/indexing pipeline unless added as a later job stage.

## Recommended Architecture

Use a database-backed import job queue inside the manager service.

```text
Manager Web
  uploads EPUB/TXT
  polls import job

Manager Service HTTP API
  validates request
  saves temp file
  computes hash
  creates catalog_import_jobs row

Import Worker
  claims queued jobs
  parses EPUB/TXT
  writes original/cover/derived files
  creates catalog_books draft row
  marks job succeeded or failed

Catalog Storage + MySQL
  source of truth for files, books, jobs, and audit
```

The first implementation can run one or more workers in the same service process. A future version can move workers into a separate process as long as both processes share the database and catalog storage.

## Reader Backend Compatibility Requirements

The import pipeline is only complete when a successfully imported and published book can be served by `oops-reader-backend` to the app. Manager-side success must therefore mean two things:

1. The manager created a valid draft book and import job succeeded.
2. After the admin publishes the book, the reader backend can list it, return book detail, return cover when available, return manifest, return chapter text, and return the original download.

Current backend behavior that affects this design:

- `oops-reader-backend/internal/catalog/mysql_store.go` reads `catalog_books` and currently filters published books with `status = 1`.
- Manager migrations change `catalog_books.status` to string values such as `draft`, `active`, `hidden`, and `deleted`.
- Backend catalog service currently resolves `storage_path` relative to its configured catalog root.
- Backend manifest/chapter/cover logic currently parses the original file through EPUB-only parsing.
- Backend list/detail JSON exposes `id`, `title`, `author`, `description`, `cover_url`, `download_url`, `language`, and `chapter_count` to the app.

Required backend alignment:

- Backend must treat `status = 'active'` as the only app-visible published state. If old numeric rows still exist during migration, temporarily support both `status = 'active'` and `status = 1`, then remove numeric compatibility after data migration.
- Backend must select and expose the new manager fields: `format`, `description`, and `cover_storage_path`.
- Backend must dispatch manifest/chapter/cover behavior by `format`, not assume every catalog book is EPUB.
- Backend and manager must use the same catalog root path in deployment. Manager writes relative `storage_path`; backend joins that relative path with its own catalog root.
- Backend must never expose `draft`, `hidden`, or `deleted` rows to the app.
- Backend should keep read-only access to catalog storage and database. It must not update import jobs or catalog book rows.

Minimum backend changes for app compatibility:

```text
oops-reader-backend/internal/catalog
  Book adds Format, Description, CoverStoragePath.
  MySQLStore filters WHERE status = 'active'.
  MySQLStore selects format, description, cover_storage_path.
  Service parse path dispatches by Book.Format.
  EPUB keeps current ParseEPUB path.
  TXT uses a TXT parser compatible with manager TXT chapter splitting.
  Cover first reads cover_storage_path when present; EPUB fallback can extract embedded cover.

oops-reader-backend/internal/transport/http/handlers/catalog.go
  bookJSON returns description from DB.
  cover_url can be omitted/null or return 404 for TXT books without cover.
  manifest/chapter endpoints work for both epub and txt.
```

If derived manifest/chapter files are produced by manager, backend should prefer derived files over reparsing originals. This makes EPUB and TXT behavior consistent and prevents the app from seeing different chapter IDs than the manager validated during import.

Recommended compatibility contract:

```text
catalog_books.book_key          -> app-visible book id
catalog_books.status = active   -> visible to app
catalog_books.format            -> reader backend parser dispatch key
catalog_books.storage_path      -> catalog-root-relative original file
catalog_books.cover_storage_path -> catalog-root-relative cover file, nullable
derived/<book_key>/manifest.json -> optional preferred reader manifest
derived/<book_key>/chapters/*.json -> optional preferred reader chapters
```

The safest implementation path is to introduce derived reader artifacts in the manager import worker and make backend read them first. If no derived artifacts exist, backend falls back to parsing the original file. That fallback keeps existing EPUB rows working while allowing TXT to be supported without reparsing rules drifting between repositories.

## Data Model

Add a `catalog_import_jobs` table.

Recommended fields:

- `job_id`: stable public ID, preferably UUID or ULID.
- `admin_username`: upload operator.
- `original_filename`: browser-provided filename for display only.
- `format`: `epub` or `txt`.
- `temp_path`: catalog-temp-root-relative path to the uploaded file.
- `content_sha1`: content hash used for duplicate detection and storage sharding.
- `file_size`: uploaded file size in bytes.
- `status`: `queued`, `processing`, `succeeded`, `failed`, `canceled`.
- `stage`: current stage for UI display.
- `progress_percent`: coarse progress, nullable.
- `attempt_count`: number of processing attempts.
- `max_attempts`: default `3`.
- `book_key`: created book key after success, nullable before success.
- `error_code`: stable machine-readable error code, nullable.
- `error_message`: short user-facing message, nullable.
- `internal_error`: longer diagnostic string for operators, nullable or restricted.
- `created_at`.
- `started_at`.
- `finished_at`.
- `updated_at`.

Recommended indexes:

- Unique index on `job_id`.
- Index on `(status, created_at)` for worker claiming.
- Index on `content_sha1` for debugging and duplicate job lookup.
- Index on `(admin_username, created_at)` for user-facing history.

## Job Status State Machine

Allowed transitions:

```text
queued -> processing
processing -> succeeded
processing -> failed
processing -> queued       when retryable failure and attempts remain
queued -> canceled
failed -> queued           manual retry
```

Terminal statuses:

- `succeeded`
- `failed`
- `canceled`

The worker must not process terminal jobs.

`stage` should be separate from `status` so the UI can show useful progress without inventing many statuses. Suggested stages:

- `uploaded`
- `hashing`
- `duplicate_check`
- `parsing_metadata`
- `extracting_cover`
- `splitting_chapters`
- `writing_storage`
- `creating_book`
- `recording_audit`
- `finished`

## HTTP API

### Upload

`POST /admin/catalog/books/import-jobs`

Request:

- Multipart form field `file`.
- Only `.epub` and `.txt` are accepted.

Response:

```json
{
  "data": {
    "job_id": "01HZ...",
    "status": "queued",
    "stage": "uploaded"
  }
}
```

Recommended status code: `202 Accepted`.

The upload handler should:

1. Validate admin authentication.
2. Validate extension and configured file size limit.
3. Create a unique temp filename. Do not use the original filename as the temp filename.
4. Save the uploaded file under `catalog.temp_root`.
5. Compute file size and SHA1.
6. Detect an already imported duplicate by `catalog_books.content_sha1`.
7. Create a `catalog_import_jobs` row with `status = queued`.
8. Record an audit entry like `book_import_requested`.
9. Return the job ID.

If the content hash already exists in `catalog_books`, return `409 Conflict` by default and do not create a new job. If product behavior later needs re-import, make that an explicit override.

### Get Job

`GET /admin/catalog/import-jobs/:job_id`

Response:

```json
{
  "data": {
    "job_id": "01HZ...",
    "status": "processing",
    "stage": "splitting_chapters",
    "progress_percent": 45,
    "book_key": null,
    "error_code": null,
    "error_message": null,
    "created_at": "2026-06-01T10:00:00Z",
    "started_at": "2026-06-01T10:00:03Z",
    "finished_at": null
  }
}
```

When `status = succeeded`, `book_key` must be present so the frontend can navigate to the draft book detail page.

### List Jobs

`GET /admin/catalog/import-jobs?status=&page=&page_size=`

This is useful for an import history page and for recovering when the user closes the browser after uploading.

### Retry Job

`POST /admin/catalog/import-jobs/:job_id/retry`

Allowed only for `failed` jobs whose temp file still exists. It resets:

- `status = queued`
- `stage = uploaded`
- `error_code = null`
- `error_message = null`
- `internal_error = null`

It should increment attempt state when the worker actually claims the job, not when retry is requested.

### Cancel Job

`POST /admin/catalog/import-jobs/:job_id/cancel`

Allowed for `queued` jobs. Canceling a `processing` job can be added later with context cancellation, but the first version can reject it with `409 Conflict`.

## Worker Design

The worker loop should:

1. Claim one queued job atomically.
2. Set `status = processing`, `started_at`, `stage`, and increment `attempt_count`.
3. Run import stages with context timeout.
4. On success, set `status = succeeded`, `stage = finished`, `book_key`, and `finished_at`.
5. On failure, classify the error.
6. If retryable and attempts remain, return the job to `queued`.
7. Otherwise set `status = failed`, `error_code`, `error_message`, `internal_error`, and `finished_at`.

Job claiming should use a transaction and row lock. In MySQL 8, `SELECT ... FOR UPDATE SKIP LOCKED` is ideal. If the deployed database does not support `SKIP LOCKED`, use an atomic `UPDATE ... WHERE status = 'queued' ORDER BY created_at LIMIT 1` pattern with a worker ID column or claim token.

Recommended worker configuration:

```yaml
catalog:
  import_worker:
    enabled: true
    concurrency: 2
    poll_interval: 2s
    job_timeout: 5m
    max_attempts: 3
```

## Import Processing Steps

### 1. Validate Temp File

- Ensure `temp_path` is relative to `catalog.temp_root`.
- Ensure the file still exists.
- Recompute hash if needed and compare with `content_sha1`.
- Fail with `temp_file_missing` or `content_hash_mismatch` if validation fails.

### 2. Duplicate Check

- Check `catalog_books.content_sha1`.
- If found, fail with `duplicate_book`.
- Also guard with a unique index on `catalog_books.content_sha1` where possible, because two jobs can race.

### 3. Parse Content

EPUB importer should:

- Open as ZIP without extracting entries to arbitrary filesystem paths.
- Reject absolute paths and `..` paths inside the archive.
- Parse `META-INF/container.xml`.
- Parse OPF metadata.
- Use spine/nav/ncx order for chapters.
- Extract readable text from XHTML/HTML.
- Extract cover if present and within configured size.
- Fail with `invalid_epub` or `empty_content` when no usable content exists.

TXT importer should:

- Detect UTF-8, UTF-8 BOM, and GB18030/GBK if supported.
- Normalize line endings.
- Split by common Chinese and English chapter headings.
- Fall back to a single chapter or configured size-based chunks.
- Fail with `invalid_text_encoding` or `empty_content` when content cannot be decoded or is blank.

### 4. Generate Book Key

Use the existing stable key strategy:

```text
slug(title or filename) + "-" + sha1[:10]
```

If the title is missing, fall back to the original filename without extension.

### 5. Write Storage

Move the original file to:

```text
originals/<format>/<sha1[0:2]>/<sha1[2:4]>/<book_key>.<ext>
```

Write cover when present:

```text
covers/<sha1[0:2]>/<sha1[2:4]>/<book_key>.<ext>
```

Optional first-version derived output:

```text
derived/<book_key>/manifest.json
derived/<book_key>/chapters/<chapter_id>.json
```

If derived output is not implemented in the first pass, keep the manifest/chapter APIs backed by the importer and original file.

### 6. Create Book Row

Create `catalog_books` with:

- `status = draft`
- `source = admin_upload`
- `format`
- `filename`
- `storage_path`
- `cover_storage_path`
- `file_size`
- `content_sha1`
- `language`
- `chapter_count`
- `uploaded_at`
- `updated_by`

The database write and job success update should be in one transaction when practical. File writes cannot be transactional, so failures after storage movement must perform best-effort cleanup.

Do not mark a book `active` from the import worker. The app should only see a book after an admin review and publish action. The publish action must update `catalog_books.status = 'active'` and set `published_at`.

Before marking the job succeeded, the worker should verify the reader-facing contract for the draft row:

- `book_key` is non-empty and unique.
- `format` is one of the backend-supported formats.
- `storage_path` is relative and points to an existing file under catalog root.
- `chapter_count > 0`.
- Manifest generation for backend consumption succeeds.
- Every chapter ID in the manifest can be loaded.
- `cover_storage_path`, when set, points to an existing file under catalog root.

These checks keep "import succeeded" from meaning only "database insert succeeded"; it must mean the later publish step will make a readable app catalog item.

### 7. Audit

Record:

- `book_import_requested` when the upload job is created.
- `book_import_succeeded` after the draft book is created.
- `book_import_failed` after terminal failure.
- `book_import_retried` for manual retries.
- `book_import_canceled` for cancellations.

Audit entries should include `job_id`, `book_key` when available, admin username, IP, User-Agent, and stable error code for failures.

## Error Codes

Use stable error codes so the frontend can map messages cleanly.

- `unsupported_format`
- `file_too_large`
- `upload_save_failed`
- `temp_file_missing`
- `content_hash_mismatch`
- `duplicate_book`
- `invalid_epub`
- `invalid_text_encoding`
- `empty_content`
- `cover_too_large`
- `storage_error`
- `database_error`
- `job_timeout`
- `internal_error`

User-facing messages should be short. Full wrapped errors should go to structured logs and `internal_error` only if the admin API is allowed to expose diagnostics.

## Cleanup Policy

Temporary files:

- Delete after successful import.
- Delete after terminal non-retryable failure unless diagnostics require retention.
- Keep after retryable failure while attempts remain.
- Periodically delete orphaned temp files older than a configured TTL.

Final files:

- If database create fails after moving the original, remove the moved original and cover.
- If job success is recorded, do not remove final files except through book delete/archive workflows.

Recommended cleanup configuration:

```yaml
catalog:
  import_cleanup:
    temp_ttl: 24h
    failed_temp_ttl: 72h
```

## Frontend Behavior

The book page should:

1. Upload file to `POST /admin/catalog/books/import-jobs`.
2. Show a row/card with job status immediately.
3. Poll `GET /admin/catalog/import-jobs/:job_id` every 1-2 seconds while the job is non-terminal.
4. Navigate to the draft book detail page when `status = succeeded`.
5. Show a concise failure message and retry action when `status = failed` and retry is allowed.
6. Keep an import history list so closing the browser does not lose visibility.

The upload control should reject unsupported extensions before sending the request, but the backend remains the source of truth.

## Idempotency and Concurrency

- Uploading the same already-imported content returns `409 Conflict`.
- Two queued jobs with the same hash can exist only if duplicate detection races or is intentionally relaxed. The worker must still check duplicates before creating a book.
- `catalog_books.content_sha1` should be unique if the product wants strict deduplication.
- Worker claiming must be atomic so two workers cannot process the same job.
- Retry must not create a second book if the first attempt actually succeeded but failed to update the job record. On retry, check for `catalog_books.content_sha1` first and mark the job succeeded if the existing book was created by the same job, otherwise fail as duplicate.

## Observability

Structured logs should include:

- `job_id`
- `book_key`
- `format`
- `content_sha1`
- `stage`
- `attempt_count`
- `duration_ms`
- `error_code`

Metrics to add later:

- Import jobs created.
- Import jobs succeeded/failed.
- Import duration by format.
- Failure count by error code.
- Queue depth.
- Oldest queued job age.

## Migration Plan

1. Add `catalog_import_jobs` migration and store interface.
2. Add upload endpoint that creates jobs instead of importing synchronously.
3. Keep the current synchronous endpoint temporarily or route it through the new job path.
4. Add worker service with one goroutine and database-backed claiming.
5. Move current `ImportUploadedFile` internals into an import processor used by the worker.
6. Add job polling endpoints.
7. Update frontend upload flow to use jobs.
8. Add cleanup task for old temp files.
9. Remove or deprecate the old synchronous upload endpoint after frontend migration.

Reader backend migration must be part of the same release train:

1. Apply the shared `catalog_books` schema migration to both repositories' migration sets.
2. Update backend catalog MySQL queries from numeric status filtering to string status filtering.
3. Add backend `format` dispatch for EPUB and TXT.
4. Add backend support for `cover_storage_path` and derived manifest/chapter files.
5. Run an end-to-end smoke test: manager upload -> import succeeded -> publish -> app/backend list -> manifest -> first chapter -> cover/download.
6. Deploy backend changes before or at the same time as manager publishing support. Do not publish TXT books until backend TXT support is live.

## Testing Plan

Service tests:

- Upload creates queued job and does not create book immediately.
- Unsupported extension returns `400`.
- Duplicate already-imported hash returns `409`.
- Worker claims only one job under concurrent workers.
- Successful EPUB job creates draft book and marks job succeeded.
- Successful TXT job creates draft book and marks job succeeded.
- Parser failure marks job failed with stable error code.
- Retry failed job resets status and later succeeds.
- Storage failure cleans up partial files.
- Database failure after storage move cleans up final files.
- Import success validates reader-facing manifest/chapter availability before marking job succeeded.

Importer tests:

- EPUB metadata extraction.
- EPUB chapter order from spine/nav.
- EPUB with missing cover still imports.
- Invalid EPUB fails cleanly.
- TXT UTF-8 import.
- TXT Chinese chapter splitting.
- TXT no headings fallback.
- TXT invalid encoding failure if unsupported bytes cannot be decoded.

HTTP tests:

- Auth required for upload and job queries.
- `POST /admin/catalog/books/import-jobs` returns `202`.
- `GET /admin/catalog/import-jobs/:job_id` returns status and book key after success.
- Retry and cancel enforce allowed state transitions.

Cross-repository compatibility tests:

- Manager-created active EPUB row is visible through backend list API.
- Manager-created draft EPUB row is not visible through backend list API.
- Manager-created hidden/deleted rows are not visible through backend list API.
- Backend resolves manager relative `storage_path` under the configured catalog root.
- Backend returns manifest and chapter text for a manager-imported EPUB.
- Backend returns manifest and chapter text for a manager-imported TXT.
- Backend serves `cover_storage_path` when present.
- Backend returns a controlled 404 for cover when no cover exists.
- Backend download endpoint returns the original manager-stored file.

Frontend tests:

- Upload starts polling.
- Successful job shows draft navigation.
- Failed job shows error and retry action.
- Import history renders terminal and active jobs.

## Open Decisions

- Whether to keep `POST /admin/catalog/books/upload` as a compatibility wrapper or replace it outright.
- Whether first implementation should persist derived chapter files or continue parsing original files on demand. For backend/app compatibility, derived files are recommended because they make manager validation and backend serving use the same manifest and chapter IDs.
- Whether strict deduplication should use SHA1 only or introduce SHA256 while keeping existing SHA1 fields.
- Whether failed temp files should be retained by default in development only or also in production for diagnostics.
