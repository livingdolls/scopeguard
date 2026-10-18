//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	passwordadapter "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/password"
	identitypostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/postgres"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/domain"
)

func newRegistrationService(
	t *testing.T,
) (*pgxpool.Pool, *application.RegisterUserService) {
	t.Helper()

	pool := openTestDB(t)

	repository := identitypostgres.NewUserRepository(pool)
	hasher := passwordadapter.NewArgon2idHasher()

	return pool, application.NewRegisterUserService(
		repository, hasher,
	)
}

func cleanupRegisteredUser(
	t *testing.T,
	pool *pgxpool.Pool,
	id uuid.UUID,
) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		_, err := pool.Exec(
			ctx,
			"DELETE FROM users WHERE id = $1",
			id,
		)
		if err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})
}

func TestRegisterUserPersistence(t *testing.T) {
	pool, service := newRegistrationService(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	email := "REGISTER-" + uuid.NewString() + "@EXAMPLE.TEST"
	password := "correct horse battery staple"

	output, err := service.Execute(
		ctx,
		application.RegisterUserInput{
			Email:    email,
			Password: password,
		},
	)
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	cleanupRegisteredUser(t, pool, output.ID)

	var storedEmail string
	var storedHash string

	err = pool.QueryRow(
		ctx,
		`SELECT u.email, c.password_hash
         FROM users u
         JOIN user_password_credentials c
           ON c.user_id = u.id
         WHERE u.id = $1`,
		output.ID,
	).Scan(&storedEmail, &storedHash)

	if err != nil {
		t.Fatalf("load registered user: %v", err)
	}

	if storedEmail != output.Email {
		t.Fatalf("unexpected stored email: %s", storedEmail)
	}

	var status string
	var verifiedAt *time.Time

	err = pool.QueryRow(
		ctx,
		`SELECT status, email_verified_at
         FROM users
         WHERE id = $1`,
		output.ID,
	).Scan(&status, &verifiedAt)

	if err != nil {
		t.Fatal(err)
	}

	if status != "pending_verification" || verifiedAt != nil {
		t.Fatal("new user must start unverified")
	}

	if storedHash == password {
		t.Fatal("plaintext password was stored")
	}

	hasher := passwordadapter.NewArgon2idHasher()

	matched, err := hasher.Verify(password, storedHash)
	if err != nil {
		t.Fatalf("verify stored password: %v", err)
	}

	if !matched {
		t.Fatal("stored password does not match")
	}
}

func TestRegisterUserDuplicateEmail(t *testing.T) {
	pool, service := newRegistrationService(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	defer cancel()

	email := "duplicate-" + uuid.NewString() + "@example.test"

	first, err := service.Execute(
		ctx,
		application.RegisterUserInput{
			Email:    email,
			Password: "correct horse battery staple",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	cleanupRegisteredUser(t, pool, first.ID)

	// Same email with different letter case.
	_, err = service.Execute(
		ctx,
		application.RegisterUserInput{
			Email:    "  " + email + "  ",
			Password: "another secure passphrase",
		},
	)

	if !errors.Is(err, application.ErrEmailAlreadyExists) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}

	var count int

	err = pool.QueryRow(
		ctx,
		"SELECT COUNT(*) FROM users WHERE email = $1",
		email,
	).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("expected 1 user, got %d", count)
	}
}

func TestRegisterUserRollbackOnCredentialFailure(
	t *testing.T,
) {
	pool := openTestDB(t)

	repository := identitypostgres.NewUserRepository(pool)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	user, err := domain.NewUser(
		uuid.New(),
		"rollback-"+uuid.NewString()+"@example.test",
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Intentionally violate password_hash CHECK constraint.
	err = repository.CreateWithPassword(ctx, user, "")

	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) {
		t.Fatalf("expected PostgreSQL error, got %v", err)
	}

	if pgErr.Code != "23514" ||
		pgErr.ConstraintName !=
			"user_password_credentials_hash_not_blank" {
		t.Fatalf(
			"unexpected database error: %s (%s)",
			pgErr.ConstraintName,
			pgErr.Code,
		)
	}

	// Confirm the earlier INSERT into users was rolled back.
	var count int

	err = pool.QueryRow(
		ctx,
		"SELECT COUNT(*) FROM users WHERE id = $1",
		user.ID(),
	).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatal("user persisted despite credential failure")
	}
}

func TestRegisterUserConcurrentDuplicateEmail(t *testing.T) {
	pool, service := newRegistrationService(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	email := "concurrent-" + uuid.NewString() + "@example.test"

	type result struct {
		id  uuid.UUID
		err error
	}

	start := make(chan struct{})
	results := make(chan result, 2)

	for i := 0; i < 2; i++ {
		go func() {
			<-start

			output, err := service.Execute(
				ctx,
				application.RegisterUserInput{
					Email:    email,
					Password: "correct horse battery staple",
				},
			)

			results <- result{
				id:  output.ID,
				err: err,
			}
		}()
	}

	close(start)

	successCount := 0
	duplicateCount := 0

	for i := 0; i < 2; i++ {
		current := <-results

		switch {
		case current.err == nil:
			successCount++
			cleanupRegisteredUser(t, pool, current.id)

		case errors.Is(
			current.err,
			application.ErrEmailAlreadyExists,
		):
			duplicateCount++

		default:
			t.Fatalf(
				"unexpected registration error: %v",
				current.err,
			)
		}
	}

	if successCount != 1 || duplicateCount != 1 {
		t.Fatalf(
			"expected 1 success and 1 duplicate, got %d and %d",
			successCount,
			duplicateCount,
		)
	}
}
