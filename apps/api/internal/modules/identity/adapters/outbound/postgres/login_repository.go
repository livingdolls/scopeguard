package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
)

var _ application.LoginRepository = (*UserRepository)(nil)

func (u *UserRepository) FindByEmail(ctx context.Context, email string) (application.LoginCredential, error) {
	const query = `
		SELECT
			u.id,
			c.password_hash
		FROM users u 
		INNER JOIN user_password_credentials c 
			ON c.user_id = u.id
		WHERE u.email = $1
	`

	var credential application.LoginCredential

	err := u.pool.QueryRow(
		ctx,
		query,
		email,
	).Scan(
		&credential.UserID,
		&credential.PasswordHash,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.LoginCredential{}, application.ErrUserNotFound
		}

		return application.LoginCredential{}, fmt.Errorf("query login: %w", err)
	}

	return credential, nil
}
