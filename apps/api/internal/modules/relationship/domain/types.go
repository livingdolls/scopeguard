package domain

import "regexp"

const MaxHierarchyDepth = 8

type ResourceType string

const (
	ResourceProject  ResourceType = "project"
	ResourceFolder   ResourceType = "folder"
	ResourceDocument ResourceType = "document"
)

func (t ResourceType) Valid() bool {
	switch t {
	case ResourceProject, ResourceFolder, ResourceDocument:
		return true
	default:
		return false
	}
}

func ParentTypeAllowed(child, parent ResourceType) bool {
	switch child {
	case ResourceFolder, ResourceDocument:
		return parent == ResourceProject || parent == ResourceFolder
	default:
		return false
	}
}

var externalKeyPattern = regexp.MustCompile(
	`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`,
)

func ValidExternalKey(key string) bool {
	return externalKeyPattern.MatchString(key)
}

func ValidateHierarchyDepth(depth int) error {
	if depth < 0 || depth > MaxHierarchyDepth {
		return ErrInvalidHierarchyDepth
	}
	return nil
}

type Relation string

const (
	RelationViewer Relation = "viewer"
	RelationEditor Relation = "editor"
	RelationOwner  Relation = "owner"
)

func (r Relation) Valid() bool {
	return r == RelationViewer || r == RelationEditor || r == RelationOwner
}

func (r Relation) Grantable() bool {
	return r == RelationViewer || r == RelationEditor
}

type Action string

const (
	ActionRead     Action = "read"
	ActionWrite    Action = "write"
	ActionShare    Action = "share"
	ActionDelete   Action = "delete"
	ActionTransfer Action = "transfer"
)

func (a Action) Valid() bool {
	switch a {
	case ActionRead, ActionWrite, ActionShare, ActionDelete, ActionTransfer:
		return true
	default:
		return false
	}
}
