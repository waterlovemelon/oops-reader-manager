# Oops Reader Manager Platform Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an independent Oops Reader manager web app and manager service, with reader backend catalog read-path adaptation.

**Architecture:** `oops-reader-manager` contains `service/` and `web/`. The manager service owns admin auth, uploads, catalog writes, moderation writes, and audit logs. `oops-reader-backend` remains reader-facing and read-only, but reads the shared catalog schema and storage layout.

**Tech Stack:** Go 1.21, Gin, Viper, Zap, MySQL/MariaDB, React, TypeScript, Vite, Ant Design, Vitest.

---

## Repository Map

Manager repository:

```text
oops-reader-manager/
  service/
    cmd/api/main.go
    config.yaml.example
    go.mod
    internal/adminauth/
    internal/audit/
    internal/catalog/
    internal/communityadmin/
    internal/config/
    internal/httpapi/
    internal/platform/db/
    internal/platform/log/
    internal/usersadmin/
    migrations/
  web/
    package.json
    vite.config.ts
    tsconfig.json
    src/
      api/
      app/
      components/
      features/
      pages/
```

Reader backend repository:

```text
oops-reader-backend/
  internal/catalog/
  internal/content/
  internal/transport/http/handlers/catalog.go
  migrations/
```

## Implementation Rules

- Keep commits small. Commit after every task that passes its verification command.
- Write tests before implementation for service logic and reader backend changes.
- Do not add manager write endpoints to `oops-reader-backend`.
- Uploaded books start as `draft`; reader backend exposes only `active`.
- Use soft delete for catalog books.
- Prefer catalog-root-relative paths in database rows.

## Phase 1: Manager Service Skeleton

### Task 1: Initialize Go Service Module

**Files:**
- Create: `service/go.mod`
- Create: `service/cmd/api/main.go`
- Create: `service/config.yaml.example`
- Create: `service/internal/config/config.go`
- Create: `service/internal/platform/log/log.go`
- Create: `service/internal/platform/db/db.go`
- Create: `service/internal/httpapi/router.go`

- [ ] **Step 1: Create `service/go.mod`**

```go
module github.com/oops-reader/oops-reader-manager/service

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/go-sql-driver/mysql v1.7.1
	github.com/spf13/viper v1.18.2
	go.uber.org/zap v1.26.0
	golang.org/x/crypto v0.18.0
)
```

- [ ] **Step 2: Create `service/config.yaml.example`**

```yaml
server:
  port: 8090
  mode: debug

database:
  host: localhost
  port: 3306
  user: root
  password: "your_password_here"
  database: oops_reader
  max_open_conns: 20
  max_idle_conns: 5

admin:
  username: admin
  password_hash: "$2a$12$replace-with-bcrypt-hash"
  token_secret: "change-this-manager-token-secret"
  access_token_expiry: 8h

catalog:
  storage:
    provider: local
    root: /data/oops-reader/catalog
    temp_root: /data/oops-reader/catalog-tmp

log:
  level: info
  format: console
```

- [ ] **Step 3: Create config structs in `service/internal/config/config.go`**

```go
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Admin    AdminConfig    `mapstructure:"admin"`
	Catalog  CatalogConfig  `mapstructure:"catalog"`
	Log      LogConfig      `mapstructure:"log"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type AdminConfig struct {
	Username          string        `mapstructure:"username"`
	PasswordHash      string        `mapstructure:"password_hash"`
	TokenSecret       string        `mapstructure:"token_secret"`
	AccessTokenExpiry time.Duration `mapstructure:"access_token_expiry"`
}

type CatalogConfig struct {
	Storage CatalogStorageConfig `mapstructure:"storage"`
}

type CatalogStorageConfig struct {
	Provider string `mapstructure:"provider"`
	Root     string `mapstructure:"root"`
	TempRoot string `mapstructure:"temp_root"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.SetDefault("server.port", 8090)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("admin.access_token_expiry", "8h")
	v.SetDefault("catalog.storage.provider", "local")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}
```

- [ ] **Step 4: Create logger in `service/internal/platform/log/log.go`**

```go
package log

import (
	"github.com/oops-reader/oops-reader-manager/service/internal/config"
	"go.uber.org/zap"
)

func NewLogger(cfg *config.Config) *zap.Logger {
	if cfg.Log.Format == "json" {
		logger, _ := zap.NewProduction()
		return logger
	}
	logger, _ := zap.NewDevelopment()
	return logger
}
```

- [ ] **Step 5: Create DB connector in `service/internal/platform/db/db.go`**

```go
package db

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/oops-reader/oops-reader-manager/service/internal/config"
)

func Open(cfg config.DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)
	pool, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		pool.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		pool.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if err := pool.Ping(); err != nil {
		_ = pool.Close()
		return nil, err
	}
	return pool, nil
}
```

- [ ] **Step 6: Create health router in `service/internal/httpapi/router.go`**

```go
package httpapi

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/config"
	"go.uber.org/zap"
)

type Deps struct {
	Config *config.Config
	DB     *sql.DB
	Logger *zap.Logger
}

func NewRouter(deps Deps) *gin.Engine {
	if deps.Config.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return router
}
```

- [ ] **Step 7: Create entrypoint in `service/cmd/api/main.go`**

```go
package main

import (
	"fmt"
	"net/http"

	"github.com/oops-reader/oops-reader-manager/service/internal/config"
	"github.com/oops-reader/oops-reader-manager/service/internal/httpapi"
	"github.com/oops-reader/oops-reader-manager/service/internal/platform/db"
	"github.com/oops-reader/oops-reader-manager/service/internal/platform/log"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	logger := log.NewLogger(cfg)
	pool, err := db.Open(cfg.Database)
	if err != nil {
		logger.Fatal("open database", zap.Error(err))
	}
	defer pool.Close()
	router := httpapi.NewRouter(httpapi.Deps{Config: cfg, DB: pool, Logger: logger})
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	if err := http.ListenAndServe(addr, router); err != nil {
		logger.Fatal("server stopped", zap.Error(err))
	}
}
```

- [ ] **Step 8: Download dependencies**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go mod tidy
```

Expected: `go.mod` and `go.sum` exist and `go mod tidy` exits 0.

- [ ] **Step 9: Verify build**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go test ./...
```

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add service
git commit -m "feat: scaffold manager service"
```

## Phase 2: Admin Auth and Audit

### Task 2: Add Admin Token Auth

**Files:**
- Create: `service/internal/adminauth/service.go`
- Create: `service/internal/adminauth/service_test.go`
- Create: `service/internal/httpapi/admin_auth_handler.go`
- Modify: `service/internal/httpapi/router.go`

- [ ] **Step 1: Write auth service tests**

Create `service/internal/adminauth/service_test.go`:

```go
package adminauth

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestLoginAndValidate(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(Config{
		Username: "admin",
		PasswordHash: string(hash),
		TokenSecret: "test-secret",
		AccessTokenExpiry: time.Hour,
	})
	token, err := svc.Login("admin", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	claims, err := svc.Validate(token)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.Username != "admin" {
		t.Fatalf("username = %q", claims.Username)
	}
}

func TestLoginRejectsBadPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(Config{
		Username: "admin",
		PasswordHash: string(hash),
		TokenSecret: "test-secret",
		AccessTokenExpiry: time.Hour,
	})
	if _, err := svc.Login("admin", "wrong"); err == nil {
		t.Fatal("expected bad password error")
	}
}
```

- [ ] **Step 2: Run failing tests**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go test ./internal/adminauth -run TestLogin -v
```

Expected: FAIL because `NewService` is undefined.

- [ ] **Step 3: Implement auth service**

Create `service/internal/adminauth/service.go`:

```go
package adminauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid admin credentials")
var ErrInvalidToken = errors.New("invalid admin token")

type Config struct {
	Username string
	PasswordHash string
	TokenSecret string
	AccessTokenExpiry time.Duration
}

type Claims struct {
	Username string `json:"username"`
	ExpiresAt int64 `json:"expires_at"`
}

type Service struct {
	cfg Config
	now func() time.Time
}

func NewService(cfg Config) *Service {
	return &Service{cfg: cfg, now: time.Now}
}

func (s *Service) Login(username, password string) (string, error) {
	if username != s.cfg.Username {
		return "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(s.cfg.PasswordHash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}
	claims := Claims{
		Username: username,
		ExpiresAt: s.now().Add(s.cfg.AccessTokenExpiry).Unix(),
	}
	return s.sign(claims)
}

func (s *Service) Validate(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Claims{}, ErrInvalidToken
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	expected := s.signature(parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return Claims{}, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(body, &claims); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if claims.ExpiresAt <= s.now().Unix() {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}

func (s *Service) sign(claims Claims) (string, error) {
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(body)
	return payload + "." + s.signature(payload), nil
}

func (s *Service) signature(payload string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.TokenSecret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
```

- [ ] **Step 4: Run auth tests**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go test ./internal/adminauth -v
```

Expected: PASS.

- [ ] **Step 5: Add auth handler**

Create `service/internal/httpapi/admin_auth_handler.go`:

```go
package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/adminauth"
)

type AdminAuthHandler struct {
	service *adminauth.Service
}

func NewAdminAuthHandler(service *adminauth.Service) *AdminAuthHandler {
	return &AdminAuthHandler{service: service}
}

type adminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AdminAuthHandler) Login(c *gin.Context) {
	var req adminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	token, err := h.service.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"access_token": token}})
}

func (h *AdminAuthHandler) Me(c *gin.Context) {
	claims, ok := CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"username": claims.Username}})
}

func AdminRequired(service *adminauth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		value := c.GetHeader("Authorization")
		token := strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
		if token == "" || token == value {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
			return
		}
		claims, err := service.Validate(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("admin_claims", claims)
		c.Next()
	}
}

func CurrentAdmin(c *gin.Context) (adminauth.Claims, bool) {
	value, ok := c.Get("admin_claims")
	if !ok {
		return adminauth.Claims{}, false
	}
	claims, ok := value.(adminauth.Claims)
	return claims, ok
}
```

- [ ] **Step 6: Wire auth routes**

Modify `service/internal/httpapi/router.go` to create `adminauth.Service` and routes:

```go
authService := adminauth.NewService(adminauth.Config{
	Username: deps.Config.Admin.Username,
	PasswordHash: deps.Config.Admin.PasswordHash,
	TokenSecret: deps.Config.Admin.TokenSecret,
	AccessTokenExpiry: deps.Config.Admin.AccessTokenExpiry,
})
authHandler := NewAdminAuthHandler(authService)
admin := router.Group("/admin")
admin.POST("/auth/login", authHandler.Login)
admin.GET("/auth/me", AdminRequired(authService), authHandler.Me)
```

Also add import:

```go
"github.com/oops-reader/oops-reader-manager/service/internal/adminauth"
```

- [ ] **Step 7: Run all service tests**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go test ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add service
git commit -m "feat: add manager admin authentication"
```

### Task 3: Add Audit Store

**Files:**
- Create: `service/migrations/001_manager_audit_logs.sql`
- Create: `service/internal/audit/store.go`
- Create: `service/internal/audit/mysql_store.go`
- Create: `service/internal/audit/service.go`
- Create: `service/internal/audit/service_test.go`

- [ ] **Step 1: Create migration**

Create `service/migrations/001_manager_audit_logs.sql`:

```sql
CREATE TABLE IF NOT EXISTS `admin_audit_logs` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    `admin_username` VARCHAR(191) NOT NULL,
    `action` VARCHAR(191) NOT NULL,
    `resource_type` VARCHAR(191) NOT NULL,
    `resource_id` VARCHAR(191) NOT NULL,
    `before_json` JSON NULL,
    `after_json` JSON NULL,
    `ip_address` VARCHAR(64) NULL,
    `user_agent` VARCHAR(512) NULL,
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX `idx_admin_audit_created_at` (`created_at`),
    INDEX `idx_admin_audit_resource` (`resource_type`, `resource_id`),
    INDEX `idx_admin_audit_admin` (`admin_username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

- [ ] **Step 2: Write audit service test**

Create `service/internal/audit/service_test.go`:

```go
package audit

import (
	"context"
	"testing"
)

type fakeStore struct {
	entries []Entry
}

func (s *fakeStore) Create(ctx context.Context, entry Entry) error {
	s.entries = append(s.entries, entry)
	return nil
}

func TestRecordRequiresActionAndResource(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	err := service.Record(context.Background(), Entry{AdminUsername: "admin"})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRecordPersistsEntry(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	err := service.Record(context.Background(), Entry{
		AdminUsername: "admin",
		Action: "catalog.publish",
		ResourceType: "catalog_book",
		ResourceID: "book-1",
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if len(store.entries) != 1 {
		t.Fatalf("entries = %d", len(store.entries))
	}
}
```

- [ ] **Step 3: Implement audit store interface and service**

Create `service/internal/audit/store.go`:

```go
package audit

import "context"

type Entry struct {
	AdminUsername string
	Action string
	ResourceType string
	ResourceID string
	BeforeJSON []byte
	AfterJSON []byte
	IPAddress string
	UserAgent string
}

type Store interface {
	Create(ctx context.Context, entry Entry) error
}
```

Create `service/internal/audit/service.go`:

```go
package audit

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidEntry = errors.New("invalid audit entry")

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Record(ctx context.Context, entry Entry) error {
	if strings.TrimSpace(entry.AdminUsername) == "" ||
		strings.TrimSpace(entry.Action) == "" ||
		strings.TrimSpace(entry.ResourceType) == "" ||
		strings.TrimSpace(entry.ResourceID) == "" {
		return ErrInvalidEntry
	}
	return s.store.Create(ctx, entry)
}
```

- [ ] **Step 4: Implement MySQL audit store**

Create `service/internal/audit/mysql_store.go`:

```go
package audit

import (
	"context"
	"database/sql"
	"fmt"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) Create(ctx context.Context, entry Entry) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO admin_audit_logs
(admin_username, action, resource_type, resource_id, before_json, after_json, ip_address, user_agent)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.AdminUsername,
		entry.Action,
		entry.ResourceType,
		entry.ResourceID,
		nullableBytes(entry.BeforeJSON),
		nullableBytes(entry.AfterJSON),
		nullableString(entry.IPAddress),
		nullableString(entry.UserAgent),
	)
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
```

- [ ] **Step 5: Run audit tests**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go test ./internal/audit -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add service
git commit -m "feat: add manager audit logging"
```

## Phase 3: Catalog Upload in Manager Service

### Task 4: Add Catalog Schema Migration

**Files:**
- Create: `service/migrations/002_extend_catalog_books_for_manager.sql`
- Copy to backend migration path during Task 4: `/home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-backend/migrations/010_extend_catalog_books_for_manager.sql`

- [ ] **Step 1: Create migration in manager service**

Create `service/migrations/002_extend_catalog_books_for_manager.sql`:

```sql
ALTER TABLE `catalog_books`
    ADD COLUMN `description` TEXT NULL AFTER `author`,
    ADD COLUMN `format` VARCHAR(32) NOT NULL DEFAULT 'epub' AFTER `description`,
    ADD COLUMN `cover_storage_path` VARCHAR(1024) NULL AFTER `storage_path`,
    ADD COLUMN `source` VARCHAR(64) NOT NULL DEFAULT 'batch_import' AFTER `status`,
    ADD COLUMN `uploaded_at` DATETIME NULL AFTER `indexed_at`,
    ADD COLUMN `published_at` DATETIME NULL AFTER `uploaded_at`,
    ADD COLUMN `deleted_at` DATETIME NULL AFTER `published_at`,
    ADD COLUMN `updated_by` VARCHAR(191) NULL AFTER `deleted_at`;

ALTER TABLE `catalog_books`
    MODIFY COLUMN `status` VARCHAR(32) NOT NULL DEFAULT 'active';

CREATE INDEX `idx_catalog_books_format_status` ON `catalog_books` (`format`, `status`);
CREATE INDEX `idx_catalog_books_uploaded_at` ON `catalog_books` (`uploaded_at`);
```

- [ ] **Step 2: Copy migration to backend repository**

Run:

```bash
cp /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service/migrations/002_extend_catalog_books_for_manager.sql \
  /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-backend/migrations/010_extend_catalog_books_for_manager.sql
```

Expected: backend migration file exists.

- [ ] **Step 3: Commit manager migration**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add service/migrations/002_extend_catalog_books_for_manager.sql
git commit -m "feat: add manager catalog schema migration"
```

### Task 5: Add Local Catalog Storage

**Files:**
- Create: `service/internal/catalog/storage.go`
- Create: `service/internal/catalog/storage_test.go`

- [ ] **Step 1: Write storage path tests**

Create `service/internal/catalog/storage_test.go`:

```go
package catalog

import "testing"

func TestOriginalPathUsesFormatAndHashPrefix(t *testing.T) {
	storage := NewLocalStorage("/catalog", "/tmp/catalog")
	got := storage.OriginalPath("epub", "abcdef1234567890", "book-1", ".epub")
	want := "/catalog/originals/epub/ab/cd/book-1.epub"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestRelativeOriginalPath(t *testing.T) {
	storage := NewLocalStorage("/catalog", "/tmp/catalog")
	got := storage.RelativeOriginalPath("txt", "abcdef1234567890", "book-1", ".txt")
	want := "originals/txt/ab/cd/book-1.txt"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Implement local storage**

Create `service/internal/catalog/storage.go`:

```go
package catalog

import (
	"path/filepath"
	"strings"
)

type LocalStorage struct {
	root string
	tempRoot string
}

func NewLocalStorage(root, tempRoot string) *LocalStorage {
	return &LocalStorage{root: filepath.Clean(root), tempRoot: filepath.Clean(tempRoot)}
}

func (s *LocalStorage) Root() string {
	return s.root
}

func (s *LocalStorage) TempRoot() string {
	return s.tempRoot
}

func (s *LocalStorage) OriginalPath(format, sha1, bookKey, ext string) string {
	return filepath.Join(s.root, s.RelativeOriginalPath(format, sha1, bookKey, ext))
}

func (s *LocalStorage) RelativeOriginalPath(format, sha1, bookKey, ext string) string {
	a, b := hashPrefix(sha1)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return filepath.ToSlash(filepath.Join("originals", format, a, b, bookKey+ext))
}

func hashPrefix(sha1 string) (string, string) {
	if len(sha1) < 4 {
		return "00", "00"
	}
	return sha1[:2], sha1[2:4]
}
```

- [ ] **Step 3: Run storage tests**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go test ./internal/catalog -run TestOriginalPath -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add service/internal/catalog
git commit -m "feat: add catalog local storage paths"
```

### Task 6: Add EPUB and TXT Importer Interfaces

**Files:**
- Create: `service/internal/catalog/importer.go`
- Create: `service/internal/catalog/txt_importer.go`
- Create: `service/internal/catalog/txt_importer_test.go`
- Create: `service/internal/catalog/epub_importer.go`

- [ ] **Step 1: Create importer contracts**

Create `service/internal/catalog/importer.go`:

```go
package catalog

import "context"

type ImportedBook struct {
	Title string
	Author string
	Description string
	Language string
	ChapterCount int
	CoverMediaType string
	CoverData []byte
}

type Manifest struct {
	BookID string
	Title string
	Author string
	Chapters []Chapter
}

type Chapter struct {
	ID string
	Title string
	Text string
}

type Importer interface {
	Format() string
	Inspect(ctx context.Context, filePath string) (ImportedBook, error)
	Manifest(ctx context.Context, filePath string) (Manifest, error)
	Chapter(ctx context.Context, filePath string, chapterID string) (Chapter, error)
	Cover(ctx context.Context, filePath string) (*Cover, error)
}

type Cover struct {
	MediaType string
	Data []byte
}
```

- [ ] **Step 2: Write TXT importer tests**

Create `service/internal/catalog/txt_importer_test.go`:

```go
package catalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTXTImporterSplitsHeadings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	body := "第一章 开始\n这里是第一章。\n第二章 继续\n这里是第二章。"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	importer := TXTImporter{}
	manifest, err := importer.Manifest(context.Background(), path)
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if len(manifest.Chapters) != 2 {
		t.Fatalf("chapters = %d", len(manifest.Chapters))
	}
	if manifest.Chapters[0].Title != "第一章 开始" {
		t.Fatalf("title = %q", manifest.Chapters[0].Title)
	}
}

func TestTXTImporterFallsBackToSingleChapter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.txt")
	if err := os.WriteFile(path, []byte("没有章节标题的内容"), 0644); err != nil {
		t.Fatal(err)
	}
	importer := TXTImporter{}
	manifest, err := importer.Manifest(context.Background(), path)
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if len(manifest.Chapters) != 1 {
		t.Fatalf("chapters = %d", len(manifest.Chapters))
	}
}
```

- [ ] **Step 3: Implement TXT importer**

Create `service/internal/catalog/txt_importer.go`:

```go
package catalog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var txtHeadingPattern = regexp.MustCompile(`(?m)^(第[一二三四五六七八九十百千万0-9]+[章节回卷部].*|Chapter\s+[0-9]+.*)$`)

type TXTImporter struct{}

func (TXTImporter) Format() string {
	return "txt"
}

func (i TXTImporter) Inspect(ctx context.Context, filePath string) (ImportedBook, error) {
	manifest, err := i.Manifest(ctx, filePath)
	if err != nil {
		return ImportedBook{}, err
	}
	title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	return ImportedBook{
		Title: title,
		ChapterCount: len(manifest.Chapters),
	}, nil
}

func (TXTImporter) Manifest(ctx context.Context, filePath string) (Manifest, error) {
	body, err := os.ReadFile(filePath)
	if err != nil {
		return Manifest{}, err
	}
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	matches := txtHeadingPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return Manifest{Chapters: []Chapter{{ID: "ch-1", Title: "正文", Text: text}}}, nil
	}
	chapters := make([]Chapter, 0, len(matches))
	for idx, match := range matches {
		start := match[0]
		end := len(text)
		if idx+1 < len(matches) {
			end = matches[idx+1][0]
		}
		chunk := strings.TrimSpace(text[start:end])
		lines := strings.SplitN(chunk, "\n", 2)
		title := strings.TrimSpace(lines[0])
		chapters = append(chapters, Chapter{
			ID: fmt.Sprintf("ch-%d", idx+1),
			Title: title,
			Text: chunk,
		})
	}
	return Manifest{Chapters: chapters}, nil
}

func (i TXTImporter) Chapter(ctx context.Context, filePath string, chapterID string) (Chapter, error) {
	manifest, err := i.Manifest(ctx, filePath)
	if err != nil {
		return Chapter{}, err
	}
	for _, chapter := range manifest.Chapters {
		if chapter.ID == chapterID {
			return chapter, nil
		}
	}
	return Chapter{}, fmt.Errorf("chapter not found: %s", chapterID)
}

func (TXTImporter) Cover(ctx context.Context, filePath string) (*Cover, error) {
	return nil, nil
}
```

- [ ] **Step 4: Add temporary EPUB importer before porting parser**

Create `service/internal/catalog/epub_importer.go`:

```go
package catalog

import (
	"context"
	"fmt"
)

type EPUBImporter struct{}

func (EPUBImporter) Format() string {
	return "epub"
}

func (EPUBImporter) Inspect(ctx context.Context, filePath string) (ImportedBook, error) {
	return ImportedBook{}, fmt.Errorf("epub importer implementation requires porting content.ParseEPUB from oops-reader-backend")
}

func (EPUBImporter) Manifest(ctx context.Context, filePath string) (Manifest, error) {
	return Manifest{}, fmt.Errorf("epub importer implementation requires porting content.ParseEPUB from oops-reader-backend")
}

func (EPUBImporter) Chapter(ctx context.Context, filePath string, chapterID string) (Chapter, error) {
	return Chapter{}, fmt.Errorf("epub importer implementation requires porting content.ParseEPUB from oops-reader-backend")
}

func (EPUBImporter) Cover(ctx context.Context, filePath string) (*Cover, error) {
	return nil, fmt.Errorf("epub importer implementation requires porting content.ExtractEPUBCover from oops-reader-backend")
}
```

- [ ] **Step 5: Run TXT importer tests**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go test ./internal/catalog -run TestTXTImporter -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add service/internal/catalog
git commit -m "feat: add catalog importer contracts and txt importer"
```

### Task 7: Port EPUB Parser Into Manager Service

**Files:**
- Create: `service/internal/content/epub.go`
- Modify: `service/internal/catalog/epub_importer.go`
- Create: `service/internal/catalog/epub_importer_test.go`

- [ ] **Step 1: Copy backend EPUB parser**

Run:

```bash
mkdir -p /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service/internal/content
cp /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-backend/internal/content/epub.go \
  /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service/internal/content/epub.go
```

Expected: `service/internal/content/epub.go` exists.

- [ ] **Step 2: Replace temporary EPUB importer**

Modify `service/internal/catalog/epub_importer.go`:

```go
package catalog

import (
	"context"
	"fmt"

	"github.com/oops-reader/oops-reader-manager/service/internal/content"
)

type EPUBImporter struct{}

func (EPUBImporter) Format() string {
	return "epub"
}

func (EPUBImporter) Inspect(ctx context.Context, filePath string) (ImportedBook, error) {
	book, err := content.ParseEPUB(filePath)
	if err != nil {
		return ImportedBook{}, err
	}
	return ImportedBook{
		Title: book.Title,
		Author: book.Author,
		ChapterCount: len(book.Chapters),
		CoverMediaType: book.CoverMediaType,
	}, nil
}

func (EPUBImporter) Manifest(ctx context.Context, filePath string) (Manifest, error) {
	book, err := content.ParseEPUB(filePath)
	if err != nil {
		return Manifest{}, err
	}
	chapters := make([]Chapter, 0, len(book.Chapters))
	for _, chapter := range book.Chapters {
		chapters = append(chapters, Chapter{ID: chapter.ID, Title: chapter.Title, Text: chapter.Text})
	}
	return Manifest{Title: book.Title, Author: book.Author, Chapters: chapters}, nil
}

func (i EPUBImporter) Chapter(ctx context.Context, filePath string, chapterID string) (Chapter, error) {
	manifest, err := i.Manifest(ctx, filePath)
	if err != nil {
		return Chapter{}, err
	}
	for _, chapter := range manifest.Chapters {
		if chapter.ID == chapterID {
			return chapter, nil
		}
	}
	return Chapter{}, fmt.Errorf("chapter not found: %s", chapterID)
}

func (EPUBImporter) Cover(ctx context.Context, filePath string) (*Cover, error) {
	cover, err := content.ExtractEPUBCover(filePath)
	if err != nil || cover == nil {
		return nil, err
	}
	return &Cover{MediaType: cover.MediaType, Data: cover.Data}, nil
}
```

- [ ] **Step 3: Add EPUB importer smoke test**

Create `service/internal/catalog/epub_importer_test.go`:

```go
package catalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEPUBImporterRejectsInvalidZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.epub")
	if err := os.WriteFile(path, []byte("not an epub"), 0644); err != nil {
		t.Fatal(err)
	}
	importer := EPUBImporter{}
	if _, err := importer.Inspect(context.Background(), path); err == nil {
		t.Fatal("expected invalid epub error")
	}
}
```

- [ ] **Step 4: Run catalog tests**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go test ./internal/catalog ./internal/content
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add service/internal/catalog service/internal/content service/go.mod service/go.sum
git commit -m "feat: add manager epub importer"
```

### Task 8: Add Catalog Store and Upload Service

**Files:**
- Create: `service/internal/catalog/book.go`
- Create: `service/internal/catalog/store.go`
- Create: `service/internal/catalog/mysql_store.go`
- Create: `service/internal/catalog/service.go`
- Create: `service/internal/catalog/service_test.go`

- [ ] **Step 1: Create catalog book model**

Create `service/internal/catalog/book.go`:

```go
package catalog

import "time"

type BookStatus string

const (
	StatusDraft BookStatus = "draft"
	StatusActive BookStatus = "active"
	StatusHidden BookStatus = "hidden"
	StatusDeleted BookStatus = "deleted"
)

type Book struct {
	BookKey string
	Title string
	Author string
	Description string
	Format string
	Filename string
	StoragePath string
	CoverStoragePath string
	FileSize int64
	ContentSHA1 string
	Language string
	ChapterCount int
	Status BookStatus
	Source string
	UploadedAt *time.Time
	PublishedAt *time.Time
	DeletedAt *time.Time
	UpdatedBy string
}
```

- [ ] **Step 2: Create store interface**

Create `service/internal/catalog/store.go`:

```go
package catalog

import "context"

type Store interface {
	FindBySHA1(ctx context.Context, sha1 string) (*Book, error)
	Create(ctx context.Context, book Book) error
	Update(ctx context.Context, book Book) error
	UpdateStatus(ctx context.Context, bookKey string, status BookStatus, admin string) error
}
```

- [ ] **Step 3: Implement MySQL store**

Create `service/internal/catalog/mysql_store.go`:

```go
package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("not found")

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) FindBySHA1(ctx context.Context, sha1 string) (*Book, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT book_key, title, author, description, format, filename, storage_path, cover_storage_path,
file_size, content_sha1, language, chapter_count, status, source, updated_by
FROM catalog_books WHERE content_sha1 = ? AND status <> 'deleted' LIMIT 1`, sha1)
	book, err := scanBook(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find catalog book by sha1: %w", err)
	}
	return &book, nil
}

func (s *MySQLStore) Create(ctx context.Context, book Book) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO catalog_books
(book_key, title, author, description, format, filename, storage_path, cover_storage_path,
file_size, content_sha1, language, chapter_count, status, source, uploaded_at, updated_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), ?)`,
		book.BookKey, book.Title, nullableString(book.Author), nullableString(book.Description),
		book.Format, book.Filename, book.StoragePath, nullableString(book.CoverStoragePath),
		book.FileSize, nullableString(book.ContentSHA1), nullableString(book.Language),
		book.ChapterCount, string(book.Status), book.Source, nullableString(book.UpdatedBy),
	)
	if err != nil {
		return fmt.Errorf("create catalog book: %w", err)
	}
	return nil
}

func (s *MySQLStore) Update(ctx context.Context, book Book) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE catalog_books
SET title = ?, author = ?, description = ?, language = ?, updated_by = ?
WHERE book_key = ?`,
		book.Title, nullableString(book.Author), nullableString(book.Description),
		nullableString(book.Language), nullableString(book.UpdatedBy), book.BookKey,
	)
	if err != nil {
		return fmt.Errorf("update catalog book: %w", err)
	}
	return nil
}

func (s *MySQLStore) UpdateStatus(ctx context.Context, bookKey string, status BookStatus, admin string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE catalog_books
SET status = ?,
    published_at = CASE WHEN ? = 'active' THEN NOW() ELSE published_at END,
    deleted_at = CASE WHEN ? = 'deleted' THEN NOW() ELSE deleted_at END,
    updated_by = ?
WHERE book_key = ?`,
		string(status), string(status), string(status), nullableString(admin), bookKey,
	)
	if err != nil {
		return fmt.Errorf("update catalog book status: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanBook(row scanner) (Book, error) {
	var book Book
	var author, description, coverPath, sha1, language, updatedBy sql.NullString
	if err := row.Scan(
		&book.BookKey, &book.Title, &author, &description, &book.Format, &book.Filename,
		&book.StoragePath, &coverPath, &book.FileSize, &sha1, &language, &book.ChapterCount,
		&book.Status, &book.Source, &updatedBy,
	); err != nil {
		return Book{}, err
	}
	book.Author = author.String
	book.Description = description.String
	book.CoverStoragePath = coverPath.String
	book.ContentSHA1 = sha1.String
	book.Language = language.String
	book.UpdatedBy = updatedBy.String
	return book, nil
}
```

- [ ] **Step 4: Implement upload service**

Create `service/internal/catalog/service.go` with upload orchestration:

```go
package catalog

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

var ErrDuplicateBook = errors.New("duplicate catalog book")
var ErrUnsupportedFormat = errors.New("unsupported book format")

type Service struct {
	store Store
	storage *LocalStorage
	importers map[string]Importer
}

type UploadInput struct {
	AdminUsername string
	OriginalFilename string
	TempPath string
}

func NewService(store Store, storage *LocalStorage, importers []Importer) *Service {
	byFormat := map[string]Importer{}
	for _, importer := range importers {
		byFormat[importer.Format()] = importer
	}
	return &Service{store: store, storage: storage, importers: byFormat}
}

func (s *Service) ImportUploadedFile(ctx context.Context, input UploadInput) (Book, error) {
	ext := strings.ToLower(filepath.Ext(input.OriginalFilename))
	format := strings.TrimPrefix(ext, ".")
	importer, ok := s.importers[format]
	if !ok {
		return Book{}, ErrUnsupportedFormat
	}
	sha, size, err := fileSHA1(input.TempPath)
	if err != nil {
		return Book{}, err
	}
	if existing, err := s.store.FindBySHA1(ctx, sha); err == nil && existing != nil {
		return Book{}, ErrDuplicateBook
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return Book{}, err
	}
	inspected, err := importer.Inspect(ctx, input.TempPath)
	if err != nil {
		return Book{}, err
	}
	bookKey := stableBookKey(inspected.Title, input.OriginalFilename, sha)
	relativePath := s.storage.RelativeOriginalPath(format, sha, bookKey, ext)
	finalPath := s.storage.OriginalPath(format, sha, bookKey, ext)
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return Book{}, err
	}
	if err := os.Rename(input.TempPath, finalPath); err != nil {
		return Book{}, err
	}
	now := time.Now()
	book := Book{
		BookKey: bookKey,
		Title: fallbackTitle(inspected.Title, input.OriginalFilename),
		Author: inspected.Author,
		Description: inspected.Description,
		Format: format,
		Filename: input.OriginalFilename,
		StoragePath: relativePath,
		FileSize: size,
		ContentSHA1: sha,
		Language: inspected.Language,
		ChapterCount: inspected.ChapterCount,
		Status: StatusDraft,
		Source: "admin_upload",
		UploadedAt: &now,
		UpdatedBy: input.AdminUsername,
	}
	if err := s.store.Create(ctx, book); err != nil {
		_ = os.Remove(finalPath)
		return Book{}, err
	}
	return book, nil
}

func fileSHA1(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha1.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func stableBookKey(title, filename, sha string) string {
	base := title
	if strings.TrimSpace(base) == "" {
		base = strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	id := slug(base)
	if id == "" {
		id = "book"
	}
	if len(sha) >= 10 {
		return id + "-" + sha[:10]
	}
	return id
}

func fallbackTitle(title, filename string) string {
	if strings.TrimSpace(title) != "" {
		return strings.TrimSpace(title)
	}
	return strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
}

func slug(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

```

- [ ] **Step 5: Add upload service tests**

Create `service/internal/catalog/service_test.go`:

```go
package catalog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeCatalogStore struct {
	bySHA map[string]Book
	created []Book
}

func (s *fakeCatalogStore) FindBySHA1(ctx context.Context, sha1 string) (*Book, error) {
	if book, ok := s.bySHA[sha1]; ok {
		return &book, nil
	}
	return nil, ErrNotFound
}

func (s *fakeCatalogStore) Create(ctx context.Context, book Book) error {
	s.created = append(s.created, book)
	return nil
}

func (s *fakeCatalogStore) Update(ctx context.Context, book Book) error { return nil }
func (s *fakeCatalogStore) UpdateStatus(ctx context.Context, bookKey string, status BookStatus, admin string) error { return nil }

func TestImportUploadedTXTCreatesDraftBook(t *testing.T) {
	dir := t.TempDir()
	temp := filepath.Join(dir, "upload.txt")
	if err := os.WriteFile(temp, []byte("第一章 开始\n内容"), 0644); err != nil {
		t.Fatal(err)
	}
	store := &fakeCatalogStore{bySHA: map[string]Book{}}
	service := NewService(store, NewLocalStorage(filepath.Join(dir, "catalog"), filepath.Join(dir, "tmp")), []Importer{TXTImporter{}})
	book, err := service.ImportUploadedFile(context.Background(), UploadInput{
		AdminUsername: "admin",
		OriginalFilename: "测试.txt",
		TempPath: temp,
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if book.Status != StatusDraft {
		t.Fatalf("status = %s", book.Status)
	}
	if len(store.created) != 1 {
		t.Fatalf("created = %d", len(store.created))
	}
	if _, err := os.Stat(filepath.Join(dir, "catalog", book.StoragePath)); err != nil {
		t.Fatalf("final file missing: %v", err)
	}
}

func TestImportUploadedFileRejectsUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	temp := filepath.Join(dir, "upload.pdf")
	if err := os.WriteFile(temp, []byte("pdf"), 0644); err != nil {
		t.Fatal(err)
	}
	store := &fakeCatalogStore{bySHA: map[string]Book{}}
	service := NewService(store, NewLocalStorage(filepath.Join(dir, "catalog"), filepath.Join(dir, "tmp")), []Importer{TXTImporter{}})
	_, err := service.ImportUploadedFile(context.Background(), UploadInput{
		AdminUsername: "admin",
		OriginalFilename: "测试.pdf",
		TempPath: temp,
	})
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 6: Run catalog tests**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go test ./internal/catalog -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add service/internal/catalog
git commit -m "feat: add manager catalog upload service"
```

### Task 9: Add Catalog Upload HTTP Endpoint

**Files:**
- Create: `service/internal/httpapi/catalog_handler.go`
- Modify: `service/internal/httpapi/router.go`

- [ ] **Step 1: Create catalog handler**

Create `service/internal/httpapi/catalog_handler.go`:

```go
package httpapi

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/catalog"
)

type CatalogHandler struct {
	service *catalog.Service
	tempRoot string
}

func NewCatalogHandler(service *catalog.Service, tempRoot string) *CatalogHandler {
	return &CatalogHandler{service: service, tempRoot: tempRoot}
}

func (h *CatalogHandler) Upload(c *gin.Context) {
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
	if err := os.MkdirAll(h.tempRoot, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tempPath := filepath.Join(h.tempRoot, file.Filename+".upload")
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	book, err := h.service.ImportUploadedFile(c.Request.Context(), catalog.UploadInput{
		AdminUsername: claims.Username,
		OriginalFilename: file.Filename,
		TempPath: tempPath,
	})
	if err != nil {
		_ = os.Remove(tempPath)
		status := http.StatusInternalServerError
		if errors.Is(err, catalog.ErrUnsupportedFormat) || errors.Is(err, catalog.ErrDuplicateBook) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": bookJSON(book)})
}

func bookJSON(book catalog.Book) gin.H {
	return gin.H{
		"id": book.BookKey,
		"title": book.Title,
		"author": book.Author,
		"format": book.Format,
		"filename": book.Filename,
		"storage_path": book.StoragePath,
		"file_size": book.FileSize,
		"content_sha1": book.ContentSHA1,
		"chapter_count": book.ChapterCount,
		"status": book.Status,
	}
}
```

- [ ] **Step 2: Wire catalog service in router**

Modify `service/internal/httpapi/router.go`:

```go
catalogStorage := catalog.NewLocalStorage(deps.Config.Catalog.Storage.Root, deps.Config.Catalog.Storage.TempRoot)
catalogService := catalog.NewService(
	catalog.NewMySQLStore(deps.DB),
	catalogStorage,
	[]catalog.Importer{catalog.TXTImporter{}, catalog.EPUBImporter{}},
)
catalogHandler := NewCatalogHandler(catalogService, deps.Config.Catalog.Storage.TempRoot)
adminCatalog := admin.Group("/catalog")
adminCatalog.Use(AdminRequired(authService))
adminCatalog.POST("/books/upload", catalogHandler.Upload)
```

Add import:

```go
"github.com/oops-reader/oops-reader-manager/service/internal/catalog"
```

- [ ] **Step 3: Run service tests**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add service
git commit -m "feat: expose manager catalog upload endpoint"
```

## Phase 4: Reader Backend Read Path Adaptation

### Task 10: Adapt Backend Catalog Model to New Schema

**Repository:** `/home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-backend`

**Files:**
- Modify: `internal/catalog/service.go`
- Modify: `internal/catalog/mysql_store.go`
- Modify: `internal/transport/http/handlers/catalog.go`
- Add: `migrations/010_extend_catalog_books_for_manager.sql`

- [ ] **Step 1: Add migration copied from manager**

```bash
cp /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service/migrations/002_extend_catalog_books_for_manager.sql \
  /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-backend/migrations/010_extend_catalog_books_for_manager.sql
```

- [ ] **Step 2: Extend backend `catalog.Book`**

Modify `internal/catalog/service.go` `Book` struct to include:

```go
Format           string
Description      string
CoverStoragePath string
Status           string
```

- [ ] **Step 3: Update backend MySQL SELECTs**

Modify `internal/catalog/mysql_store.go` SELECT columns:

```sql
SELECT book_key, title, author, description, format, filename, storage_path, cover_storage_path,
language, chapter_count, file_size, content_sha1, status
FROM catalog_books
```

Use `WHERE status = 'active'`.

- [ ] **Step 4: Update scanner**

Modify scanner to read nullable description, format, cover path, and status:

```go
var description, format, coverPath, status sql.NullString
```

Set defaults:

```go
book.Format = format.String
if book.Format == "" {
	book.Format = "epub"
}
book.Status = status.String
book.Description = description.String
book.CoverStoragePath = coverPath.String
```

- [ ] **Step 5: Update catalog JSON**

Modify `internal/transport/http/handlers/catalog.go` `bookJSON` to include:

```go
"description": book.Description,
"format": book.Format,
```

- [ ] **Step 6: Run backend catalog tests**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-backend
go test ./internal/catalog ./cmd/api
```

Expected: PASS.

- [ ] **Step 7: Commit backend change**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-backend
git add internal/catalog internal/transport/http/handlers/catalog.go migrations/010_extend_catalog_books_for_manager.sql
git commit -m "feat: read manager catalog schema"
```

### Task 11: Add TXT Read Support to Reader Backend

**Repository:** `/home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-backend`

**Files:**
- Create: `internal/content/txt.go`
- Create: `internal/content/txt_test.go`
- Modify: `internal/catalog/service.go`

- [ ] **Step 1: Add TXT parser test**

Create `internal/content/txt_test.go`:

```go
package content

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTXTChapters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "book.txt")
	body := "第一章 开始\n内容一\n第二章 继续\n内容二"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	book, err := ParseTXT(path)
	if err != nil {
		t.Fatalf("parse txt: %v", err)
	}
	if len(book.Chapters) != 2 {
		t.Fatalf("chapters = %d", len(book.Chapters))
	}
}
```

- [ ] **Step 2: Implement TXT parser**

Create `internal/content/txt.go`:

```go
package content

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var txtHeadingPattern = regexp.MustCompile(`(?m)^(第[一二三四五六七八九十百千万0-9]+[章节回卷部].*|Chapter\s+[0-9]+.*)$`)

func ParseTXT(filePath string) (*Book, error) {
	body, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	book := &Book{Title: strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))}
	matches := txtHeadingPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		book.Chapters = []Chapter{{ID: "ch-1", Title: "正文", Text: text}}
		return book, nil
	}
	for idx, match := range matches {
		start := match[0]
		end := len(text)
		if idx+1 < len(matches) {
			end = matches[idx+1][0]
		}
		chunk := strings.TrimSpace(text[start:end])
		lines := strings.SplitN(chunk, "\n", 2)
		title := strings.TrimSpace(lines[0])
		book.Chapters = append(book.Chapters, Chapter{
			ID: fmt.Sprintf("ch-%d", idx+1),
			Title: title,
			Text: chunk,
		})
	}
	return book, nil
}
```

- [ ] **Step 3: Route parse by format**

Modify `internal/catalog/service.go` `parseBook`:

```go
var parsed *content.Book
switch strings.ToLower(book.Format) {
case "", "epub":
	parsed, err = content.ParseEPUB(book.Path)
case "txt":
	parsed, err = content.ParseTXT(book.Path)
default:
	return Book{}, nil, fmt.Errorf("unsupported format: %s", book.Format)
}
```

- [ ] **Step 4: Make cover return not found for TXT**

Modify `GetCover`:

```go
if strings.EqualFold(book.Format, "txt") {
	return nil, fmt.Errorf("%w: cover for %s", ErrNotFound, id)
}
```

- [ ] **Step 5: Run backend tests**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-backend
go test ./internal/content ./internal/catalog ./cmd/api
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-backend
git add internal/content internal/catalog/service.go
git commit -m "feat: support txt catalog reads"
```

## Phase 5: Manager Web Skeleton

### Task 12: Create React Vite App

**Files:**
- Create under: `web/`

- [ ] **Step 1: Scaffold Vite React TypeScript**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
npm create vite@latest web -- --template react-ts
```

Expected: `web/package.json` exists.

- [ ] **Step 2: Install Ant Design and router dependencies**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/web
npm install antd @ant-design/icons react-router-dom
npm install -D vitest @testing-library/react @testing-library/jest-dom jsdom
```

- [ ] **Step 3: Add scripts in `web/package.json`**

Ensure scripts include:

```json
{
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "lint": "eslint .",
    "test": "vitest run",
    "preview": "vite preview"
  }
}
```

- [ ] **Step 4: Verify web build**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/web
npm run build
```

Expected: PASS and `dist/` exists.

- [ ] **Step 5: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add web
git commit -m "feat: scaffold manager web"
```

### Task 13: Add Web API Client and Auth Store

**Files:**
- Create: `web/src/api/http.ts`
- Create: `web/src/api/auth.ts`
- Create: `web/src/app/authStore.ts`

- [ ] **Step 1: Create HTTP client**

Create `web/src/api/http.ts`:

```ts
const API_BASE_URL = import.meta.env.VITE_MANAGER_API_BASE_URL ?? 'http://localhost:8090';

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = localStorage.getItem('manager_access_token');
  const headers = new Headers(init.headers);
  headers.set('Content-Type', 'application/json');
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  const response = await fetch(`${API_BASE_URL}${path}`, { ...init, headers });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(body.error ?? `Request failed: ${response.status}`);
  }
  return body.data as T;
}
```

- [ ] **Step 2: Create auth API**

Create `web/src/api/auth.ts`:

```ts
import { request } from './http';

export interface LoginResponse {
  access_token: string;
}

export interface CurrentAdmin {
  username: string;
}

export function login(username: string, password: string) {
  return request<LoginResponse>('/admin/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
}

export function me() {
  return request<CurrentAdmin>('/admin/auth/me');
}
```

- [ ] **Step 3: Create auth store**

Create `web/src/app/authStore.ts`:

```ts
export function getAccessToken() {
  return localStorage.getItem('manager_access_token');
}

export function setAccessToken(token: string) {
  localStorage.setItem('manager_access_token', token);
}

export function clearAccessToken() {
  localStorage.removeItem('manager_access_token');
}
```

- [ ] **Step 4: Run web tests/build**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/web
npm run build
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add web/src
git commit -m "feat: add manager web api client"
```

### Task 14: Add Login and App Layout

**Files:**
- Replace: `web/src/App.tsx`
- Create: `web/src/pages/LoginPage.tsx`
- Create: `web/src/pages/DashboardPage.tsx`
- Create: `web/src/app/AppLayout.tsx`

- [ ] **Step 1: Create login page**

Create `web/src/pages/LoginPage.tsx`:

```tsx
import { Button, Card, Form, Input, Typography, message } from 'antd';
import { useNavigate } from 'react-router-dom';
import { login } from '../api/auth';
import { setAccessToken } from '../app/authStore';

export function LoginPage() {
  const navigate = useNavigate();

  async function handleFinish(values: { username: string; password: string }) {
    try {
      const result = await login(values.username, values.password);
      setAccessToken(result.access_token);
      navigate('/');
    } catch (error) {
      message.error(error instanceof Error ? error.message : '登录失败');
    }
  }

  return (
    <main style={{ minHeight: '100vh', display: 'grid', placeItems: 'center', background: '#f5f7fb' }}>
      <Card style={{ width: 360 }}>
        <Typography.Title level={3}>Oops Reader Manager</Typography.Title>
        <Form layout="vertical" onFinish={handleFinish}>
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input autoComplete="username" />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" block>
            登录
          </Button>
        </Form>
      </Card>
    </main>
  );
}
```

- [ ] **Step 2: Create dashboard page**

Create `web/src/pages/DashboardPage.tsx`:

```tsx
import { Card, Col, Row, Statistic } from 'antd';

export function DashboardPage() {
  return (
    <Row gutter={16}>
      <Col span={6}><Card><Statistic title="用户" value={0} /></Card></Col>
      <Col span={6}><Card><Statistic title="帖子" value={0} /></Card></Col>
      <Col span={6}><Card><Statistic title="书籍" value={0} /></Card></Col>
      <Col span={6}><Card><Statistic title="待处理" value={0} /></Card></Col>
    </Row>
  );
}
```

- [ ] **Step 3: Create layout**

Create `web/src/app/AppLayout.tsx`:

```tsx
import { BookOutlined, DashboardOutlined, FileTextOutlined, TeamOutlined } from '@ant-design/icons';
import { Layout, Menu } from 'antd';
import { Link, Outlet, useLocation } from 'react-router-dom';

export function AppLayout() {
  const location = useLocation();
  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Layout.Sider theme="light">
        <div style={{ height: 56, display: 'grid', placeItems: 'center', fontWeight: 700 }}>Oops Manager</div>
        <Menu
          selectedKeys={[location.pathname]}
          items={[
            { key: '/', icon: <DashboardOutlined />, label: <Link to="/">总览</Link> },
            { key: '/users', icon: <TeamOutlined />, label: <Link to="/users">用户</Link> },
            { key: '/posts', icon: <FileTextOutlined />, label: <Link to="/posts">帖子</Link> },
            { key: '/books', icon: <BookOutlined />, label: <Link to="/books">书籍</Link> },
          ]}
        />
      </Layout.Sider>
      <Layout>
        <Layout.Content style={{ padding: 24 }}>
          <Outlet />
        </Layout.Content>
      </Layout>
    </Layout>
  );
}
```

- [ ] **Step 4: Replace app routes**

Replace `web/src/App.tsx`:

```tsx
import { Navigate, Route, BrowserRouter, Routes } from 'react-router-dom';
import { AppLayout } from './app/AppLayout';
import { getAccessToken } from './app/authStore';
import { DashboardPage } from './pages/DashboardPage';
import { LoginPage } from './pages/LoginPage';

function RequireAuth({ children }: { children: JSX.Element }) {
  if (!getAccessToken()) {
    return <Navigate to="/login" replace />;
  }
  return children;
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/" element={<RequireAuth><AppLayout /></RequireAuth>}>
          <Route index element={<DashboardPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
```

- [ ] **Step 5: Build web**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/web
npm run build
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add web/src
git commit -m "feat: add manager login and layout"
```

### Task 15: Add Books Page and Upload UI

**Files:**
- Create: `web/src/api/catalog.ts`
- Create: `web/src/pages/BooksPage.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Create catalog API client**

Create `web/src/api/catalog.ts`:

```ts
const API_BASE_URL = import.meta.env.VITE_MANAGER_API_BASE_URL ?? 'http://localhost:8090';

export interface CatalogBook {
  id: string;
  title: string;
  author: string;
  format: string;
  filename: string;
  file_size: number;
  chapter_count: number;
  status: string;
}

export async function uploadBook(file: File): Promise<CatalogBook> {
  const token = localStorage.getItem('manager_access_token');
  const form = new FormData();
  form.append('file', file);
  const response = await fetch(`${API_BASE_URL}/admin/catalog/books/upload`, {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    body: form,
  });
  const body = await response.json();
  if (!response.ok) {
    throw new Error(body.error ?? '上传失败');
  }
  return body.data as CatalogBook;
}
```

- [ ] **Step 2: Create books page**

Create `web/src/pages/BooksPage.tsx`:

```tsx
import { UploadOutlined } from '@ant-design/icons';
import { Button, Card, Space, Table, Tag, Upload, message } from 'antd';
import type { UploadProps } from 'antd';
import { useState } from 'react';
import { CatalogBook, uploadBook } from '../api/catalog';

export function BooksPage() {
  const [books, setBooks] = useState<CatalogBook[]>([]);
  const [uploading, setUploading] = useState(false);

  const props: UploadProps = {
    accept: '.epub,.txt',
    showUploadList: false,
    beforeUpload: async (file) => {
      setUploading(true);
      try {
        const book = await uploadBook(file);
        setBooks((current) => [book, ...current]);
        message.success('上传成功，书籍已进入草稿');
      } catch (error) {
        message.error(error instanceof Error ? error.message : '上传失败');
      } finally {
        setUploading(false);
      }
      return false;
    },
  };

  return (
    <Card
      title="在线书籍"
      extra={
        <Upload {...props}>
          <Button icon={<UploadOutlined />} loading={uploading}>上传 EPUB/TXT</Button>
        </Upload>
      }
    >
      <Table
        rowKey="id"
        dataSource={books}
        columns={[
          { title: '书名', dataIndex: 'title' },
          { title: '作者', dataIndex: 'author' },
          { title: '格式', dataIndex: 'format', render: (value) => <Tag>{value}</Tag> },
          { title: '章节', dataIndex: 'chapter_count' },
          { title: '状态', dataIndex: 'status', render: (value) => <Tag color={value === 'draft' ? 'gold' : 'green'}>{value}</Tag> },
          {
            title: '操作',
            render: () => <Space><Button size="small">编辑</Button><Button size="small">发布</Button></Space>,
          },
        ]}
      />
    </Card>
  );
}
```

- [ ] **Step 3: Add books route**

Modify `web/src/App.tsx`:

```tsx
import { BooksPage } from './pages/BooksPage';
```

Add child route:

```tsx
<Route path="books" element={<BooksPage />} />
```

- [ ] **Step 4: Build web**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/web
npm run build
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add web/src
git commit -m "feat: add manager book upload page"
```

## Phase 6: Management Lists and Moderation

### Task 16: Add Manager User APIs

**Files:**
- Create: `service/internal/usersadmin/store.go`
- Create: `service/internal/usersadmin/mysql_store.go`
- Create: `service/internal/httpapi/users_handler.go`
- Modify: `service/internal/httpapi/router.go`

- [ ] **Step 1: Create user admin store interface**

Create `service/internal/usersadmin/store.go`:

```go
package usersadmin

import "context"

type User struct {
	ID string `json:"id"`
	Email string `json:"email"`
	DisplayName string `json:"display_name"`
	Status string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type Store interface {
	List(ctx context.Context, query string, limit, offset int) ([]User, int, error)
	UpdateStatus(ctx context.Context, id string, status string) error
}
```

- [ ] **Step 2: Implement MySQL user store**

Create `service/internal/usersadmin/mysql_store.go`:

```go
package usersadmin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) List(ctx context.Context, query string, limit, offset int) ([]User, int, error) {
	if limit < 1 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	where := "WHERE 1=1"
	args := []any{}
	query = strings.TrimSpace(query)
	if query != "" {
		where += " AND (CAST(id AS CHAR) = ? OR LOWER(email) LIKE ? OR LOWER(nickname) LIKE ?)"
		like := "%" + strings.ToLower(query) + "%"
		args = append(args, query, like, like)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, `
SELECT id, COALESCE(email, ''), nickname, account_status, created_at
FROM users `+where+`
ORDER BY created_at DESC
LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate users: %w", err)
	}
	return users, total, nil
}

func (s *MySQLStore) UpdateStatus(ctx context.Context, id string, status string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE users
SET account_status = ?, status = CASE WHEN ? = 'active' THEN 1 WHEN ? = 'frozen' THEN 2 ELSE 3 END
WHERE id = ?`, status, status, status, id)
	if err != nil {
		return fmt.Errorf("update user status: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Add users handler**

Create `service/internal/httpapi/users_handler.go` with:

```go
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/usersadmin"
)

type UsersHandler struct {
	store usersadmin.Store
}

func NewUsersHandler(store usersadmin.Store) *UsersHandler {
	return &UsersHandler{store: store}
}

func (h *UsersHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }
	users, total, err := h.store.List(c.Request.Context(), c.Query("q"), pageSize, (page-1)*pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users, "pagination": gin.H{"page": page, "page_size": pageSize, "total": total}})
}
```

- [ ] **Step 4: Add route**

Modify router:

```go
usersHandler := NewUsersHandler(usersadmin.NewMySQLStore(deps.DB))
admin.GET("/users", AdminRequired(authService), usersHandler.List)
```

- [ ] **Step 5: Run service tests**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add service
git commit -m "feat: add manager user listing api"
```

### Task 17: Add Community Moderation APIs

**Files:**
- Create: `service/internal/communityadmin/store.go`
- Create: `service/internal/communityadmin/mysql_store.go`
- Create: `service/internal/httpapi/community_handler.go`
- Modify: `service/internal/httpapi/router.go`

- [ ] **Step 1: Create community admin store contract**

Create `service/internal/communityadmin/store.go`:

```go
package communityadmin

import "context"

type Thread struct {
	ID string `json:"id"`
	BoardID string `json:"board_id"`
	Title string `json:"title"`
	AuthorID string `json:"author_id"`
	Status string `json:"status"`
	CreatedAt string `json:"created_at"`
	CommentCount int `json:"comment_count"`
}

type Comment struct {
	ID string `json:"id"`
	ThreadID string `json:"thread_id"`
	AuthorID string `json:"author_id"`
	Body string `json:"body"`
	Status string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type Store interface {
	ListThreads(ctx context.Context, query string, limit, offset int) ([]Thread, int, error)
	UpdateThreadStatus(ctx context.Context, id string, status string) error
	ListComments(ctx context.Context, threadID string, limit, offset int) ([]Comment, int, error)
	UpdateCommentStatus(ctx context.Context, id string, status string) error
}
```

- [ ] **Step 2: Implement MySQL store**

Create `service/internal/communityadmin/mysql_store.go`:

```go
package communityadmin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) ListThreads(ctx context.Context, query string, limit, offset int) ([]Thread, int, error) {
	if limit < 1 {
		limit = 20
	}
	where := "WHERE status <> 'deleted'"
	args := []any{}
	query = strings.TrimSpace(query)
	if query != "" {
		where += " AND (LOWER(title) LIKE ? OR LOWER(content) LIKE ? OR id = ?)"
		like := "%" + strings.ToLower(query) + "%"
		args = append(args, like, like, query)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM community_threads "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count community threads: %w", err)
	}
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, `
SELECT id, board_id, title, CAST(user_id AS CHAR), status, created_at, comment_count
FROM community_threads `+where+`
ORDER BY updated_at DESC
LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list community threads: %w", err)
	}
	defer rows.Close()
	threads := []Thread{}
	for rows.Next() {
		var thread Thread
		if err := rows.Scan(&thread.ID, &thread.BoardID, &thread.Title, &thread.AuthorID, &thread.Status, &thread.CreatedAt, &thread.CommentCount); err != nil {
			return nil, 0, fmt.Errorf("scan community thread: %w", err)
		}
		threads = append(threads, thread)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate community threads: %w", err)
	}
	return threads, total, nil
}

func (s *MySQLStore) UpdateThreadStatus(ctx context.Context, id string, status string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE community_threads SET status = ?, updated_at = NOW() WHERE id = ?", status, id)
	if err != nil {
		return fmt.Errorf("update community thread status: %w", err)
	}
	return nil
}

func (s *MySQLStore) ListComments(ctx context.Context, threadID string, limit, offset int) ([]Comment, int, error) {
	if limit < 1 {
		limit = 20
	}
	where := "WHERE status <> 'deleted'"
	args := []any{}
	if strings.TrimSpace(threadID) != "" {
		where += " AND thread_id = ?"
		args = append(args, threadID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM community_comments "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count community comments: %w", err)
	}
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, `
SELECT id, thread_id, CAST(user_id AS CHAR), content, status, created_at
FROM community_comments `+where+`
ORDER BY created_at DESC
LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list community comments: %w", err)
	}
	defer rows.Close()
	comments := []Comment{}
	for rows.Next() {
		var comment Comment
		if err := rows.Scan(&comment.ID, &comment.ThreadID, &comment.AuthorID, &comment.Body, &comment.Status, &comment.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan community comment: %w", err)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate community comments: %w", err)
	}
	return comments, total, nil
}

func (s *MySQLStore) UpdateCommentStatus(ctx context.Context, id string, status string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE community_comments SET status = ?, updated_at = NOW() WHERE id = ?", status, id)
	if err != nil {
		return fmt.Errorf("update community comment status: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Add HTTP handlers**

Create `service/internal/httpapi/community_handler.go`:

```go
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/communityadmin"
)

type CommunityHandler struct {
	store communityadmin.Store
}

func NewCommunityHandler(store communityadmin.Store) *CommunityHandler {
	return &CommunityHandler{store: store}
}

type updateStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func (h *CommunityHandler) ListThreads(c *gin.Context) {
	page, pageSize := pageParams(c)
	threads, total, err := h.store.ListThreads(c.Request.Context(), c.Query("q"), pageSize, (page-1)*pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": threads, "pagination": gin.H{"page": page, "page_size": pageSize, "total": total}})
}

func (h *CommunityHandler) UpdateThreadStatus(c *gin.Context) {
	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.store.UpdateThreadStatus(c.Request.Context(), c.Param("id"), req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": c.Param("id"), "status": req.Status}})
}

func (h *CommunityHandler) ListComments(c *gin.Context) {
	page, pageSize := pageParams(c)
	comments, total, err := h.store.ListComments(c.Request.Context(), c.Query("thread_id"), pageSize, (page-1)*pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": comments, "pagination": gin.H{"page": page, "page_size": pageSize, "total": total}})
}

func (h *CommunityHandler) UpdateCommentStatus(c *gin.Context) {
	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.store.UpdateCommentStatus(c.Request.Context(), c.Param("id"), req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": c.Param("id"), "status": req.Status}})
}

func pageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return page, pageSize
}
```

- [ ] **Step 4: Wire routes**

Add:

```go
communityHandler := NewCommunityHandler(communityadmin.NewMySQLStore(deps.DB))
admin.GET("/community/threads", AdminRequired(authService), communityHandler.ListThreads)
admin.PATCH("/community/threads/:id/status", AdminRequired(authService), communityHandler.UpdateThreadStatus)
admin.GET("/community/comments", AdminRequired(authService), communityHandler.ListComments)
admin.PATCH("/community/comments/:id/status", AdminRequired(authService), communityHandler.UpdateCommentStatus)
```

- [ ] **Step 5: Run tests**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add service
git commit -m "feat: add manager community moderation api"
```

## Phase 7: Frontend Lists

### Task 18: Add Users Page

**Files:**
- Create: `web/src/api/users.ts`
- Create: `web/src/pages/UsersPage.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Create users API client**

Create `web/src/api/users.ts`:

```ts
import { request } from './http';

export interface User {
  id: string;
  email: string;
  display_name: string;
  status: string;
  created_at: string;
}

export function listUsers() {
  return request<User[]>('/admin/users');
}
```

- [ ] **Step 2: Create users page**

Create `web/src/pages/UsersPage.tsx`:

```tsx
import { Card, Table, Tag } from 'antd';
import { useEffect, useState } from 'react';
import { User, listUsers } from '../api/users';

export function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  useEffect(() => {
    listUsers().then(setUsers);
  }, []);
  return (
    <Card title="用户管理">
      <Table
        rowKey="id"
        dataSource={users}
        columns={[
          { title: '邮箱', dataIndex: 'email' },
          { title: '昵称', dataIndex: 'display_name' },
          { title: '状态', dataIndex: 'status', render: (value) => <Tag>{value}</Tag> },
          { title: '创建时间', dataIndex: 'created_at' },
        ]}
      />
    </Card>
  );
}
```

- [ ] **Step 3: Add route**

Modify `web/src/App.tsx`:

```tsx
import { UsersPage } from './pages/UsersPage';
```

Add:

```tsx
<Route path="users" element={<UsersPage />} />
```

- [ ] **Step 4: Build**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/web
npm run build
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add web/src
git commit -m "feat: add manager users page"
```

### Task 19: Add Posts Page

**Files:**
- Create: `web/src/api/community.ts`
- Create: `web/src/pages/PostsPage.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Create community API client**

Create `web/src/api/community.ts`:

```ts
import { request } from './http';

export interface Thread {
  id: string;
  board_id: string;
  title: string;
  author_id: string;
  status: string;
  created_at: string;
  comment_count: number;
}

export function listThreads() {
  return request<Thread[]>('/admin/community/threads');
}
```

- [ ] **Step 2: Create posts page**

Create `web/src/pages/PostsPage.tsx`:

```tsx
import { Card, Table, Tag } from 'antd';
import { useEffect, useState } from 'react';
import { Thread, listThreads } from '../api/community';

export function PostsPage() {
  const [threads, setThreads] = useState<Thread[]>([]);
  useEffect(() => {
    listThreads().then(setThreads);
  }, []);
  return (
    <Card title="帖子管理">
      <Table
        rowKey="id"
        dataSource={threads}
        columns={[
          { title: '标题', dataIndex: 'title' },
          { title: '版块', dataIndex: 'board_id' },
          { title: '作者', dataIndex: 'author_id' },
          { title: '评论', dataIndex: 'comment_count' },
          { title: '状态', dataIndex: 'status', render: (value) => <Tag>{value}</Tag> },
          { title: '创建时间', dataIndex: 'created_at' },
        ]}
      />
    </Card>
  );
}
```

- [ ] **Step 3: Add route**

Modify `web/src/App.tsx`:

```tsx
import { PostsPage } from './pages/PostsPage';
```

Add:

```tsx
<Route path="posts" element={<PostsPage />} />
```

- [ ] **Step 4: Build**

Run:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/web
npm run build
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager
git add web/src
git commit -m "feat: add manager posts page"
```

## Final Verification

- [ ] **Step 1: Run manager service tests**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run manager web build**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/web
npm run build
```

Expected: PASS.

- [ ] **Step 3: Run reader backend tests**

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-backend
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Manual smoke test**

Run manager service:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/service
go run ./cmd/api
```

Run manager web:

```bash
cd /home/ut003607@uos/Workspace/PrivateCode/Oops/reader/oops-reader-manager/web
npm run dev
```

Expected:

- Login page opens.
- Login succeeds with configured admin credentials.
- Books page uploads TXT and receives `status = draft`.
- Reader backend catalog does not list draft books.
- After status is changed to `active`, reader backend lists the book.
