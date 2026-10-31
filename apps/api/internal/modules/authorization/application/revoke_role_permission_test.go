package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

type fakeRolePermissionRevoker struct {
	calls          int
	actorID        uuid.UUID
	organizationID uuid.UUID
	roleID         uuid.UUID
	permissionCode string
	removed        bool
	err            error
}

func (f *fakeRolePermissionRevoker) Revoke(
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

	return f.removed, f.err
}

func TestRevokeRolePermissionSuccess(t *testing.T) {
	repo := &fakeRolePermissionRevoker{
		removed: true,
	}

	service := NewRevokeRolePermissionService(repo)

	input := RevokeRolePermissionInput{
		ActorUserID:    uuid.New(),
		OrganizationID: uuid.New(),
		RoleID:         uuid.New(),
		PermissionCode: domain.PermissionMemberInvite,
	}

	output, err := service.Execute(
		context.Background(),
		input,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !output.Removed || repo.calls != 1 {
		t.Fatal("expected one successful revoke")
	}

	if repo.actorID != input.ActorUserID ||
		repo.organizationID != input.OrganizationID ||
		repo.roleID != input.RoleID ||
		repo.permissionCode != input.PermissionCode {
		t.Fatal("repository received incorrect input")
	}
}
