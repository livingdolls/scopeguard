package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

type fakeRolePermissionRepository struct {
	calls          int
	actorID        uuid.UUID
	organizationID uuid.UUID
	roleID         uuid.UUID
	permissionCode string

	added bool
	err   error
}

func (f *fakeRolePermissionRepository) Grant(
	ctx context.Context,
	actorID uuid.UUID,
	organizationID uuid.UUID,
	roleID uuid.UUID,
	permissionCode string,
) (bool, error) {
	f.calls++
	f.actorID = actorID
	f.organizationID = organizationID
	f.roleID = roleID
	f.permissionCode = permissionCode

	return f.added, f.err
}

func TestGrantRolePermissionSuccess(t *testing.T) {
	repo := &fakeRolePermissionRepository{
		added: true,
	}

	service := NewGrantRolePermissionService(repo)

	input := GrantRolePermissionInput{
		ActorUserID:    uuid.New(),
		OrganizationID: uuid.New(),
		RoleID:         uuid.New(),
		PermissionCode: domain.PermissionMemberInvite,
	}

	result, err := service.Execute(
		context.Background(),
		input,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Added || repo.calls != 1 {
		t.Fatal("expected one successful grant")
	}

	if repo.actorID != input.ActorUserID ||
		repo.organizationID != input.OrganizationID ||
		repo.roleID != input.RoleID ||
		repo.permissionCode != input.PermissionCode {
		t.Fatal("repository received incorrect input")
	}
}

func TestGrantRolePermissionIdempotentReturn(t *testing.T) {
	repo := &fakeRolePermissionRepository{
		added: false,
	}
	service := NewGrantRolePermissionService(repo)

	result, err := service.Execute(
		context.Background(),
		GrantRolePermissionInput{
			ActorUserID:    uuid.New(),
			OrganizationID: uuid.New(),
			RoleID:         uuid.New(),
			PermissionCode: domain.PermissionMemberInvite,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.Added {
		t.Fatal("expected Added=false for an existing grant")
	}

	if repo.calls != 1 {
		t.Fatalf("expected one repository call, got %d", repo.calls)
	}
}

func TestGrantRolePermissionCanceledContext(t *testing.T) {
	repo := &fakeRolePermissionRepository{added: true}
	service := NewGrantRolePermissionService(repo)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := service.Execute(
		ctx,
		GrantRolePermissionInput{
			ActorUserID:    uuid.New(),
			OrganizationID: uuid.New(),
			RoleID:         uuid.New(),
			PermissionCode: domain.PermissionMemberInvite,
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context, got %v", err)
	}

	if result.Added {
		t.Fatal("canceled operation must not report an added grant")
	}

	if repo.calls != 0 {
		t.Fatalf("canceled operation reached repository: %d calls", repo.calls)
	}
}

func TestGrantRolePermissionRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input GrantRolePermissionInput
		want  error
	}{
		{
			name: "missing actor",
			input: GrantRolePermissionInput{
				OrganizationID: uuid.New(),
				RoleID:         uuid.New(),
				PermissionCode: domain.PermissionMemberInvite,
			},
			want: ErrPermissionAssignmentForbidden,
		},
		{
			name: "missing organization",
			input: GrantRolePermissionInput{
				ActorUserID:    uuid.New(),
				RoleID:         uuid.New(),
				PermissionCode: domain.PermissionMemberInvite,
			},
			want: ErrPermissionAssignmentForbidden,
		},
		{
			name: "invalid permission",
			input: GrantRolePermissionInput{
				ActorUserID:    uuid.New(),
				OrganizationID: uuid.New(),
				RoleID:         uuid.New(),
				PermissionCode: "member:*",
			},
			want: domain.ErrInvalidPermissionCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRolePermissionRepository{}
			service := NewGrantRolePermissionService(repo)

			_, err := service.Execute(
				context.Background(),
				tt.input,
			)

			if !errors.Is(err, tt.want) {
				t.Fatalf(
					"expected %v, got %v",
					tt.want, err,
				)
			}

			if repo.calls != 0 {
				t.Fatal("invalid input reached repository")
			}
		})
	}
}

func TestGrantRolePermissionPropagatesForbidden(t *testing.T) {
	repo := &fakeRolePermissionRepository{
		err: ErrPermissionAssignmentForbidden,
	}

	service := NewGrantRolePermissionService(repo)

	_, err := service.Execute(
		context.Background(),
		GrantRolePermissionInput{
			ActorUserID:    uuid.New(),
			OrganizationID: uuid.New(),
			RoleID:         uuid.New(),
			PermissionCode: domain.PermissionMemberInvite,
		},
	)

	if !errors.Is(err, ErrPermissionAssignmentForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}
