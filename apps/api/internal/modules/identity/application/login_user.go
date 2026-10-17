package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/domain"
)

type LoginUserInput struct {
	Email    string
	Password string
}

type LoginUserOutput struct {
	AccessToken string
	TokenType   string
	ExpiresAt   time.Time
}

type LoginUser interface {
	Execute(ctx context.Context, input LoginUserInput) (LoginUserOutput, error)
}

type LoginUserService struct {
	repository LoginRepository
	hasher     PasswordHasher
	tokens     AccessTokenIssuer
	dummyHash  string
}

var _ LoginUser = (*LoginUserService)(nil)

func NewLoginUserService(repository LoginRepository, hasher PasswordHasher, tokens AccessTokenIssuer, dummyHash string) (*LoginUserService, error) {
	if dummyHash == "" {
		return nil, errors.New("dummy password hash is required")
	}
	return &LoginUserService{
		repository: repository,
		hasher:     hasher,
		tokens:     tokens,
		dummyHash:  dummyHash,
	}, nil
}

func (l LoginUserService) Execute(ctx context.Context, input LoginUserInput) (LoginUserOutput, error) {
	if err := ctx.Err(); err != nil {
		return LoginUserOutput{}, err
	}

	email, err := domain.NormalizeEmail(input.Email)
	if err != nil {
		return LoginUserOutput{}, ErrInvalidCredentials
	}

	if input.Password == "" || len(input.Password) > 1024 {
		return LoginUserOutput{}, ErrInvalidCredentials
	}

	credentials, err := l.repository.FindByEmail(ctx, email)

	if errors.Is(err, ErrUserNotFound) {
		// perform a comparable password verification
		// even when the email is not registered
		_, verifyErr := l.hasher.Verify(input.Password, l.dummyHash)
		if verifyErr != nil {
			return LoginUserOutput{}, fmt.Errorf("verify dummy password: %w", verifyErr)
		}

		return LoginUserOutput{}, ErrInvalidCredentials
	}

	if err != nil {
		return LoginUserOutput{}, fmt.Errorf("find login credentials: %w", err)
	}

	matched, err := l.hasher.Verify(input.Password, credentials.PasswordHash)

	if err != nil {
		return LoginUserOutput{}, fmt.Errorf("verify login credentials: %w", err)
	}

	if !matched {
		return LoginUserOutput{}, ErrInvalidCredentials
	}

	token, expiresAt, err := l.tokens.Issue(credentials.UserID)
	if err != nil {
		return LoginUserOutput{}, fmt.Errorf("issue access token: %w", err)
	}

	return LoginUserOutput{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
	}, nil
}
