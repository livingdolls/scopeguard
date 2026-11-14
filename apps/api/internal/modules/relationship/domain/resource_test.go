package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testResource(
	t *testing.T,
	kind ResourceType,
	parent *Resource,
) *Resource {
	t.Helper()

	org := uuid.New()
	if parent != nil {
		org = parent.OrganizationID()
	}

	resource, err := NewResource(NewResourceInput{
		ID:                uuid.New(),
		OrganizationID:    org,
		Type:              kind,
		ExternalKey:       "resource-001",
		Parent:            parent,
		OwnerMembershipID: uuid.New(),
		Now:               time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	return resource
}

func TestNewResourceHierarchy(t *testing.T) {
	project := testResource(t, ResourceProject, nil)
	folder := testResource(t, ResourceFolder, project)
	nested := testResource(t, ResourceFolder, folder)
	document := testResource(t, ResourceDocument, nested)

	if project.ParentID() != uuid.Nil ||
		folder.ParentID() != project.ID() ||
		nested.ParentID() != folder.ID() ||
		document.ParentID() != nested.ID() {
		t.Fatal("unexpected parent IDs")
	}

	if document.OrganizationID() != project.OrganizationID() {
		t.Fatal("resource escaped its tenant")
	}

	if document.Version() != 1 {
		t.Fatal("new resource must begin with version 1")
	}

	if !document.CreatedAt().Equal(document.UpdatedAt()) {
		t.Fatal("new resource timestamps should match")
	}
}

func TestNewResourceValidation(t *testing.T) {
	now := time.Now()

	project := testResource(t, ResourceProject, nil)
	folder := testResource(t, ResourceFolder, project)

	base := NewResourceInput{
		ID:                uuid.New(),
		OrganizationID:    project.OrganizationID(),
		Type:              ResourceDocument,
		ExternalKey:       "doc-2026:main",
		Parent:            folder,
		OwnerMembershipID: uuid.New(),
		Now:               now,
	}

	tests := []struct {
		name   string
		change func(*NewResourceInput)
		want   error
	}{
		{
			"nil resource ID",
			func(p *NewResourceInput) { p.ID = uuid.Nil },
			ErrInvalidResourceID,
		},
		{
			"nil tenant",
			func(p *NewResourceInput) { p.OrganizationID = uuid.Nil },
			ErrInvalidOrganizationID,
		},
		{
			"unknown type",
			func(p *NewResourceInput) { p.Type = "widget" },
			ErrInvalidResourceType,
		},
		{
			"blank key",
			func(p *NewResourceInput) { p.ExternalKey = "" },
			ErrInvalidExternalKey,
		},
		{
			"space in key",
			func(p *NewResourceInput) { p.ExternalKey = "hello world" },
			ErrInvalidExternalKey,
		},
		{
			"key too long",
			func(p *NewResourceInput) {
				p.ExternalKey = strings.Repeat("a", 129)
			},
			ErrInvalidExternalKey,
		},
		{
			"missing owner",
			func(p *NewResourceInput) { p.OwnerMembershipID = uuid.Nil },
			ErrInvalidOwnerMembership,
		},
		{
			"zero time",
			func(p *NewResourceInput) { p.Now = time.Time{} },
			ErrInvalidResourceTime,
		},
		{
			"missing parent",
			func(p *NewResourceInput) { p.Parent = nil },
			ErrInvalidParent,
		},
		{
			"cross tenant parent",
			func(p *NewResourceInput) { p.OrganizationID = uuid.New() },
			ErrInvalidParent,
		},
		{
			"self parent",
			func(p *NewResourceInput) { p.ID = p.Parent.ID() },
			ErrInvalidParent,
		},
		{
			"document parent",
			func(p *NewResourceInput) {
				p.Parent = testResource(
					t,
					ResourceDocument,
					folder,
				)
			},
			ErrInvalidParent,
		},
		{
			"project with parent",
			func(p *NewResourceInput) { p.Type = ResourceProject },
			ErrInvalidParent,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			tc.change(&input)

			got, err := NewResource(input)

			if got != nil || !errors.Is(err, tc.want) {
				t.Fatalf(
					"resource=%v error=%v; want %v",
					got, err, tc.want,
				)
			}
		})
	}
}

func TestRestoreResourceValidation(t *testing.T) {
	now := time.Now().UTC()

	base := ResourceSnapshot{
		ID:                uuid.New(),
		OrganizationID:    uuid.New(),
		Type:              ResourceProject,
		ExternalKey:       "example:v1",
		OwnerMembershipID: uuid.New(),
		Version:           4,
		CreatedAt:         now.Add(-time.Hour),
		UpdatedAt:         now,
	}

	resource, err := RestoreResource(base)
	if err != nil ||
		resource.Version() != 4 ||
		resource.Type() != ResourceProject {
		t.Fatalf("restore failed: %v", err)
	}

	tests := []struct {
		name   string
		change func(*ResourceSnapshot)
		want   error
	}{
		{
			"nil id",
			func(s *ResourceSnapshot) { s.ID = uuid.Nil },
			ErrInvalidResourceID,
		},
		{
			"nil org",
			func(s *ResourceSnapshot) { s.OrganizationID = uuid.Nil },
			ErrInvalidOrganizationID,
		},
		{
			"invalid type",
			func(s *ResourceSnapshot) { s.Type = "file" },
			ErrInvalidResourceType,
		},
		{
			"invalid key",
			func(s *ResourceSnapshot) { s.ExternalKey = "hello/world" },
			ErrInvalidExternalKey,
		},
		{
			"no owner",
			func(s *ResourceSnapshot) { s.OwnerMembershipID = uuid.Nil },
			ErrInvalidOwnerMembership,
		},
		{
			"project with parent",
			func(s *ResourceSnapshot) { s.ParentID = uuid.New() },
			ErrInvalidParent,
		},
		{
			"folder without parent",
			func(s *ResourceSnapshot) { s.Type = ResourceFolder },
			ErrInvalidParent,
		},
		{
			"self parent",
			func(s *ResourceSnapshot) {
				s.Type = ResourceDocument
				s.ParentID = s.ID
			},
			ErrInvalidParent,
		},
		{
			"zero version",
			func(s *ResourceSnapshot) { s.Version = 0 },
			ErrInvalidResourceVersion,
		},
		{
			"zero timestamp",
			func(s *ResourceSnapshot) { s.CreatedAt = time.Time{} },
			ErrInvalidResourceTime,
		},
		{
			"reverse timestamp",
			func(s *ResourceSnapshot) {
				s.UpdatedAt = s.CreatedAt.Add(-time.Second)
			},
			ErrInvalidResourceTime,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := base
			tc.change(&snapshot)

			got, err := RestoreResource(snapshot)

			if got != nil || !errors.Is(err, tc.want) {
				t.Fatalf(
					"resource=%v error=%v; want %v",
					got, err, tc.want,
				)
			}
		})
	}
}

func TestResourceTransferOwnership(t *testing.T) {
	r := testResource(t, ResourceProject, nil)

	previousOwner := r.OwnerMembershipID()
	previousTime := r.UpdatedAt()
	newOwner := uuid.New()

	changed, err := r.TransferOwnership(
		newOwner,
		1,
		previousTime.Add(time.Second),
	)
	if err != nil || !changed {
		t.Fatalf(
			"expected transfer, changed=%v err=%v",
			changed, err,
		)
	}

	if r.OwnerMembershipID() != newOwner || r.Version() != 2 {
		t.Fatal("owner/version not updated")
	}

	// Same owner must be a no-op.
	changed, err = r.TransferOwnership(
		newOwner, 2, time.Time{},
	)
	if err != nil || changed || r.Version() != 2 {
		t.Fatal("same owner must be an idempotent no-op")
	}

	// A stale version must not mutate the resource.
	changed, err = r.TransferOwnership(
		previousOwner,
		1,
		previousTime.Add(2*time.Second),
	)
	if changed || !errors.Is(err, ErrResourceVersionConflict) {
		t.Fatalf("expected stale version rejection, got %v", err)
	}

	changed, err = r.TransferOwnership(
		uuid.Nil,
		2,
		previousTime.Add(2*time.Second),
	)
	if changed || !errors.Is(err, ErrInvalidOwnerMembership) {
		t.Fatalf("expected invalid target, got %v", err)
	}

	changed, err = r.TransferOwnership(
		previousOwner,
		2,
		previousTime,
	)
	if changed || !errors.Is(err, ErrInvalidResourceTime) {
		t.Fatalf("expected invalid timestamp, got %v", err)
	}

	if r.OwnerMembershipID() != newOwner || r.Version() != 2 {
		t.Fatal("invalid transfer mutated resource")
	}
}

func TestHierarchyDepthAndExternalKeys(t *testing.T) {
	for _, n := range []int{
		0, 1, MaxHierarchyDepth,
	} {
		if err := ValidateHierarchyDepth(n); err != nil {
			t.Fatalf("depth=%d: %v", n, err)
		}
	}

	for _, n := range []int{
		-1, MaxHierarchyDepth + 1,
	} {
		if !errors.Is(
			ValidateHierarchyDepth(n),
			ErrInvalidHierarchyDepth,
		) {
			t.Fatalf("expected invalid depth %d", n)
		}
	}

	validKeys := []string{
		"ABC_123",
		"doc-2026:main",
		strings.Repeat("a", 128),
	}

	for _, key := range validKeys {
		if !ValidExternalKey(key) {
			t.Fatalf("expected valid key %q", key)
		}
	}

	invalidKeys := []string{
		"",
		":bad",
		"doc/2026",
		"hello world",
		strings.Repeat("a", 129),
	}

	for _, key := range invalidKeys {
		if ValidExternalKey(key) {
			t.Fatalf("expected invalid key %q", key)
		}
	}
}
