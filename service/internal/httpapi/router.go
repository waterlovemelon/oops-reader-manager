package httpapi

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-manager/service/internal/adminauth"
	"github.com/oops-reader/oops-reader-manager/service/internal/catalog"
	"github.com/oops-reader/oops-reader-manager/service/internal/communityadmin"
	"github.com/oops-reader/oops-reader-manager/service/internal/config"
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

	// Catalog
	catalogStorage := catalog.NewLocalStorage(deps.Config.Catalog.Storage.Root, deps.Config.Catalog.Storage.TempRoot)
	catalogService := catalog.NewService(
		catalog.NewMySQLStore(deps.DB),
		catalogStorage,
		[]catalog.Importer{catalog.TXTImporter{}, catalog.EPUBImporter{}},
	)
	catalogHandler := NewCatalogHandler(catalogService, deps.Config.Catalog.Storage.TempRoot)
	admin.POST("/catalog/books/upload", AdminRequired(authService), catalogHandler.Upload)

	// Users
	usersHandler := NewUsersHandler(usersadmin.NewMySQLStore(deps.DB))
	admin.GET("/users", AdminRequired(authService), usersHandler.List)

	// Community
	communityHandler := NewCommunityHandler(communityadmin.NewMySQLStore(deps.DB))
	admin.GET("/community/threads", AdminRequired(authService), communityHandler.ListThreads)
	admin.PATCH("/community/threads/:id/status", AdminRequired(authService), communityHandler.UpdateThreadStatus)
	admin.GET("/community/comments", AdminRequired(authService), communityHandler.ListComments)
	admin.PATCH("/community/comments/:id/status", AdminRequired(authService), communityHandler.UpdateCommentStatus)

	return router
}
