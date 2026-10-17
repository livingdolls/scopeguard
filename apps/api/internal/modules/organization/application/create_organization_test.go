package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/domain"
)

type fakeOrganizationRepository struct {
	created *domain.Organization
	err     error
	calls   int
}

func (f *fakeOrganizationRepository) Create(
	ctx context.Context,
	org *domain.Organization,
) error {
	f.calls++

	if f.err != nil {
		return f.err
	}

	f.created = org
	return nil
}

func TestCreateOrganizationSuccess(t *testing.T) {
	repo := &fakeOrganizationRepository{}
	service := NewCreateOrganizationService(repo)

	result, err := service.Execute(
		context.Background(),
		CreateOrganizationInput{
			Name: "  Acme Corporation  ",
			Slug: "acme-corp",
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
			name: "empty name",
			input: CreateOrganizationInput{
				Name: "",
				Slug: "acme",
			},
			wantErr: domain.ErrInvalidOrganizationName,
		},
		{
			name: "invalid slug",
			input: CreateOrganizationInput{
				Name: "Acme",
				Slug: "INVALID_SLUG",
			},
			wantErr: domain.ErrInvalidOrganizationSlug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeOrganizationRepository{}
			service := NewCreateOrganizationService(repo)

			_, err := service.Execute(
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeOrganizationRepository{
				err: tt.err,
			}
			service := NewCreateOrganizationService(repo)

			_, err := service.Execute(
				context.Background(),
				CreateOrganizationInput{
					Name: "Acme",
					Slug: "acme",
				},
			)

			if !errors.Is(err, tt.err) {
				t.Fatalf(
					"expected %v, got %v",
					tt.err,
					err,
				)
			}
		})
	}
}

func TestCreateOrganizationCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repo := &fakeOrganizationRepository{}
	service := NewCreateOrganizationService(repo)

	_, err := service.Execute(
		ctx,
		CreateOrganizationInput{
			Name: "Acme",
			Slug: "acme",
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}

	if repo.calls != 0 {
		t.Fatal("repository must not be called")
	}
}
