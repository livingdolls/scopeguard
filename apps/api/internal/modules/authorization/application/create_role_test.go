package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

type fakeRoleRepository struct {
	calls    int
	received *domain.Role
	err      error
}

func (f *fakeRoleRepository) Insert(
	ctx context.Context,
	role *domain.Role,
) error {
	f.calls++
	f.received = role
	return f.err
}

func TestCreateRoleSuccess(t *testing.T) {
	repo := &fakeRoleRepository{}
	service := NewCreateRoleService(repo)

	organizationID := uuid.New()

	output, err := service.Execute(
		context.Background(),
		CreateRoleInput{
			OrganizationID: organizationID,
			Name:           " Content Editor ",
			Slug:           "content_editor",
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	if repo.calls != 1 {
		t.Fatal("expected one repository call")
	}

	if output.ID == uuid.Nil {
		t.Fatal("role ID must be generated")
	}

	if output.OrganizationID != organizationID {
		t.Fatal("incorrect organization ID")
	}

	if output.Name != "Content Editor" {
		t.Fatal("role name must be trimmed")
	}

	if output.Slug != "content_editor" {
		t.Fatal("incorrect role slug")
	}

	if output.IsSystem {
		t.Fatal("custom role must not be a system role")
	}

	if repo.received == nil ||
		repo.received.ID() != output.ID {
		t.Fatal("repository received incorrect role")
	}
}

func TestCreateRoleRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input CreateRoleInput
		want  error
	}{
		{
			name: "missing organization",
			input: CreateRoleInput{
				Name: "Editor",
				Slug: "editor",
			},
			want: domain.ErrInvalidRoleOrganizationID,
		},
		{
			name: "empty name",
			input: CreateRoleInput{
				OrganizationID: uuid.New(),
				Name:           "",
				Slug:           "editor",
			},
			want: domain.ErrInvalidRoleName,
		},
		{
			name: "invalid slug",
			input: CreateRoleInput{
				OrganizationID: uuid.New(),
				Name:           "Editor",
				Slug:           "Editor",
			},
			want: domain.ErrInvalidRoleSlug,
		},
		{
			name: "reserved owner slug",
			input: CreateRoleInput{
				OrganizationID: uuid.New(),
				Name:           "Owner",
				Slug:           "owner",
			},
			want: domain.ErrInvalidRoleSystemFlag,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRoleRepository{}
			service := NewCreateRoleService(repo)

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

func TestCreateRoleRepositoryError(t *testing.T) {
	repo := &fakeRoleRepository{
		err: ErrRoleSlugAlreadyExists,
	}

	service := NewCreateRoleService(repo)

	_, err := service.Execute(
		context.Background(),
		CreateRoleInput{
			OrganizationID: uuid.New(),
			Name:           "Editor",
			Slug:           "editor",
		},
	)

	if !errors.Is(err, ErrRoleSlugAlreadyExists) {
		t.Fatalf("expected wrapped duplicate error: %v", err)
	}
}

func TestCreateRoleCanceledContext(t *testing.T) {
	repo := &fakeRoleRepository{}
	service := NewCreateRoleService(repo)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := service.Execute(
		ctx,
		CreateRoleInput{
			OrganizationID: uuid.New(),
			Name:           "Editor",
			Slug:           "editor",
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context: %v", err)
	}

	if repo.calls != 0 {
		t.Fatal("canceled operation reached repository")
	}
}
