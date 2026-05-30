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
