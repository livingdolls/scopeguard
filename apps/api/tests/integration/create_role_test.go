//go:build integration

package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	authorizationpostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/adapters/outbound/postgres"
	authorizationapp "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

func TestCreateRolePersistence(t *testing.T) {
	pool, _, ownerID := setupVerificationUser(t)

	organizationID := createRoleTestOrganization(
		t, pool, ownerID,
	)

	repository := authorizationpostgres.NewRoleRepository(pool)
	service := authorizationapp.NewCreateRoleService(repository)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	output, err := service.Execute(
		ctx,
		authorizationapp.CreateRoleInput{
			OrganizationID: organizationID,
			Name:           "Content Editor",
			Slug:           "content_editor",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	var storedName, storedSlug string
	var storedTenant uuid.UUID
	var isSystem bool

	err = pool.QueryRow(
		ctx,
		`SELECT
            organization_id,
            name,
            slug,
            is_system
         FROM roles
         WHERE id = $1`,
		output.ID,
	).Scan(
		&storedTenant,
		&storedName,
		&storedSlug,
		&isSystem,
	)

	if err != nil {
		t.Fatal(err)
	}

	if storedTenant != organizationID {
		t.Fatal("role belongs to incorrect tenant")
	}

	if storedName != "Content Editor" ||
		storedSlug != "content_editor" {
		t.Fatal("incorrect persisted role")
	}

	if isSystem {
		t.Fatal("custom role must not be a system role")
	}

	if output.IsSystem {
		t.Fatal("output incorrectly marks custom role as system")
	}
}

func TestCreateRoleSlugUniquePerTenant(t *testing.T) {
	pool, _, ownerID := setupVerificationUser(t)

	tenantA := createRoleTestOrganization(t, pool, ownerID)
	tenantB := createRoleTestOrganization(t, pool, ownerID)

	service := authorizationapp.NewCreateRoleService(
		authorizationpostgres.NewRoleRepository(pool),
	)

	ctx := context.Background()

	create := func(tenantID uuid.UUID) error {
		_, err := service.Execute(
			ctx,
			authorizationapp.CreateRoleInput{
				OrganizationID: tenantID,
				Name:           "Editor",
				Slug:           "editor",
			},
		)
		return err
	}

	// First role in Tenant A.
	if err := create(tenantA); err != nil {
		t.Fatal(err)
	}

	// Duplicate role slug in the same tenant.
	err := create(tenantA)

	if !errors.Is(
		err,
		authorizationapp.ErrRoleSlugAlreadyExists,
	) {
		t.Fatalf("expected duplicate role error: %v", err)
	}

	// Same slug in Tenant B must be accepted.
	if err := create(tenantB); err != nil {
		t.Fatalf(
			"same role slug must be allowed in another tenant: %v",
			err,
		)
	}
}

func TestCreateRoleOrganizationNotFound(t *testing.T) {
	pool := openTestDB(t)

	service := authorizationapp.NewCreateRoleService(
		authorizationpostgres.NewRoleRepository(pool),
	)

	_, err := service.Execute(
		context.Background(),
		authorizationapp.CreateRoleInput{
			OrganizationID: uuid.New(),
			Name:           "Editor",
			Slug:           "editor",
		},
	)

	if !errors.Is(
		err,
		authorizationapp.ErrRoleOrganizationNotFound,
	) {
		t.Fatalf(
			"expected organization not found, got %v",
			err,
		)
	}
}

func TestCreateRoleConcurrentDuplicate(t *testing.T) {
	pool, _, ownerID := setupVerificationUser(t)

	organizationID := createRoleTestOrganization(
		t, pool, ownerID,
	)

	service := authorizationapp.NewCreateRoleService(
		authorizationpostgres.NewRoleRepository(pool),
	)

	const workers = 2

	results := make(chan error, workers)

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			_, err := service.Execute(
				context.Background(),
				authorizationapp.CreateRoleInput{
					OrganizationID: organizationID,
					Name:           "Auditor",
					Slug:           "auditor",
				},
			)

			results <- err
		}()
	}

	wg.Wait()
	close(results)

	successes := 0
	duplicates := 0

	for err := range results {
		switch {
		case err == nil:
			successes++

		case errors.Is(
			err,
			authorizationapp.ErrRoleSlugAlreadyExists,
		):
			duplicates++

		default:
			t.Fatalf("unexpected creation error: %v", err)
		}
	}

	if successes != 1 || duplicates != 1 {
		t.Fatalf(
			"expected 1 success and 1 duplicate, got %d/%d",
			successes, duplicates,
		)
	}

	var count int

	err := pool.QueryRow(
		context.Background(),
		`SELECT COUNT(*)
         FROM roles
         WHERE organization_id = $1
           AND slug = 'auditor'`,
		organizationID,
	).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("expected one auditor role, got %d", count)
	}
}
