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
	activateRoleTestUser(t, pool, ownerID)

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
			ActorUserID:    ownerID,
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
	activateRoleTestUser(t, pool, ownerID)

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
				ActorUserID:    ownerID,
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
			ActorUserID:    uuid.New(),
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
	activateRoleTestUser(t, pool, ownerID)

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
					ActorUserID:    ownerID,
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

func TestCreateRoleRejectsUnverifiedActor(t *testing.T) {
	pool, _, ownerID := setupVerificationUser(t)
	organizationID := createRoleTestOrganization(t, pool, ownerID)
	service := authorizationapp.NewCreateRoleService(
		authorizationpostgres.NewRoleRepository(pool),
	)

	ctx := context.Background()
	_, err := service.Execute(ctx, authorizationapp.CreateRoleInput{
		ActorUserID:    ownerID,
		OrganizationID: organizationID,
		Name:           "Auditor",
		Slug:           "auditor",
	})
	if !errors.Is(err, authorizationapp.ErrPermissionAssignmentForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM roles WHERE organization_id = $1 AND slug = 'auditor'`,
		organizationID,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("unverified actor's role was persisted")
	}
}

func TestCreateRoleRequiresPermission(t *testing.T) {
	pool, _, ownerID := setupVerificationUser(t)
	activateRoleTestUser(t, pool, ownerID)
	organizationID := createRoleTestOrganization(t, pool, ownerID)
	ctx := context.Background()

	// Remove role:create from the owner's system role.
	result, err := pool.Exec(ctx,
		`DELETE FROM role_permissions rp
		 USING roles r
		 WHERE rp.organization_id = r.organization_id
		   AND rp.role_id = r.id
		   AND r.organization_id = $1
		   AND r.slug = 'owner'
		   AND rp.permission_code = 'role:create'`,
		organizationID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected() != 1 {
		t.Fatal("expected one role:create permission removed")
	}

	service := authorizationapp.NewCreateRoleService(
		authorizationpostgres.NewRoleRepository(pool),
	)
	_, err = service.Execute(ctx, authorizationapp.CreateRoleInput{
		ActorUserID:    ownerID,
		OrganizationID: organizationID,
		Name:           "Unauthorized Editor",
		Slug:           "unauthorized_editor",
	})
	if !errors.Is(err, authorizationapp.ErrPermissionAssignmentForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM roles
		 WHERE organization_id = $1 AND slug = 'unauthorized_editor'`,
		organizationID,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("unauthorized role was persisted")
	}
}

func TestCreateRoleRejectsCrossTenantActor(t *testing.T) {
	pool, _, actorA := setupVerificationUser(t)
	_, _, actorB := setupVerificationUser(t)
	activateRoleTestUser(t, pool, actorA)
	activateRoleTestUser(t, pool, actorB)

	// Tenant A establishes actor A's membership and role:create permission.
	createRoleTestOrganization(t, pool, actorA)
	tenantB := createRoleTestOrganization(t, pool, actorB)
	service := authorizationapp.NewCreateRoleService(
		authorizationpostgres.NewRoleRepository(pool),
	)

	ctx := context.Background()
	_, err := service.Execute(ctx, authorizationapp.CreateRoleInput{
		ActorUserID:    actorA,
		OrganizationID: tenantB,
		Name:           "Cross Tenant Editor",
		Slug:           "cross_tenant_editor",
	})
	if !errors.Is(err, authorizationapp.ErrPermissionAssignmentForbidden) {
		t.Fatalf("expected cross-tenant rejection, got %v", err)
	}

	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM roles
		 WHERE organization_id = $1 AND slug = 'cross_tenant_editor'`,
		tenantB,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("cross-tenant role was persisted")
	}
}
