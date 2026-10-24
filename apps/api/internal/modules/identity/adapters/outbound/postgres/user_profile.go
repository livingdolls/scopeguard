package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
)

func (r *UserRepository) FindProfileByID(ctx context.Context, userID uuid.UUID) (application.UserProfile, error) {
	const query = `
		SELECT id, email
		FROM users
		WHERE id = $1
			AND status = 'active'
			AND email_verified_at IS NOT NULL
	`

	var profile application.UserProfile

	err := r.pool.QueryRow(
		ctx,
		query,
		userID,
	).Scan(
		&profile.ID,
		&profile.Email,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return application.UserProfile{}, application.ErrUserNotFound
	}

	if err != nil {
		return application.UserProfile{}, fmt.Errorf("get user profile query: %w", err)
	}

	return profile, nil
}
