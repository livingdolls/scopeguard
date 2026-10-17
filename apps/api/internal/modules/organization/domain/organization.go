package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	maxOrganizationNameLength = 160
	maxOrganizationSlugLength = 63
)

var slugPattern = regexp.MustCompile(
	`^[a-z0-9]+(-[a-z0-9]+)*$`,
)

type Organization struct {
	id        uuid.UUID
	name      string
	slug      string
	createdAt time.Time
	updatedAt time.Time
}

func NewOrganization(id uuid.UUID, name string, slug string, now time.Time) (*Organization, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidOrganizationID
	}

	name = strings.TrimSpace(name)

	if err := validateName(name); err != nil {
		return nil, err
	}

	if err := validateSlug(slug); err != nil {
		return nil, err
	}

	if now.IsZero() {
		return nil, ErrInvalidOrganizationTime
	}

	now = now.UTC()

	return &Organization{
		id:        id,
		name:      name,
		slug:      slug,
		createdAt: now,
		updatedAt: now,
	}, nil
}

func validateName(name string) error {
	length := utf8.RuneCountInString(name)

	if length == 0 || length > maxOrganizationNameLength {
		return ErrInvalidOrganizationName
	}

	return nil
}

func validateSlug(slug string) error {
	if len(slug) == 0 || len(slug) > maxOrganizationSlugLength || !slugPattern.MatchString(slug) {
		return ErrInvalidOrganizationSlug
	}

	return nil
}

// Read only accessors

func (o *Organization) ID() uuid.UUID {
	return o.id
}

func (o *Organization) Name() string {
	return o.name
}

func (o *Organization) Slug() string {
	return o.slug
}

func (o *Organization) CreatedAt() time.Time {
	return o.createdAt
}

func (o *Organization) UpdatedAt() time.Time {
	return o.updatedAt
}

// Domain behavior

func (o *Organization) Rename(name string, now time.Time) error {
	name = strings.TrimSpace(name)

	if err := validateName(name); err != nil {
		return err
	}

	if name == o.name {
		return nil
	}

	if now.IsZero() || now.Before(o.updatedAt) {
		return ErrInvalidOrganizationTime
	}

	o.name = name
	o.updatedAt = now.UTC()

	return nil
}
