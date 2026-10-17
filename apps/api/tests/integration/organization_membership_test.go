//go:build integration

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func beginMembershipTestTx(t *testing.T) (
	context.Context,
	pgx.Tx,
) {
	t.Helper()

	pool := openTestDB(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	t.Cleanup(cancel)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}

	t.Cleanup(func() {
		rollbackCtx, done := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer done()

		err := tx.Rollback(rollbackCtx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback transaction: %v", err)
		}
	})

	return ctx, tx
}

func seedMembershipRecords(
	t *testing.T,
	ctx context.Context,
	tx pgx.Tx,
) (string, string) {
	t.Helper()

	userID := uuid.NewString()
	organizationID := uuid.NewString()

	email := "user-" + uuid.NewString() + "@example.test"
	slug := "org-" + strings.ReplaceAll(
		uuid.NewString(), "-", "",
	)

	_, err := tx.Exec(
		ctx,
		"INSERT INTO users (id, email) VALUES ($1, $2)",
		userID,
		email,
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	_, err = tx.Exec(
		ctx,
		`INSERT INTO organizations (id, name, slug)
         VALUES ($1, $2, $3)`,
		organizationID,
		"Test Organization",
		slug,
	)
	if err != nil {
		t.Fatalf("insert organization: %v", err)
	}

	return userID, organizationID
}

func TestOrganizationOwnerMembership(t *testing.T) {
	ctx, tx := beginMembershipTestTx(t)

	userID, organizationID := seedMembershipRecords(
		t, ctx, tx,
	)

	_, err := tx.Exec(
		ctx,
		`INSERT INTO organization_memberships (
            organization_id,
            user_id,
            role
        ) VALUES ($1, $2, $3)`,
		organizationID,
		userID,
		"owner",
	)
	if err != nil {
		t.Fatalf("create owner membership: %v", err)
	}

	var role string

	err = tx.QueryRow(
		ctx,
		`SELECT role
         FROM organization_memberships
         WHERE organization_id = $1 AND user_id = $2`,
		organizationID,
		userID,
	).Scan(&role)

	if err != nil {
		t.Fatalf("query membership: %v", err)
	}

	if role != "owner" {
		t.Fatalf("expected owner role, got %s", role)
	}
}

func TestDuplicateOrganizationMembership(t *testing.T) {
	ctx, tx := beginMembershipTestTx(t)

	userID, organizationID := seedMembershipRecords(
		t, ctx, tx,
	)

	query := `
        INSERT INTO organization_memberships (
            organization_id, user_id, role
        ) VALUES ($1, $2, 'member')
    `

	_, err := tx.Exec(
		ctx, query, organizationID, userID,
	)
	if err != nil {
		t.Fatalf("first membership: %v", err)
	}

	_, err = tx.Exec(
		ctx, query, organizationID, userID,
	)

	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) {
		t.Fatalf("expected PostgreSQL error, got %v", err)
	}

	if pgErr.Code != "23505" ||
		pgErr.ConstraintName !=
			"organization_memberships_unique_member" {
		t.Fatalf(
			"unexpected constraint error: %s (%s)",
			pgErr.ConstraintName,
			pgErr.Code,
		)
	}
}

func TestMembershipRequiresExistingUser(t *testing.T) {
	ctx, tx := beginMembershipTestTx(t)

	_, organizationID := seedMembershipRecords(t, ctx, tx)

	_, err := tx.Exec(
		ctx,
		`INSERT INTO organization_memberships (
            organization_id, user_id, role
        ) VALUES ($1, $2, 'owner')`,
		organizationID,
		uuid.NewString(),
	)

	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) {
		t.Fatalf("expected foreign key error, got %v", err)
	}

	if pgErr.Code != "23503" ||
		pgErr.ConstraintName !=
			"organization_memberships_user_fk" {
		t.Fatalf(
			"unexpected foreign key error: %s (%s)",
			pgErr.ConstraintName,
			pgErr.Code,
		)
	}
}
