package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/adminauth"
	"github.com/oops-reader/oops-reader-manager/service/internal/audit"
	"github.com/oops-reader/oops-reader-manager/service/internal/catalog"
	"github.com/oops-reader/oops-reader-manager/service/internal/communityadmin"
	"github.com/oops-reader/oops-reader-manager/service/internal/config"
	"github.com/oops-reader/oops-reader-manager/service/internal/dashboard"
	"github.com/oops-reader/oops-reader-manager/service/internal/entitlementadmin"
	"github.com/oops-reader/oops-reader-manager/service/internal/importjob"
	"github.com/oops-reader/oops-reader-manager/service/internal/recommendation"
	"github.com/oops-reader/oops-reader-manager/service/internal/usersadmin"
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

	// CORS
	router.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Auth
	authService := adminauth.NewService(adminauth.Config{
		Username:          deps.Config.Admin.Username,
		PasswordHash:      deps.Config.Admin.PasswordHash,
		TokenSecret:       deps.Config.Admin.TokenSecret,
		AccessTokenExpiry: deps.Config.Admin.AccessTokenExpiry,
	})
	authHandler := NewAdminAuthHandler(authService)

	admin := router.Group("/admin")
	admin.POST("/auth/login", authHandler.Login)
	admin.GET("/auth/me", AdminRequired(authService), authHandler.Me)

	// Audit service (shared across handlers)
	auditStore := audit.NewMySQLStore(deps.DB)
	auditSvc := audit.NewService(auditStore)

	// Catalog
	catalogStorage := catalog.NewLocalStorage(deps.Config.Catalog.Storage.Root, deps.Config.Catalog.Storage.TempRoot)
	catalogService := catalog.NewService(
		catalog.NewMySQLStore(deps.DB),
		catalogStorage,
		[]catalog.Importer{catalog.TXTImporter{}, catalog.EPUBImporter{}},
	)
	catalogHandler := NewCatalogHandler(catalogService, deps.Config.Catalog.Storage.TempRoot, auditSvc)
	admin.POST("/catalog/books/upload", AdminRequired(authService), catalogHandler.Upload)
	admin.GET("/catalog/books", AdminRequired(authService), catalogHandler.List)
	admin.GET("/catalog/books/:book_key", AdminRequired(authService), catalogHandler.Get)
	admin.PATCH("/catalog/books/:book_key", AdminRequired(authService), catalogHandler.Update)
	admin.PATCH("/catalog/books/:book_key/status", AdminRequired(authService), catalogHandler.UpdateStatus)
	admin.GET("/catalog/books/:book_key/cover", AdminRequired(authService), catalogHandler.Cover)

	// Import Jobs (async book import)
	importJobStore := importjob.NewMySQLStore(deps.DB)
	importJobSvc := importjob.NewService(importJobStore, catalogService.Store(), catalogStorage)
	importHandler := NewImportHandler(importJobSvc, deps.Config.Catalog.Storage.TempRoot, auditSvc)
	admin.POST("/catalog/import-jobs", AdminRequired(authService), importHandler.CreateJob)
	admin.GET("/catalog/import-jobs", AdminRequired(authService), importHandler.ListJobs)
	admin.GET("/catalog/import-jobs/:job_id", AdminRequired(authService), importHandler.GetJob)
	admin.POST("/catalog/import-jobs/:job_id/retry", AdminRequired(authService), importHandler.RetryJob)
	admin.POST("/catalog/import-jobs/:job_id/cancel", AdminRequired(authService), importHandler.CancelJob)

	// Start import worker in background
	workerCfg := importjob.WorkerConfig{
		Enabled:      true,
		Concurrency:  2,
		PollInterval: 2 * time.Second,
		JobTimeout:   5 * time.Minute,
		MaxAttempts:  3,
	}
	worker := importjob.NewWorker(workerCfg, importJobStore, catalogService, catalogStorage, auditSvc, deps.Logger)
	go worker.Start(context.Background())

	// Users
	usersHandler := NewUsersHandler(usersadmin.NewMySQLStore(deps.DB), auditSvc)
	admin.GET("/users", AdminRequired(authService), usersHandler.List)
	admin.PATCH("/users/:id/status", AdminRequired(authService), usersHandler.UpdateStatus)

	// Community
	communityHandler := NewCommunityHandler(communityadmin.NewMySQLStore(deps.DB), auditSvc)
	admin.GET("/community/threads", AdminRequired(authService), communityHandler.ListThreads)
	admin.PATCH("/community/threads/:id/status", AdminRequired(authService), communityHandler.UpdateThreadStatus)
	admin.GET("/community/comments", AdminRequired(authService), communityHandler.ListComments)
	admin.PATCH("/community/comments/:id/status", AdminRequired(authService), communityHandler.UpdateCommentStatus)

	// Audit logs
	auditHandler := NewAuditHandler(auditStore)
	admin.GET("/audit/logs", AdminRequired(authService), auditHandler.List)

	// Dashboard
	dashboardHandler := NewDashboardHandler(dashboard.NewMySQLStore(deps.DB))
	admin.GET("/dashboard/summary", AdminRequired(authService), dashboardHandler.Summary)

	// Entitlements
	entitlementHandler := NewEntitlementHandler(entitlementadmin.NewMySQLStore(deps.DB), auditSvc)
	admin.GET("/users/:id/entitlements", AdminRequired(authService), entitlementHandler.ListByUser)
	admin.POST("/users/:id/entitlements", AdminRequired(authService), entitlementHandler.Create)
	admin.POST("/entitlements/:id/revoke", AdminRequired(authService), entitlementHandler.Revoke)
	admin.POST("/entitlements/:id/extend", AdminRequired(authService), entitlementHandler.Extend)

	// Recommendations
	recStore := recommendation.NewMySQLStore(deps.DB)
	recSvc := recommendation.NewService(recStore)
	recHandler := NewRecommendationHandler(recSvc, auditSvc)
	admin.GET("/recommendations/books", AdminRequired(authService), recHandler.List)
	admin.GET("/recommendations/books/:id", AdminRequired(authService), recHandler.Get)
	admin.POST("/recommendations/books", AdminRequired(authService), recHandler.Create)
	admin.PATCH("/recommendations/books/:id", AdminRequired(authService), recHandler.Update)
	admin.DELETE("/recommendations/books/:id", AdminRequired(authService), recHandler.Delete)

	return router
}
