package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

type VerifyEmailService struct {
	repository EmailVerificationRepository
}

func NewVerifyEmailService(repository EmailVerificationRepository) *VerifyEmailService {
	return &VerifyEmailService{
		repository: repository,
	}
}

func (s *VerifyEmailService) Execute(ctx context.Context, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// decode the URL-save token
	raw, err := base64.RawURLEncoding.DecodeString(token)

	if err != nil || len(raw) != verificationTokenBytes {
		return ErrInvalidVerificationToken
	}

	// enforce the canonical base64 representation
	if base64.RawURLEncoding.EncodeToString(raw) != token {
		return ErrInvalidVerificationToken
	}

	// hash the incoming token
	digest := sha256.Sum256(raw)

	// consume the token and activate user atomically

	if err := s.repository.ConsumeAndActivate(ctx, digest[:]); err != nil {
		return fmt.Errorf("verify email: %w", err)
	}

	return nil
}
