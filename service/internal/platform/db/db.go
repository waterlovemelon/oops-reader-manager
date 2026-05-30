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
