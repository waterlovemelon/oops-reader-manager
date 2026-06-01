# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Admin management platform for the "Oops Reader" e-book ecosystem. This is the **sole owner of administrative writes** — it handles admin auth, book uploads (EPUB/TXT), user management, community moderation, and audit logging. The companion `oops-reader-backend` is read-only and shares the same MySQL database and file storage.

## Development Commands

### Backend (Go — `service/`)

```bash
cd service
go test ./...                      # run all tests
go test ./internal/catalog/...     # run tests for a single package
go test -run TestFunctionName      # run a single test
go run ./cmd/api                   # start server (needs config.yaml)
```

Requires `config.yaml` in `service/` (copy from `config.yaml.example`). Expects MySQL/MariaDB at localhost:3306, database `oops_reader`. Server runs on port 8090.

### Frontend (React — `web/`)

```bash
cd web
npm install          # install dependencies
npm run dev          # dev server on :3000 (proxies /admin → :8090)
npm run build        # production build (tsc -b && vite build)
npm run test         # run tests (vitest run)
npm run lint         # eslint
```

## Architecture

### Repo Layout

- `service/` — Go API server (Gin). All backend code lives under `service/internal/`.
- `web/` — React SPA (Vite + Ant Design + TypeScript).
- `docs/` — Design specs and implementation plans.

### Backend Module Map (`service/internal/`)

| Package | Role |
|---------|------|
| `httpapi` | Gin router, HTTP handlers, auth middleware. All admin routes under `/admin/`. |
| `adminauth` | Single-admin auth: bcrypt passwords, custom HMAC-SHA256 token scheme (not JWT). |
| `catalog` | Book upload pipeline: format detection → SHA1 dedup → importer dispatch → file storage → DB insert. |
| `content` | EPUB parser (OPF/NCX/nav chapter extraction, cover, HTML-to-text). |
| `audit` | Audit log writes for all admin operations. |
| `usersadmin` | User account listing and management. |
| `communityadmin` | Community thread/comment moderation. |
| `config` | Viper-based YAML config with env overrides. |
| `platform/db` | MySQL connection pool. |
| `platform/log` | Zap logger factory. |

### Data Flow: Book Upload

1. Frontend sends multipart file → `POST /admin/catalog/books/upload`
2. `AdminRequired` middleware validates Bearer token
3. Handler saves to temp dir → `catalog.Service.ImportUploadedFile`
4. SHA1 computed → duplicate check → format-specific importer (EPUB or TXT)
5. Importer extracts metadata (title, author, chapters, cover)
6. File stored at `catalog-root/originals/<format>/<sha1[0:2]>/<sha1[2:4]>/<book_key>.<ext>`
7. DB record inserted with `status = draft` (reader backend only serves `status = active`)

## Key Conventions

### Backend Patterns

- **Interface-based stores**: Each domain package defines a `Store` interface + `MySQLStore` implementation. Tests use fake stores.
- **Importer interface**: `catalog.Importer` with `Format()`, `Inspect()`, `Manifest()`, `Chapter()`, `Cover()` — add new formats without changing the upload service.
- **Service layer**: Business logic in `Service` structs depending on store interfaces, not on HTTP handlers directly.
- **Dependency injection**: `httpapi.Deps` struct carries config, DB, logger; services constructed in `router.go`.

### API Conventions

- All admin routes: `/admin/` prefix, `AdminRequired` middleware
- Response envelope: `{ "data": ... }` or `{ "error": "..." }`
- Pagination: `page` + `page_size` query params → `pagination` in response
- Soft delete for books: `status = deleted` with `deleted_at` timestamp
- SHA1 content-hash dedup; slug-based book keys with hash prefix for uniqueness

### Frontend Patterns

- Chinese UI labels throughout
- Ant Design components and layout
- Token stored in localStorage; `authStore.ts` manages auth state
- API calls in `src/api/` use a shared `fetchWithAuth` wrapper with Bearer token

### Testing Conventions

- Go: colocated `*_test.go` files, fake store implementations, `t.TempDir()` for filesystem isolation
- `adminauth.Service` has injectable `now` func for time-dependent tests
- Frontend: Vitest + @testing-library/react + jsdom
