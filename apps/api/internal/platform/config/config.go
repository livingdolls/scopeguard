package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	DBMaxConnections int32
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:         ":8080",
		DBMaxConnections: 5,
	}

	if addr := strings.TrimSpace(os.Getenv("HTTP_ADDR")); addr != "" {
		cfg.HTTPAddr = addr
	}

	cfg.DatabaseURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("database url is required")
	}

	if value := os.Getenv("DB_MAX_CONNECTIONS"); value != "" {
		maxConnections, err := strconv.ParseInt(value, 10, 32)

		if err != nil || maxConnections < 1 || maxConnections > 100 {
			return Config{}, fmt.Errorf("DB_MAX_CONNECTIONS must be between 1 and 100")
		}

		cfg.DBMaxConnections = int32(maxConnections)
	}

	return cfg, nil
}
