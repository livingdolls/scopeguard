package domain

import (
	"time"

	"github.com/google/uuid"
)

type Relationship struct {
	organizationID      uuid.UUID
	resourceID          uuid.UUID
	subjectMembershipID uuid.UUID
	relation            Relation
	createdAt           time.Time
}

func NewRelationship(organizationID uuid.UUID, resourceID uuid.UUID, subjectMembershipID uuid.UUID, relation Relation, now time.Time) (*Relationship, error) {
	if organizationID == uuid.Nil {
		return nil, ErrInvalidOrganizationID
	}

	if resourceID == uuid.Nil {
		return nil, ErrInvalidResourceID
	}

	if subjectMembershipID == uuid.Nil {
		return nil, ErrInvalidSubjectMembership
	}

	if !relation.Grantable() {
		return nil, ErrInvalidRelation
	}

	if now.IsZero() {
		return nil, ErrInvalidResourceTime
	}

	return &Relationship{
		organizationID:      organizationID,
		resourceID:          resourceID,
		subjectMembershipID: subjectMembershipID,
		relation:            relation,
		createdAt:           now.UTC(),
	}, nil
}

func (r *Relationship) OrganizationID() uuid.UUID {
	return r.organizationID
}

func (r *Relationship) ResourceID() uuid.UUID {
	return r.resourceID
}

func (r *Relationship) SubjectMembershipID() uuid.UUID {
	return r.subjectMembershipID
}

func (r *Relationship) Relation() Relation {
	return r.relation
}

func (r *Relationship) CreatedAt() time.Time {
	return r.createdAt
}
