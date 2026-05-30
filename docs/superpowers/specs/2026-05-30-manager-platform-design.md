# Oops Reader Manager Platform Design

Date: 2026-05-30

## Context

Oops Reader currently has three related repositories:

- `oops-reader-app`: Flutter reader app for end users.
- `oops-reader-backend`: Go reader backend that serves account, community, backup, TTS, and online catalog read APIs.
- `oops-reader-manager`: empty repository reserved for the management platform.

The management platform will manage users, community posts/comments, and online catalog books. The important architectural decision is that management writes should not live in the reader backend. The manager platform owns administrative writes. The reader backend remains a stable read service for the app.

## Goals

- Build an independent manager frontend for administrator workflows.
- Build an independent manager service for administrative APIs.
- Support a single administrator login in the first version.
- Provide production-usable management workflows, not only API demos.
- Support online book upload for EPUB and TXT in the first version.
- Keep the storage and database contract compatible with the reader backend.
- Make future book formats possible through importer extensions.
- Keep reader backend APIs focused on read-only catalog access for the reader app.

## Non-Goals

- No multi-admin RBAC in the first version.
- No object storage in the first version.
- No direct upload or administrative write endpoints in `oops-reader-backend`.
- No rewrite of the reader app.
- No support for every book format in the first version.

## Architecture

The platform is split into independent frontend and backend services:

```text
oops-reader-manager-web
  React + TypeScript + Vite + Ant Design
  Administrator UI only.

oops-reader-manager-service
  Go + Gin
  Administrator API, upload, validation, import, persistence, audit.

oops-reader-backend
  Go + Gin
  Reader-facing API only. Catalog listing, cover, download, manifest, chapter.
```

The manager service is the only component that writes catalog content and administrative state. The reader backend can read the same catalog database rows and catalog storage files, but it should not expose upload, edit, moderation, or admin login APIs.

## Service Ownership

### Manager Web

Responsibilities:

- Admin login page.
- Dashboard with operational counts.
- User list, search, detail, status changes.
- Thread and comment moderation.
- Online catalog book list, upload, metadata editing, status changes.
- Audit log browsing.

Technology:

- React.
- TypeScript.
- Vite.
- Ant Design.

### Manager Service

Responsibilities:

- Admin authentication.
- Admin session or token validation.
- File upload handling.
- EPUB and TXT validation.
- Content SHA1 calculation and duplicate detection.
- Metadata, chapter, and cover inspection through importers.
- Catalog storage writes.
- Catalog database writes.
- User status changes.
- Community thread/comment status changes.
- Audit logging.

Technology:

- Go.
- Gin.
- MariaDB/MySQL.
- Zap-compatible structured logging.

### Reader Backend

Responsibilities:

- Serve reader app catalog APIs.
- Read `catalog_books` rows with active status only.
- Read catalog files from the shared catalog root.
- Generate or return cover, download, manifest, and chapter responses.
- Support EPUB and TXT through a read-only parser/importer interface.

It should not:

- Accept uploads.
- Manage admin login.
- Write catalog rows.
- Moderate posts or users.

## Authentication

The first version supports one administrator account.

Configuration example:

```yaml
admin:
  username: "admin"
  password_hash: "$2a$12$..."
  token_secret: "change-this-secret"
  access_token_expiry: 8h
```

The manager service exposes:

- `POST /admin/auth/login`
- `POST /admin/auth/logout`
- `GET /admin/auth/me`

The access token is independent from reader app user tokens. Manager APIs use a dedicated middleware and should not accept reader app bearer tokens.

## Catalog Storage

The first version uses local filesystem storage owned by the manager service. The reader backend receives read-only access to the same root path.

Configuration example:

```yaml
catalog:
  storage:
    provider: local
    root: /data/oops-reader/catalog
    temp_root: /data/oops-reader/catalog-tmp
```

Storage layout:

```text
catalog-root/
  originals/
    epub/ab/cd/<book_key>.epub
    txt/ef/12/<book_key>.txt
  covers/
    ab/cd/<book_key>.<ext>
  derived/
    <book_key>/manifest.json
```

The first implementation may generate manifest data on demand, but the `derived` directory is reserved for cached manifests and future derived assets. Paths stored in the database should be catalog-root-relative where practical, so deployment paths can change without rewriting rows.

## Book Import Flow

1. Manager web uploads an EPUB or TXT file to manager service.
2. Manager service writes the upload to `temp_root`.
3. Manager service computes file size and SHA1.
4. Manager service checks duplicate content by SHA1.
5. Manager service selects an importer by detected format.
6. Importer validates the file and extracts title, author, language, chapter count, and cover when available.
7. Manager service creates a stable `book_key`.
8. Manager service moves the original file to `catalog-root/originals/<format>/<hash-prefix>/<book_key>.<ext>`.
9. Manager service writes or updates `catalog_books` inside a transaction with `status = draft`.
10. Manager service writes an audit log entry.
11. The administrator reviews metadata and publishes the book.
12. Reader backend can serve the book through existing catalog read APIs after `status = active`.

If any step after temporary upload fails, the manager service cleans up temporary files. If the database write fails after final file movement, the service removes the final file before returning an error.

## Importer Interface

The manager service should use a format-oriented importer interface:

```go
type Importer interface {
    Format() string
    Inspect(ctx context.Context, filePath string) (ImportedBook, error)
    Manifest(ctx context.Context, filePath string) (Manifest, error)
    Chapter(ctx context.Context, filePath string, chapterID string) (ChapterContent, error)
    Cover(ctx context.Context, filePath string) (*Cover, error)
}
```

First-version importers:

- `epubImporter`: parse OPF metadata, spine/nav chapters, and embedded cover.
- `txtImporter`: detect title from filename or first heading, split chapters by common heading patterns, and fall back to single chapter or size-based sections.

Future importers, such as MOBI or PDF, should be added without changing manager web workflows or reader-facing catalog response shapes.

## Database Design

Extend the existing `catalog_books` concept instead of creating an unrelated catalog table.

Recommended fields:

- `book_key`: stable public ID.
- `title`.
- `author`.
- `description`.
- `format`: `epub`, `txt`, future values.
- `filename`: original filename.
- `storage_path`: catalog-root-relative original file path.
- `cover_storage_path`: catalog-root-relative cover path, nullable.
- `file_size`.
- `content_sha1`.
- `language`.
- `chapter_count`.
- `status`: `draft`, `active`, `hidden`, `deleted`.
- `source`: `admin_upload`, `batch_import`, future values.
- `uploaded_at`.
- `published_at`.
- `deleted_at`.
- `created_at`.
- `updated_at`.
- `updated_by`.

Add `admin_audit_logs`:

- `id`.
- `admin_username`.
- `action`.
- `resource_type`.
- `resource_id`.
- `before_json`.
- `after_json`.
- `ip_address`.
- `user_agent`.
- `created_at`.

Reader backend should query only `status = active` books for public catalog endpoints.

## Manager API Surface

Authentication:

- `POST /admin/auth/login`
- `POST /admin/auth/logout`
- `GET /admin/auth/me`

Dashboard:

- `GET /admin/dashboard/summary`

Users:

- `GET /admin/users`
- `GET /admin/users/:id`
- `PATCH /admin/users/:id/status`

Community:

- `GET /admin/community/threads`
- `GET /admin/community/threads/:id`
- `PATCH /admin/community/threads/:id/status`
- `GET /admin/community/comments`
- `PATCH /admin/community/comments/:id/status`

Catalog:

- `GET /admin/catalog/books`
- `GET /admin/catalog/books/:id`
- `POST /admin/catalog/books/upload`
- `PATCH /admin/catalog/books/:id`
- `PATCH /admin/catalog/books/:id/status`
- `DELETE /admin/catalog/books/:id`

Audit:

- `GET /admin/audit-logs`

All write endpoints must create audit log records.

## Reader Backend Adaptation

The reader backend should remain read-only but needs to understand the shared catalog contract:

- Read `format` from `catalog_books`.
- Filter public catalog list/detail queries by `status = active`.
- Resolve `storage_path` relative to the configured catalog root.
- Use EPUB parsing for EPUB books.
- Use TXT parsing for TXT books.
- Return the existing manifest/chapter response shape for both EPUB and TXT.
- Prefer `cover_storage_path` for covers. If missing, EPUB can fall back to embedded cover extraction. TXT can return a default cover or no cover.
- Keep existing download endpoint behavior.

## Frontend UX

The manager web should be an operational interface, not a landing page.

Initial navigation:

- Dashboard.
- Users.
- Posts.
- Books.
- Audit Logs.

Book management workflow:

- Books table with search, format, status, and upload date filters.
- Upload drawer or modal with file picker.
- Upload progress and validation result.
- Metadata review form before publish. Uploaded books start as drafts.
- Detail page with file info, parsed metadata, cover preview, and status actions.

Post management workflow:

- Thread list with board, author, status, created time, and reaction/comment counts.
- Thread detail with comments and attachments.
- Hide/restore actions with required reason.

User management workflow:

- User list with search by email/name/id.
- User detail with status and account metadata.
- Disable/restore actions with required reason.

## Testing Strategy

Manager service:

- Unit tests for admin auth.
- Unit tests for storage path generation.
- Unit tests for duplicate detection.
- Unit tests for EPUB and TXT importers.
- Handler tests for auth-required manager APIs.
- Integration-style tests for upload transaction behavior with temporary directories and fake stores.

Reader backend:

- Tests for active-only catalog filtering.
- Tests for EPUB and TXT manifest/chapter responses.
- Tests for cover fallback behavior.

Manager web:

- Component tests for upload and metadata forms where practical.
- API client tests for auth handling.
- Smoke test for login and books list.

## Rollout Plan

1. Create manager service skeleton with config, logging, health check, admin auth, and audit store.
2. Add catalog schema migration for format/status/storage fields and audit logs.
3. Implement local catalog storage and EPUB/TXT importers in manager service.
4. Implement manager catalog upload and book management APIs.
5. Adapt reader backend catalog read path to the shared contract and TXT support.
6. Build manager web login and book management pages.
7. Add users and community moderation APIs and pages.
8. Add dashboard and audit log pages.

## Implementation Decisions

- Repository layout: use one manager repository with `web/` and `service/` top-level directories.
- Shared contract: do not introduce a shared Go module in the first version. Duplicate the small catalog response contract where needed, and revisit shared packages only if drift becomes painful.
- TXT chapter splitting: use heading-based splitting first, then size-based fallback, then single-chapter fallback.
- Deletion: use soft delete first. Set `status = deleted` and `deleted_at`; keep files for recovery. Add physical cleanup later as an explicit maintenance task.
- Publish behavior: uploaded books start as `draft`. Reader backend only exposes `active` books.
