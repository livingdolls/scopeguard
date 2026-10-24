package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type GetCurrentUser interface {
	Execute(ctx context.Context, userID uuid.UUID) (UserProfile, error)
}

type GetCurrentUserService struct {
	reader UserProfileReader
}

var _ GetCurrentUser = (*GetCurrentUserService)(nil)

func NewGetCurrentUserService(reader UserProfileReader) *GetCurrentUserService {
	return &GetCurrentUserService{
		reader: reader,
	}
}

func (s *GetCurrentUserService) Execute(ctx context.Context, userID uuid.UUID) (UserProfile, error) {
	if err := ctx.Err(); err != nil {
		return UserProfile{}, err
	}

	if userID == uuid.Nil {
		return UserProfile{}, ErrUserNotFound
	}

	profile, err := s.reader.FindProfileByID(ctx, userID)
	if err != nil {
		return UserProfile{}, fmt.Errorf("get current user: %w", err)
	}

	return profile, nil
}
