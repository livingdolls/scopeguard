package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewOrganization(t *testing.T) {
	id := uuid.New()
	now := time.Date(
		2026, 10, 17, 8, 0, 0, 0, time.UTC,
	)

	org, err := NewOrganization(
		id,
		"  Acme Corporation  ",
		"acme-corp",
		now,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if org.ID() != id {
		t.Errorf("unexpected organization ID")
	}

	if org.Name() != "Acme Corporation" {
		t.Errorf("unexpected name: %s", org.Name())
	}

	if org.Slug() != "acme-corp" {
		t.Errorf("unexpected slug: %s", org.Slug())
	}

	if !org.CreatedAt().Equal(now) {
		t.Error("unexpected createdAt")
	}

	if !org.UpdatedAt().Equal(now) {
		t.Error("unexpected updatedAt")
	}
}

func TestNewOrganizationValidation(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		id      uuid.UUID
		orgName string
		slug    string
		now     time.Time
		wantErr error
	}{
		{
			name:    "invalid ID",
			id:      uuid.Nil,
			orgName: "Acme",
			slug:    "acme",
			now:     now,
			wantErr: ErrInvalidOrganizationID,
		},
		{
			name:    "empty name",
			id:      uuid.New(),
			orgName: "   ",
			slug:    "acme",
			now:     now,
			wantErr: ErrInvalidOrganizationName,
		},
		{
			name:    "name too long",
			id:      uuid.New(),
			orgName: strings.Repeat("a", 161),
			slug:    "acme",
			now:     now,
			wantErr: ErrInvalidOrganizationName,
		},
		{
			name:    "invalid slug",
			id:      uuid.New(),
			orgName: "Acme",
			slug:    "Acme_Corp",
			now:     now,
			wantErr: ErrInvalidOrganizationSlug,
		},
		{
			name:    "slug too long",
			id:      uuid.New(),
			orgName: "Acme",
			slug:    strings.Repeat("a", 64),
			now:     now,
			wantErr: ErrInvalidOrganizationSlug,
		},
		{
			name:    "invalid time",
			id:      uuid.New(),
			orgName: "Acme",
			slug:    "acme",
			now:     time.Time{},
			wantErr: ErrInvalidOrganizationTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, err := NewOrganization(
				tt.id,
				tt.orgName,
				tt.slug,
				tt.now,
			)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf(
					"expected %v, got %v",
					tt.wantErr,
					err,
				)
			}

			if org != nil {
				t.Fatal("expected nil organization")
			}
		})
	}
}

func TestOrganizationRename(t *testing.T) {
	now := time.Now().UTC()

	org, err := NewOrganization(
		uuid.New(),
		"Acme",
		"acme",
		now,
	)
	if err != nil {
		t.Fatal(err)
	}

	updatedAt := now.Add(time.Hour)

	err = org.Rename(
		"Acme Corporation",
		updatedAt,
	)
	if err != nil {
		t.Fatalf("rename failed: %v", err)
	}

	if org.Name() != "Acme Corporation" {
		t.Errorf("name was not updated")
	}

	if !org.UpdatedAt().Equal(updatedAt) {
		t.Error("updatedAt was not changed")
	}

	if !org.CreatedAt().Equal(now) {
		t.Error("createdAt must remain unchanged")
	}
}

func TestOrganizationRenameInvalid(t *testing.T) {
	now := time.Now().UTC()

	org, err := NewOrganization(
		uuid.New(),
		"Acme",
		"acme",
		now,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = org.Rename("", now.Add(time.Hour))

	if !errors.Is(err, ErrInvalidOrganizationName) {
		t.Fatalf("expected invalid name, got %v", err)
	}

	if org.Name() != "Acme" {
		t.Fatal("invalid rename changed organization name")
	}

	if !org.UpdatedAt().Equal(now) {
		t.Fatal("invalid rename changed updatedAt")
	}

	err = org.Rename("New Name", now.Add(-time.Hour))

	if !errors.Is(err, ErrInvalidOrganizationTime) {
		t.Fatalf("expected invalid time, got %v", err)
	}
}
