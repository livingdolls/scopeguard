package application

import (
	"context"
	"fmt"
)

type ListPermissions interface {
	Execute(ctx context.Context) ([]PermissionCatalogEntry, error)
}

type listPermissionService struct {
	reader PermissionCatalogReader
}

var _ ListPermissions = (*listPermissionService)(nil)

func NewListPermissionService(reader PermissionCatalogReader) ListPermissions {
	return &listPermissionService{
		reader: reader,
	}
}

func (l *listPermissionService) Execute(ctx context.Context) ([]PermissionCatalogEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	permissions, err := l.reader.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}

	return permissions, nil
}
