package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/domain"
)

type RegisterUserInput struct {
	Email    string
	Password string
}

type RegisterUserOutput struct {
	ID        uuid.UUID
	Email     string
	CreatedAt time.Time
}

type RegisterUser interface {
	Execute(ctx context.Context, input RegisterUserInput) (RegisterUserOutput, error)
}

type RegisterUserService struct {
	repository RegistrationRepository
	hasher     PasswordHasher
	sealer     EmailPayloadSealer
}

var _ RegisterUser = (*RegisterUserService)(nil)

func NewRegisterUserService(repository RegistrationRepository, hasher PasswordHasher, sealer EmailPayloadSealer) *RegisterUserService {
	return &RegisterUserService{
		repository: repository,
		hasher:     hasher,
		sealer:     sealer,
	}
}

func (r RegisterUserService) Execute(ctx context.Context, input RegisterUserInput) (RegisterUserOutput, error) {
	if err := ctx.Err(); err != nil {
		return RegisterUserOutput{}, err
	}

	now := time.Now().UTC()

	user, err := domain.NewUser(uuid.New(), input.Email, now)

	if err != nil {
		return RegisterUserOutput{}, fmt.Errorf("create user entity: %w", err)
	}

	if err := domain.ValidatePassword(input.Password); err != nil {
		return RegisterUserOutput{}, err
	}

	passwordHash, err := r.hasher.Hash(input.Password)
	if err != nil {
		return RegisterUserOutput{}, fmt.Errorf("hash registration password: %w", err)
	}

	//generate verification token
	verification, err := generateVerificationToken(now)
	if err != nil {
		return RegisterUserOutput{}, err
	}

	// create IDs
	eventID := uuid.New()
	verificationID := uuid.New()

	encrypted, err := r.sealer.Seal(eventID, user.Email(), verification.Raw)
	if err != nil {
		return RegisterUserOutput{}, fmt.Errorf("seal event: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return RegisterUserOutput{}, err
	}

	record := RegistrationRecord{
		User:             user,
		PasswordHash:     passwordHash,
		VerificationID:   verificationID,
		TokenHash:        verification.Hash,
		TokenExpiresAt:   verification.ExpiresAt,
		OutboxID:         eventID,
		EncryptedPayload: encrypted,
	}

	if err := r.repository.RegisterWithVerification(ctx, record); err != nil {
		return RegisterUserOutput{}, fmt.Errorf("register user: %w", err)
	}

	return RegisterUserOutput{
		ID:        user.ID(),
		Email:     user.Email(),
		CreatedAt: user.CreatedAt(),
	}, nil
}
