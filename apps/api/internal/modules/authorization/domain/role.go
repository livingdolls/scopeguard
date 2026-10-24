package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalidRoleID             = errors.New("invalid role ID")
	ErrInvalidRoleOrganizationID = errors.New("invalid role organization ID")
	ErrInvalidRoleName           = errors.New("invalid role name")
	ErrInvalidRoleSlug           = errors.New("invalid role slug")
	ErrInvalidRoleSystemFlag     = errors.New("invalid system role configuration")
	ErrInvalidRoleTime           = errors.New("invalid role timestamp")
)

const (
	RoleOwner  = "owner"
	RoleMember = "member"
)

var roleSlugPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

type Role struct {
	id             uuid.UUID
	organizationID uuid.UUID
	name           string
	slug           string
	system         bool
	createdAt      time.Time
	updatedAt      time.Time
}

func NewRole(id uuid.UUID, organizationID uuid.UUID, name string, slug string, system bool, now time.Time) (*Role, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidRoleID
	}

	if organizationID == uuid.Nil {
		return nil, ErrInvalidRoleOrganizationID
	}

	name = strings.TrimSpace(name)

	if utf8.RuneCountInString(name) == 0 || utf8.RuneCountInString(name) > 80 {
		return nil, ErrInvalidRoleName
	}

	if !roleSlugPattern.MatchString(slug) {
		return nil, ErrInvalidRoleSlug
	}

	reserved := slug == RoleOwner || slug == RoleMember

	// only owner/member may be system roles
	// custom roles cannot reuse reserved slugs
	if reserved != system {
		return nil, ErrInvalidRoleSystemFlag
	}

	if now.IsZero() {
		return nil, ErrInvalidRoleTime
	}

	now = now.UTC()

	return &Role{
		id:             id,
		organizationID: organizationID,
		name:           name,
		slug:           slug,
		system:         system,
		createdAt:      now,
		updatedAt:      now,
	}, nil
}

func (r *Role) ID() uuid.UUID {
	return r.id
}

func (r *Role) OrganizationID() uuid.UUID {
	return r.organizationID
}

func (r *Role) Name() string {
	return r.name
}

func (r *Role) Slug() string {
	return r.slug
}

func (r *Role) IsSystem() bool {
	return r.system
}

func (r *Role) CreatedAt() time.Time {
	return r.createdAt
}

func (r *Role) UpdatedAt() time.Time {
	return r.updatedAt
}
