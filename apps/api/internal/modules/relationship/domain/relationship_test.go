package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewRelationship(t *testing.T) {
	now := time.Now()

	org := uuid.New()
	resource := uuid.New()
	membership := uuid.New()

	relations := []Relation{
		RelationViewer,
		RelationEditor,
	}

	for _, relation := range relations {
		grant, err := NewRelationship(
			org,
			resource,
			membership,
			relation,
			now,
		)
		if err != nil {
			t.Fatal(err)
		}

		if grant.OrganizationID() != org ||
			grant.ResourceID() != resource ||
			grant.SubjectMembershipID() != membership ||
			grant.Relation() != relation ||
			!grant.CreatedAt().Equal(now) {
			t.Fatalf("incorrect grant fields: %+v", grant)
		}
	}
}

func TestNewRelationshipValidation(t *testing.T) {
	org := uuid.New()
	resource := uuid.New()
	membership := uuid.New()
	now := time.Now()

	tests := []struct {
		name           string
		organizationID uuid.UUID
		resourceID     uuid.UUID
		membershipID   uuid.UUID
		relation       Relation
		time           time.Time
		want           error
	}{
		{
			"no organization",
			uuid.Nil,
			resource,
			membership,
			RelationViewer,
			now,
			ErrInvalidOrganizationID,
		},
		{
			"no resource",
			org,
			uuid.Nil,
			membership,
			RelationViewer,
			now,
			ErrInvalidResourceID,
		},
		{
			"no subject",
			org,
			resource,
			uuid.Nil,
			RelationViewer,
			now,
			ErrInvalidSubjectMembership,
		},
		{
			"owner forbidden",
			org,
			resource,
			membership,
			RelationOwner,
			now,
			ErrInvalidRelation,
		},
		{
			"unknown relation",
			org,
			resource,
			membership,
			"admin",
			now,
			ErrInvalidRelation,
		},
		{
			"missing time",
			org,
			resource,
			membership,
			RelationViewer,
			time.Time{},
			ErrInvalidResourceTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grant, err := NewRelationship(
				tt.organizationID,
				tt.resourceID,
				tt.membershipID,
				tt.relation,
				tt.time,
			)

			if grant != nil || !errors.Is(err, tt.want) {
				t.Fatalf(
					"grant=%v error=%v; want %v",
					grant, err, tt.want,
				)
			}
		})
	}
}
