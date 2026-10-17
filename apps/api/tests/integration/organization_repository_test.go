//go:build integration

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	orgpostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/adapters/outbound/postgres"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/application"
)

func newOrganizationTestService(
	t *testing.T,
) (
	*pgxpool.Pool,
	*application.CreateOrganizationService,
	uuid.UUID,
) {
	t.Helper()

	pool := openTestDB(t)

	ownerID := uuid.New()
	email := "owner-" + uuid.NewString() + "@example.test"

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	_, err := pool.Exec(
		ctx,
		"INSERT INTO users (id, email) VALUES ($1, $2)",
		ownerID.String(),
		email,
	)
	if err != nil {
		t.Fatalf("create test owner: %v", err)
	}

	// Registered before organization cleanup.
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cleanupCancel()

		_, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM users WHERE id = $1",
			ownerID.String(),
		)
		if err != nil {
			t.Errorf("cleanup test owner: %v", err)
		}
	})

	repository := orgpostgres.NewOrganizationRepository(pool)
	service := application.NewCreateOrganizationService(repository)

	return pool, service, ownerID
}

func uniqueOrganizationSlug() string {
	return "test-" + strings.ReplaceAll(
		uuid.NewString(), "-", "",
	)
}

func cleanupOrganization(
	t *testing.T,
	pool *pgxpool.Pool,
	slug string,
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
			"DELETE FROM organizations WHERE slug = $1",
			slug,
		)

		if err != nil {
			t.Errorf("cleanup organization: %v", err)
		}
	})
}

func TestOrganizationRepositoryCreate(t *testing.T) {
	pool, service, ownerID := newOrganizationTestService(t)

	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, pool, slug)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	output, err := service.Execute(
		ctx,
		application.CreateOrganizationInput{
			Name:        "Acme Corporation",
			Slug:        slug,
			OwnerUserID: ownerID,
		},
	)
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}

	var storedID string
	var storedName string
	var storedSlug string
	var createdAt time.Time
	var updatedAt time.Time

	err = pool.QueryRow(
		ctx,
		`SELECT id::text, name, slug, created_at, updated_at
         FROM organizations
         WHERE id = $1`,
		output.ID.String(),
	).Scan(
		&storedID,
		&storedName,
		&storedSlug,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		t.Fatalf("query organization: %v", err)
	}

	if storedID != output.ID.String() {
		t.Error("persisted ID differs from output")
	}

	if storedName != output.Name {
		t.Errorf("unexpected stored name: %s", storedName)
	}

	if storedSlug != output.Slug {
		t.Errorf("unexpected stored slug: %s", storedSlug)
	}

	if createdAt.IsZero() || updatedAt.IsZero() {
		t.Error("timestamps must not be zero")
	}

	if !createdAt.Equal(updatedAt) {
		t.Error("creation timestamps should match")
	}

	var role string

	err = pool.QueryRow(
		ctx,
		`SELECT role
         FROM organization_memberships
         WHERE organization_id = $1
           AND user_id = $2`,
		output.ID.String(),
		ownerID.String(),
	).Scan(&role)

	if err != nil {
		t.Fatalf("query owner membership: %v", err)
	}

	if role != "owner" {
		t.Fatalf("expected owner role, got %s", role)
	}
}

func TestOrganizationRepositoryDuplicateSlug(t *testing.T) {
	pool, service, ownerID := newOrganizationTestService(t)

	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, pool, slug)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	input := application.CreateOrganizationInput{
		Name:        "Acme Corporation",
		Slug:        slug,
		OwnerUserID: ownerID,
	}

	_, err := service.Execute(ctx, input)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = service.Execute(ctx, input)

	if !errors.Is(
		err,
		application.ErrOrganizationSlugAlreadyExists,
	) {
		t.Fatalf("expected duplicate slug error, got %v", err)
	}

	var count int

	err = pool.QueryRow(
		ctx,
		"SELECT COUNT(*) FROM organizations WHERE slug = $1",
		slug,
	).Scan(&count)

	if err != nil {
		t.Fatalf("count organizations: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 organization, got %d", count)
	}
}

func TestOrganizationRepositoryConcurrentCreate(t *testing.T) {
	pool, service, ownerID := newOrganizationTestService(t)

	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, pool, slug)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	input := application.CreateOrganizationInput{
		Name:        "Concurrent Organization",
		Slug:        slug,
		OwnerUserID: ownerID,
	}

	start := make(chan struct{})
	results := make(chan error, 2)

	for i := 0; i < 2; i++ {
		go func() {
			<-start
			_, err := service.Execute(ctx, input)
			results <- err
		}()
	}

	close(start)

	successes := 0
	duplicates := 0

	for i := 0; i < 2; i++ {
		err := <-results

		switch {
		case err == nil:
			successes++

		case errors.Is(
			err,
			application.ErrOrganizationSlugAlreadyExists,
		):
			duplicates++

		default:
			t.Fatalf("unexpected create error: %v", err)
		}
	}

	if successes != 1 || duplicates != 1 {
		t.Fatalf(
			"expected 1 success and 1 duplicate, got %d and %d",
			successes,
			duplicates,
		)
	}
}

func TestCreateOrganizationRollbackWhenOwnerNotFound(
	t *testing.T,
) {
	pool, service, _ := newOrganizationTestService(t)

	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, pool, slug)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	// Valid UUID, but no corresponding user record.
	missingOwnerID := uuid.New()

	_, err := service.Execute(
		ctx,
		application.CreateOrganizationInput{
			Name:        "Rollback Test",
			Slug:        slug,
			OwnerUserID: missingOwnerID,
		},
	)

	if !errors.Is(
		err,
		application.ErrOwnerUserNotFound,
	) {
		t.Fatalf(
			"expected owner not found error, got %v",
			err,
		)
	}

	var organizationCount int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
         FROM organizations
         WHERE slug = $1`,
		slug,
	).Scan(&organizationCount)

	if err != nil {
		t.Fatalf("count organizations: %v", err)
	}

	if organizationCount != 0 {
		t.Fatalf(
			"expected rollback to remove organization, found %d",
			organizationCount,
		)
	}
}
