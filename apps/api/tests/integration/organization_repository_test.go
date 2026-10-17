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
) (*pgxpool.Pool, *application.CreateOrganizationService) {
	t.Helper()

	pool := openTestDB(t)

	repository := orgpostgres.NewOrganizationRepository(pool)
	service := application.NewCreateOrganizationService(repository)

	return pool, service
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
	pool, service := newOrganizationTestService(t)

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
			Name: "Acme Corporation",
			Slug: slug,
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
}

func TestOrganizationRepositoryDuplicateSlug(t *testing.T) {
	pool, service := newOrganizationTestService(t)

	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, pool, slug)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	input := application.CreateOrganizationInput{
		Name: "Acme Corporation",
		Slug: slug,
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
	pool, service := newOrganizationTestService(t)

	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, pool, slug)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	input := application.CreateOrganizationInput{
		Name: "Concurrent Organization",
		Slug: slug,
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
