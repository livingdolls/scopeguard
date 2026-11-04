package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

type AuthorizationEvaluator interface {
	Can(ctx context.Context, userID uuid.UUID, organizationID uuid.UUID, permissionCode string) (bool, error)
}

type authorizationEvaluatorService struct {
	reader PermissionReader
}

var _ AuthorizationEvaluator = (*authorizationEvaluatorService)(nil)

func NewAuthorizationEvaluatorService(reader PermissionReader) *authorizationEvaluatorService {
	return &authorizationEvaluatorService{
		reader: reader,
	}
}

func (a *authorizationEvaluatorService) Can(ctx context.Context, userID uuid.UUID, organizationID uuid.UUID, permissionCode string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	permission, err := domain.ParsePermission(permissionCode)
	if err != nil {
		return false, fmt.Errorf("invalid permission code: %w", err)
	}

	// missing identity or tenant never grant access
	if userID == uuid.Nil || organizationID == uuid.Nil {
		return false, nil
	}

	allowed, err := a.reader.HasPermission(ctx, userID, organizationID, permission.Code())
	if err != nil {
		return false, fmt.Errorf("evaluate effective permission: %w", err)
	}

	return allowed, nil
}
