package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeRoleReader struct {
	calls          int
	organizationID uuid.UUID
	roles          []RoleListItem
	err            error
}

func (f *fakeRoleReader) ListByOrganization(
	ctx context.Context,
	organizationID uuid.UUID,
) ([]RoleListItem, error) {
	f.calls++
	f.organizationID = organizationID

	return f.roles, f.err
}

func TestListRolesSuccess(t *testing.T) {
	organizationID := uuid.New()

	reader := &fakeRoleReader{
		roles: []RoleListItem{
			{
				ID:             uuid.New(),
				OrganizationID: organizationID,
				Name:           "Owner",
				Slug:           "owner",
				IsSystem:       true,
			},
		},
	}

	service := NewListRolesService(reader)

	roles, err := service.Execute(
		context.Background(),
		organizationID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if reader.calls != 1 {
		t.Fatal("expected one repository call")
	}

	if reader.organizationID != organizationID {
		t.Fatal("incorrect organization ID")
	}

	if len(roles) != 1 || roles[0].Slug != "owner" {
		t.Fatal("unexpected roles")
	}
}

func TestListRolesRejectsMissingOrganization(t *testing.T) {
	reader := &fakeRoleReader{}
	service := NewListRolesService(reader)

	_, err := service.Execute(
		context.Background(),
		uuid.Nil,
	)

	if !errors.Is(err, ErrRoleOrganizationNotFound) {
		t.Fatalf("expected invalid organization, got %v", err)
	}

	if reader.calls != 0 {
		t.Fatal("invalid input reached repository")
	}
}

func TestListRolesRepositoryError(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	reader := &fakeRoleReader{
		err: databaseErr,
	}

	service := NewListRolesService(reader)

	_, err := service.Execute(
		context.Background(),
		uuid.New(),
	)

	if !errors.Is(err, databaseErr) {
		t.Fatalf("expected wrapped database error, got %v", err)
	}
}

func TestListRolesCanceledContext(t *testing.T) {
	reader := &fakeRoleReader{}
	service := NewListRolesService(reader)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.Execute(ctx, uuid.New())

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context, got %v", err)
	}

	if reader.calls != 0 {
		t.Fatal("canceled operation reached repository")
	}
}
