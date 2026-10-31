//go:build integration

package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	authorizationpostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/adapters/outbound/postgres"
	authorizationapp "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

// Hold the tenant row until every operation is blocked behind it. Starting
// goroutines together alone would also pass if mutations skipped the tenant lock.
func runConcurrentRolePermissionMutations(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	organizationID uuid.UUID,
	operations ...func(context.Context) (bool, error),
) []bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	defer func() {
		cancel()
		rollbackCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := tx.Rollback(rollbackCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback tenant lock: %v", err)
		}
		wg.Wait()
	}()

	var blockerPID int32
	err = tx.QueryRow(ctx,
		`SELECT pg_backend_pid() FROM organizations WHERE id = $1 FOR UPDATE`,
		organizationID,
	).Scan(&blockerPID)
	if err != nil {
		t.Fatal(err)
	}

	type result struct {
		index   int
		changed bool
		err     error
	}
	results := make(chan result, len(operations))
	start := make(chan struct{})
	for i, operation := range operations {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			changed, err := operation(ctx)
			results <- result{index: i, changed: changed, err: err}
		}()
	}
	close(start)

	waitCtx, stopWaiting := context.WithTimeout(ctx, 5*time.Second)
	defer stopWaiting()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		// PostgreSQL may queue the second waiter behind the first waiter.
		// Follow the blocking chain back to our tenant lock in either case.
		var waiters int
		err := pool.QueryRow(waitCtx,
			`WITH RECURSIVE blocked(pid) AS (
			     SELECT pid FROM pg_stat_activity
			     WHERE $1 = ANY(pg_blocking_pids(pid))
			     UNION
			     SELECT a.pid FROM pg_stat_activity a
			     JOIN blocked b ON b.pid = ANY(pg_blocking_pids(a.pid))
			 )
			 SELECT COUNT(*) FROM blocked`,
			blockerPID,
		).Scan(&waiters)
		if err != nil {
			t.Fatalf("wait for %d operations to take the tenant lock: %v", len(operations), err)
		}
		if waiters == len(operations) {
			break
		}
		select {
		case early := <-results:
			t.Fatalf("operation %d finished before tenant lock release: changed=%t, err=%v", early.index, early.changed, early.err)
		case <-waitCtx.Done():
			t.Fatalf("expected %d tenant lock waiters, got %d: %v", len(operations), waiters, waitCtx.Err())
		case <-ticker.C:
		}
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("release tenant lock: %v", err)
	}

	changed := make([]bool, len(operations))
	for range operations {
		select {
		case current := <-results:
			if current.err != nil {
				t.Fatalf("concurrent operation %d failed: %v", current.index, current.err)
			}
			changed[current.index] = current.changed
		case <-ctx.Done():
			t.Fatalf("concurrent permission mutations did not finish: %v", ctx.Err())
		}
	}
	return changed
}

func TestRevokeRolePermissionConcurrent(t *testing.T) {
	ctx, pool, grantInput := setupPermissionGrant(t)
	repository := authorizationpostgres.NewRolePermissionRepository(pool)
	grant := authorizationapp.NewGrantRolePermissionService(repository)
	if output, err := grant.Execute(ctx, grantInput); err != nil || !output.Added {
		t.Fatalf("expected initial grant, got %+v, %v", output, err)
	}

	revoke := authorizationapp.NewRevokeRolePermissionService(repository)
	input := authorizationapp.RevokeRolePermissionInput(grantInput)
	mutation := func(ctx context.Context) (bool, error) {
		output, err := revoke.Execute(ctx, input)
		return output.Removed, err
	}
	removed := runConcurrentRolePermissionMutations(t, ctx, pool, input.OrganizationID, mutation, mutation)
	if removed[0] == removed[1] {
		t.Fatalf("expected one removal and one no-op, got %v", removed)
	}
	if permissions := getRolePermissions(t, pool, input.OrganizationID, "auditor"); len(permissions) != 0 {
		t.Fatalf("concurrent revokes left assignments: %v", permissions)
	}
}

func TestGrantAndRevokeRolePermissionConcurrent(t *testing.T) {
	tests := []struct {
		name             string
		initiallyGranted bool
	}{
		{name: "assignment absent"},
		{name: "assignment present", initiallyGranted: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, pool, grantInput := setupPermissionGrant(t)
			repository := authorizationpostgres.NewRolePermissionRepository(pool)
			grant := authorizationapp.NewGrantRolePermissionService(repository)
			revoke := authorizationapp.NewRevokeRolePermissionService(repository)
			if tt.initiallyGranted {
				if output, err := grant.Execute(ctx, grantInput); err != nil || !output.Added {
					t.Fatalf("expected initial grant, got %+v, %v", output, err)
				}
			}

			changed := runConcurrentRolePermissionMutations(t, ctx, pool, grantInput.OrganizationID,
				func(ctx context.Context) (bool, error) {
					output, err := grant.Execute(ctx, grantInput)
					return output.Added, err
				},
				func(ctx context.Context) (bool, error) {
					output, err := revoke.Execute(ctx, authorizationapp.RevokeRolePermissionInput(grantInput))
					return output.Removed, err
				},
			)
			added, removed := changed[0], changed[1]
			if !tt.initiallyGranted && !added {
				t.Fatal("grant of an initially absent assignment must add it")
			}
			if tt.initiallyGranted && !removed {
				t.Fatal("revoke of an initially present assignment must remove it")
			}

			// Either serial ordering is valid. Match persisted state to the actual changes.
			wantCount := 0
			if tt.initiallyGranted {
				wantCount++
			}
			if added {
				wantCount++
			}
			if removed {
				wantCount--
			}
			permissions := getRolePermissions(t, pool, grantInput.OrganizationID, "auditor")
			if len(permissions) > 1 || len(permissions) != wantCount {
				t.Fatalf("invalid final assignments %v for Added=%t, Removed=%t", permissions, added, removed)
			}
			if len(permissions) == 1 && permissions[0] != grantInput.PermissionCode {
				t.Fatalf("unexpected final permission: %v", permissions)
			}
		})
	}
}
