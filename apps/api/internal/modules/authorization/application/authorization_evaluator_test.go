package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

type fakePermissionReader struct {
	calls int

	userID         uuid.UUID
	organizationID uuid.UUID
	permissionCode string

	allowed bool
	err     error
}

func (f *fakePermissionReader) HasPermission(
	ctx context.Context,
	userID uuid.UUID,
	organizationID uuid.UUID,
	permissionCode string,
) (bool, error) {
	f.calls++

	f.userID = userID
	f.organizationID = organizationID
	f.permissionCode = permissionCode

	return f.allowed, f.err
}

func TestAuthorizationEvaluatorAllowed(t *testing.T) {
	reader := &fakePermissionReader{
		allowed: true,
	}

	evaluator := NewAuthorizationEvaluatorService(reader)

	userID := uuid.New()
	organizationID := uuid.New()

	allowed, err := evaluator.Can(
		context.Background(),
		userID,
		organizationID,
		domain.PermissionMemberInvite,
	)

	if err != nil {
		t.Fatal(err)
	}

	if !allowed || reader.calls != 1 {
		t.Fatal("expected permission to be allowed")
	}

	if reader.userID != userID ||
		reader.organizationID != organizationID ||
		reader.permissionCode != domain.PermissionMemberInvite {
		t.Fatal("repository received incorrect arguments")
	}
}

func TestAuthorizationEvaluatorDenied(t *testing.T) {
	reader := &fakePermissionReader{
		allowed: false,
	}

	evaluator := NewAuthorizationEvaluatorService(reader)

	allowed, err := evaluator.Can(
		context.Background(),
		uuid.New(),
		uuid.New(),
		domain.PermissionRoleCreate,
	)

	if err != nil {
		t.Fatal(err)
	}

	if allowed {
		t.Fatal("permission must be denied")
	}
}

func TestAuthorizationEvaluatorRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		userID     uuid.UUID
		orgID      uuid.UUID
		permission string
		wantErr    error
	}{
		{
			name:       "empty user",
			userID:     uuid.Nil,
			orgID:      uuid.New(),
			permission: domain.PermissionMemberRead,
		},
		{
			name:       "empty organization",
			userID:     uuid.New(),
			orgID:      uuid.Nil,
			permission: domain.PermissionMemberRead,
		},
		{
			name:       "invalid permission code",
			userID:     uuid.New(),
			orgID:      uuid.New(),
			permission: "member:*",
			wantErr:    domain.ErrInvalidPermissionCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &fakePermissionReader{
				allowed: true,
			}

			evaluator := NewAuthorizationEvaluatorService(
				reader,
			)

			allowed, err := evaluator.Can(
				context.Background(),
				tt.userID,
				tt.orgID,
				tt.permission,
			)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if allowed || reader.calls != 0 {
				t.Fatal("invalid input must not reach reader")
			}
		})
	}
}

func TestAuthorizationEvaluatorRepositoryError(t *testing.T) {
	expected := errors.New("database unavailable")

	reader := &fakePermissionReader{
		allowed: true,
		err:     expected,
	}

	evaluator := NewAuthorizationEvaluatorService(reader)

	allowed, err := evaluator.Can(
		context.Background(),
		uuid.New(),
		uuid.New(),
		domain.PermissionRoleRead,
	)

	if allowed {
		t.Fatal("database error must not grant access")
	}

	if !errors.Is(err, expected) {
		t.Fatalf("expected wrapped database error, got %v", err)
	}
}

func TestAuthorizationEvaluatorCanceledContext(t *testing.T) {
	reader := &fakePermissionReader{allowed: true}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	evaluator := NewAuthorizationEvaluatorService(reader)

	allowed, err := evaluator.Can(
		ctx,
		uuid.New(),
		uuid.New(),
		domain.PermissionRoleRead,
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}

	if allowed || reader.calls != 0 {
		t.Fatal("canceled request must not grant access")
	}
}
