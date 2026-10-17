//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	systempostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/system/adapters/outbound/postgres"
)

func openTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal("cannot create test database pool")
	}

	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatal("cannot connect to test database")
	}

	return pool
}

func TestPostgresDatabaseProbe(t *testing.T) {
	pool := openTestDB(t)

	probe := systempostgres.NewDatabaseProbe(pool)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := probe.Ping(ctx); err != nil {
		t.Fatalf("database probe failed: %v", err)
	}
}

func TestOrganizationMigration(t *testing.T) {
	pool := openTestDB(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	var exists bool

	err := pool.QueryRow(
		ctx,
		"SELECT to_regclass('public.organizations') IS NOT NULL",
	).Scan(&exists)

	if err != nil {
		t.Fatalf("check organizations table: %v", err)
	}

	if !exists {
		t.Fatal("organizations table does not exist")
	}

	tests := []struct {
		name           string
		organization   string
		slug           string
		wantConstraint string
	}{
		{
			name:         "valid organization",
			organization: "ScopeGuard Test",
			slug:         "scopeguard-ci-test",
		},
		{
			name:           "blank organization name",
			organization:   "   ",
			slug:           "scopeguard-invalid-name",
			wantConstraint: "organizations_name_not_blank",
		},
		{
			name:           "invalid organization slug",
			organization:   "Test Organization",
			slug:           "Invalid_Slug",
			wantConstraint: "organizations_slug_format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatalf("begin transaction: %v", err)
			}
			defer func() {
				_ = tx.Rollback(context.Background())
			}()

			_, err = tx.Exec(
				ctx,
				`INSERT INTO organizations (name, slug)
                 VALUES ($1, $2)`,
				tt.organization,
				tt.slug,
			)

			if tt.wantConstraint == "" {
				if err != nil {
					t.Fatalf("unexpected insert error: %v", err)
				}
				return
			}

			var pgErr *pgconn.PgError

			if !errors.As(err, &pgErr) {
				t.Fatalf("expected PostgreSQL constraint error, got %v", err)
			}

			if pgErr.Code != "23514" ||
				pgErr.ConstraintName != tt.wantConstraint {
				t.Fatalf(
					"expected constraint %s, got %s (%s)",
					tt.wantConstraint,
					pgErr.ConstraintName,
					pgErr.Code,
				)
			}
		})
	}
}
