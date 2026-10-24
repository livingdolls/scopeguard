//go:build integration

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	permissiondomain "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

func TestPermissionCatalogSeed(t *testing.T) {
	pool := openTestDB(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	for _, code := range permissiondomain.BuiltInPermissionCodes() {
		expected, err := permissiondomain.ParsePermission(code)
		if err != nil {
			t.Fatal(err)
		}

		var resource string
		var action string

		err = pool.QueryRow(
			ctx,
			`SELECT resource, action
             FROM permissions
             WHERE code = $1`,
			code,
		).Scan(&resource, &action)

		if err != nil {
			t.Fatalf(
				"permission %q is missing: %v",
				code,
				err,
			)
		}

		if resource != expected.Resource() ||
			action != expected.Action() {
			t.Fatalf(
				"permission %q has inconsistent columns",
				code,
			)
		}
	}
}

func TestPermissionCatalogConstraints(t *testing.T) {
	pool := openTestDB(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	cases := []struct {
		name        string
		code        string
		resource    string
		action      string
		description string
	}{
		{
			name:        "uppercase resource",
			code:        "Member:read",
			resource:    "Member",
			action:      "read",
			description: "Invalid case",
		},
		{
			name:        "invalid action",
			code:        "itest:INVITE",
			resource:    "itest",
			action:      "INVITE",
			description: "Invalid action",
		},
		{
			name:        "mismatched code",
			code:        "itest:read",
			resource:    "itest",
			action:      "update",
			description: "Invalid mapping",
		},
		{
			name:        "blank description",
			code:        "itest:read",
			resource:    "itest",
			action:      "read",
			description: "   ",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pool.Exec(
				ctx,
				`INSERT INTO permissions (
                    code,
                    resource,
                    action,
                    description
                ) VALUES ($1, $2, $3, $4)`,
				tt.code,
				tt.resource,
				tt.action,
				tt.description,
			)

			var pgErr *pgconn.PgError

			if !errors.As(err, &pgErr) ||
				pgErr.Code != "23514" {
				t.Fatalf(
					"expected CHECK constraint violation, got %v",
					err,
				)
			}
		})
	}
}

func TestPermissionCatalogUniqueness(t *testing.T) {
	pool := openTestDB(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	resource := "itest_" + strings.ReplaceAll(
		uuid.NewString(), "-", "",
	)
	code := resource + ":read"

	t.Cleanup(func() {
		cleanupCtx, done := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer done()

		if _, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM permissions WHERE code = $1",
			code,
		); err != nil {
			t.Errorf("cleanup permission: %v", err)
		}
	})

	insert := `
        INSERT INTO permissions (
            code, resource, action, description
        ) VALUES ($1, $2, 'read', 'Test permission')
    `

	_, err := pool.Exec(ctx, insert, code, resource)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, insert, code, resource)

	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) ||
		pgErr.Code != "23505" {
		t.Fatalf(
			"expected unique violation, got %v",
			err,
		)
	}
}
