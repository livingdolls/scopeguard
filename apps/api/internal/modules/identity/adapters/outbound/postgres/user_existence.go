package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
)

var _ application.UserAccessChecker = (*UserRepository)(nil)

func (r *UserRepository) CanAuthenticate(ctx context.Context, userID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM users
			WHERE id = $1
				AND status = 'active'
				AND email_verified_at IS NOT NULL
		)
	`

	var exists bool

	if err := r.pool.QueryRow(
		ctx,
		query,
		userID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("error checking existence of user: %w", err)
	}

	return exists, nil
}
