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
