package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/domain"
)

type fakeOrganizationRepository struct {
	created     *domain.Organization
	ownerUserID uuid.UUID
	err         error
	calls       int
}

func (f *fakeOrganizationRepository) CreateWithOwner(
	ctx context.Context,
	org *domain.Organization,
	ownerUserID uuid.UUID,
) error {
	f.calls++

	if f.err != nil {
		return f.err
	}

	f.created = org
	f.ownerUserID = ownerUserID
	return nil
}

func TestCreateOrganizationSuccess(t *testing.T) {
	ownerUserID := uuid.New()
	repo := &fakeOrganizationRepository{}
	service := NewCreateOrganizationService(repo)

	result, err := service.Execute(
		context.Background(),
		CreateOrganizationInput{
			Name:        "  Acme Corporation  ",
			Slug:        "acme-corp",
			OwnerUserID: ownerUserID,
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.calls != 1 {
		t.Fatalf("expected 1 repository call, got %d", repo.calls)
	}

	if repo.created == nil {
		t.Fatal("organization was not persisted")
	}

	if repo.ownerUserID != ownerUserID {
		t.Errorf("expected owner user ID %s, got %s", ownerUserID, repo.ownerUserID)
	}

	if result.ID == uuid.Nil {
		t.Error("organization ID must not be empty")
	}

	if result.Name != "Acme Corporation" {
		t.Errorf("unexpected name: %s", result.Name)
	}

	if result.Slug != "acme-corp" {
		t.Errorf("unexpected slug: %s", result.Slug)
	}

	if result.ID != repo.created.ID() {
		t.Error("output ID differs from persisted entity")
	}

	if result.CreatedAt.IsZero() {
		t.Error("createdAt must not be zero")
	}

	if !result.CreatedAt.Equal(result.UpdatedAt) {
		t.Error("timestamps should match on creation")
	}
}

func TestCreateOrganizationValidationError(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateOrganizationInput
		wantErr error
	}{
		{
			name: "invalid owner user ID",
			input: CreateOrganizationInput{
				Name:        "Acme",
				Slug:        "acme",
				OwnerUserID: uuid.Nil,
			},
			wantErr: ErrInvalidOwnerUserID,
		},
		{
			name: "empty name",
			input: CreateOrganizationInput{
				Name:        "",
				Slug:        "acme",
				OwnerUserID: uuid.New(),
			},
			wantErr: domain.ErrInvalidOrganizationName,
		},
		{
			name: "invalid slug",
			input: CreateOrganizationInput{
				Name:        "Acme",
				Slug:        "INVALID_SLUG",
				OwnerUserID: uuid.New(),
			},
			wantErr: domain.ErrInvalidOrganizationSlug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeOrganizationRepository{}
			service := NewCreateOrganizationService(repo)

			result, err := service.Execute(
				context.Background(),
				tt.input,
			)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf(
					"expected %v, got %v",
					tt.wantErr,
					err,
				)
			}

			if repo.calls != 0 {
				t.Fatal("repository must not be called")
			}

			if result != (CreateOrganizationOutput{}) {
				t.Errorf("expected empty output, got %+v", result)
			}
		})
	}
}

func TestCreateOrganizationRepositoryError(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "database unavailable",
			err:  errors.New("database unavailable"),
		},
		{
			name: "duplicate slug",
			err:  ErrOrganizationSlugAlreadyExists,
		},
		{
			name: "owner user not found",
			err:  ErrOwnerUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeOrganizationRepository{
				err: tt.err,
			}
			service := NewCreateOrganizationService(repo)

			result, err := service.Execute(
				context.Background(),
				CreateOrganizationInput{
					Name:        "Acme",
					Slug:        "acme",
					OwnerUserID: uuid.New(),
				},
			)

			if !errors.Is(err, tt.err) {
				t.Fatalf(
					"expected %v, got %v",
					tt.err,
					err,
				)
			}

			if repo.calls != 1 {
				t.Fatalf("expected 1 repository call, got %d", repo.calls)
			}

			if result != (CreateOrganizationOutput{}) {
				t.Errorf("expected empty output, got %+v", result)
			}
		})
	}
}

func TestCreateOrganizationCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repo := &fakeOrganizationRepository{}
	service := NewCreateOrganizationService(repo)

	result, err := service.Execute(
		ctx,
		CreateOrganizationInput{
			Name:        "Acme",
			Slug:        "acme",
			OwnerUserID: uuid.New(),
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}

	if repo.calls != 0 {
		t.Fatal("repository must not be called")
	}

	if result != (CreateOrganizationOutput{}) {
		t.Errorf("expected empty output, got %+v", result)
	}
}

func TestCreateOrganizationInvalidOwner(t *testing.T) {
	repo := &fakeOrganizationRepository{}
	service := NewCreateOrganizationService(repo)

	_, err := service.Execute(
		context.Background(),
		CreateOrganizationInput{
			Name:        "Acme",
			Slug:        "acme",
			OwnerUserID: uuid.Nil,
		},
	)

	if !errors.Is(err, ErrInvalidOwnerUserID) {
		t.Fatalf(
			"expected invalid owner ID, got %v",
			err,
		)
	}

	if repo.calls != 0 {
		t.Fatal("repository must not be called")
	}
}
