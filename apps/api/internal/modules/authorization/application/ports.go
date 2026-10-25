package application

import (
	"context"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

type RoleRepository interface {
	Insert(ctx context.Context, role *domain.Role) error
}
