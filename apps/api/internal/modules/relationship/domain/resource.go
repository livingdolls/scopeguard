package domain

import (
	"time"

	"github.com/google/uuid"
)

type Resource struct {
	id                uuid.UUID
	organizationID    uuid.UUID
	resourceType      ResourceType
	externalKey       string
	parentID          uuid.UUID
	ownerMembershipID uuid.UUID
	version           int64
	createdAt         time.Time
	updatedAt         time.Time
}

type NewResourceInput struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	Type              ResourceType
	ExternalKey       string
	Parent            *Resource
	OwnerMembershipID uuid.UUID
	Now               time.Time
}

func NewResource(input NewResourceInput) (*Resource, error) {
	if input.ID == uuid.Nil {
		return nil, ErrInvalidResourceID
	}

	if input.OrganizationID == uuid.Nil {
		return nil, ErrInvalidOrganizationID
	}

	if !input.Type.Valid() {
		return nil, ErrInvalidResourceType
	}

	parentID := uuid.Nil

	if input.Parent == nil {
		if input.Type != ResourceProject {
			return nil, ErrInvalidParent
		}
	} else {
		if input.Parent.organizationID != input.OrganizationID || input.Parent.id == input.ID || !ParentTypeAllowed(input.Type, input.Parent.resourceType) {
			return nil, ErrInvalidParent
		}

		parentID = input.Parent.id
	}

	return RestoreResource(ResourceSnapshot{
		ID:                input.ID,
		OrganizationID:    input.OrganizationID,
		Type:              input.Type,
		ExternalKey:       input.ExternalKey,
		ParentID:          parentID,
		OwnerMembershipID: input.OwnerMembershipID,
		Version:           1,
		CreatedAt:         input.Now,
		UpdatedAt:         input.Now,
	})
}

type ResourceSnapshot struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	Type              ResourceType
	ExternalKey       string
	ParentID          uuid.UUID
	OwnerMembershipID uuid.UUID
	Version           int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func RestoreResource(snapshot ResourceSnapshot) (*Resource, error) {
	if snapshot.ID == uuid.Nil {
		return nil, ErrInvalidResourceID
	}

	if snapshot.OrganizationID == uuid.Nil {
		return nil, ErrInvalidOrganizationID
	}

	if !snapshot.Type.Valid() {
		return nil, ErrInvalidResourceType
	}

	if !ValidExternalKey(snapshot.ExternalKey) {
		return nil, ErrInvalidExternalKey
	}

	if snapshot.OwnerMembershipID == uuid.Nil {
		return nil, ErrInvalidOwnerMembership
	}

	if snapshot.Type == ResourceProject {
		if snapshot.ParentID != uuid.Nil {
			return nil, ErrInvalidParent
		}
	} else if snapshot.ParentID == uuid.Nil || snapshot.ParentID == snapshot.ID {
		return nil, ErrInvalidParent
	}

	if snapshot.Version < 1 {
		return nil, ErrInvalidResourceVersion
	}

	if snapshot.CreatedAt.IsZero() || snapshot.UpdatedAt.IsZero() || snapshot.UpdatedAt.Before(snapshot.CreatedAt) {
		return nil, ErrInvalidResourceTime
	}

	return &Resource{
		id:                snapshot.ID,
		organizationID:    snapshot.OrganizationID,
		resourceType:      snapshot.Type,
		externalKey:       snapshot.ExternalKey,
		parentID:          snapshot.ParentID,
		ownerMembershipID: snapshot.OwnerMembershipID,
		version:           snapshot.Version,
		createdAt:         snapshot.CreatedAt.UTC(),
		updatedAt:         snapshot.UpdatedAt.UTC(),
	}, nil
}

func (r *Resource) ID() uuid.UUID {
	return r.id
}

func (r *Resource) OrganizationID() uuid.UUID {
	return r.organizationID
}

func (r *Resource) Type() ResourceType {
	return r.resourceType
}

func (r *Resource) ExternalKey() string {
	return r.externalKey
}

func (r *Resource) ParentID() uuid.UUID {
	return r.parentID
}

func (r *Resource) OwnerMembershipID() uuid.UUID {
	return r.ownerMembershipID
}

func (r *Resource) Version() int64 {
	return r.version
}

func (r *Resource) CreatedAt() time.Time {
	return r.createdAt
}

func (r *Resource) UpdatedAt() time.Time {
	return r.updatedAt
}

// TransferOwnership only changes domain state.
// The application/repository must first validate that
// the target membership is active and belongs to the tenant.
func (r *Resource) TransferOwnership(target uuid.UUID, expectedVersion int64, now time.Time) (bool, error) {
	if target == uuid.Nil {
		return false, ErrInvalidOwnerMembership
	}

	if expectedVersion < 1 {
		return false, ErrInvalidResourceVersion
	}

	if r.version != expectedVersion {
		return false, ErrResourceVersionConflict
	}

	if r.ownerMembershipID == target {
		return false, nil
	}

	if now.IsZero() || !now.After(r.updatedAt) {
		return false, ErrInvalidResourceTime
	}

	r.ownerMembershipID = target
	r.version++
	r.updatedAt = now.UTC()

	return true, nil
}
