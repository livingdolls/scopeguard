//go:build integration

package integration

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	passwordadapter "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/password"
)

func TestPasswordCredentialsPersistence(t *testing.T) {
	ctx, tx := beginMembershipTestTx(t)

	userID := uuid.New()
	email := "auth-" + uuid.NewString() + "@example.test"

	_, err := tx.Exec(
		ctx,
		"INSERT INTO users (id, email) VALUES ($1, $2)",
		userID,
		email,
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	hasher := passwordadapter.NewArgon2idHasher()
	plainPassword := "correct horse battery staple"

	hash, err := hasher.Hash(plainPassword)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	_, err = tx.Exec(
		ctx,
		`INSERT INTO user_password_credentials
            (user_id, password_hash)
         VALUES ($1, $2)`,
		userID,
		hash,
	)
	if err != nil {
		t.Fatalf("save password credential: %v", err)
	}

	var storedHash string

	err = tx.QueryRow(
		ctx,
		`SELECT password_hash
         FROM user_password_credentials
         WHERE user_id = $1`,
		userID,
	).Scan(&storedHash)

	if err != nil {
		t.Fatalf("read password credential: %v", err)
	}

	if !strings.HasPrefix(storedHash, "$argon2id$") {
		t.Fatal("stored password is not an Argon2id hash")
	}

	if storedHash == plainPassword {
		t.Fatal("plaintext password was stored")
	}

	matched, err := hasher.Verify(
		plainPassword,
		storedHash,
	)
	if err != nil {
		t.Fatalf("verify stored password: %v", err)
	}

	if !matched {
		t.Fatal("stored password verification failed")
	}
}

func TestPasswordCredentialsRequireExistingUser(t *testing.T) {
	ctx, tx := beginMembershipTestTx(t)

	_, err := tx.Exec(
		ctx,
		`INSERT INTO user_password_credentials
            (user_id, password_hash)
         VALUES ($1, $2)`,
		uuid.New(),
		"test-placeholder-hash",
	)

	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) {
		t.Fatalf("expected PostgreSQL error, got %v", err)
	}

	if pgErr.Code != "23503" ||
		pgErr.ConstraintName !=
			"user_password_credentials_user_fk" {
		t.Fatalf(
			"expected user foreign key violation, got %s (%s)",
			pgErr.ConstraintName,
			pgErr.Code,
		)
	}
}
