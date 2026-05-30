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
