package domain

import "errors"

var (
	ErrInvalidResourceID        = errors.New("invalid resource ID")
	ErrInvalidOrganizationID    = errors.New("invalid organization ID")
	ErrInvalidResourceType      = errors.New("invalid resource type")
	ErrInvalidExternalKey       = errors.New("invalid external key")
	ErrInvalidParent            = errors.New("invalid parent")
	ErrInvalidOwnerMembership   = errors.New("invalid owner membership")
	ErrInvalidResourceTime      = errors.New("invalid resource time")
	ErrInvalidResourceVersion   = errors.New("invalid resource version")
	ErrResourceVersionConflict  = errors.New("resource version conflict")
	ErrInvalidSubjectMembership = errors.New("invalid subject membership")
	ErrInvalidRelation          = errors.New("invalid relation")
	ErrInvalidAction            = errors.New("invalid action")
	ErrInvalidHierarchyDepth    = errors.New("invalid hierarchy depth")
)
