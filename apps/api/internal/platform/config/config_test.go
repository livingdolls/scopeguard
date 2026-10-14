package config

import "testing"

func TestLoadConfig(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("DB_MAX_CONNS", "10")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPAddr != ":9090" {
		t.Errorf("unexpected HTTPAddr: %s", cfg.HTTPAddr)
	}

	if cfg.DBMaxConnections != 10 {
		t.Errorf("expected 10, got %d", cfg.DBMaxConnections)
	}
}

func TestLoadConfigMissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL")
	}
}

func TestLoadConfigInvalidMaxConns(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("DB_MAX_CONNS", "-5")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid DB_MAX_CONNS")
	}
}
