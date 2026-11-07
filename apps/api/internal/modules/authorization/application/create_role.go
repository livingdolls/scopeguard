package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

type CreateRoleInput struct {
	ActorUserID    uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Slug           string
}

type CreateRoleOutput struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Slug           string
	IsSystem       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateRole interface {
	Execute(ctx context.Context, input CreateRoleInput) (CreateRoleOutput, error)
}

type CreateRoleService struct {
	repository RoleRepository
}

var _ CreateRole = (*CreateRoleService)(nil)

func NewCreateRoleService(repository RoleRepository) *CreateRoleService {
	return &CreateRoleService{
		repository: repository,
	}
}

func (c *CreateRoleService) Execute(ctx context.Context, input CreateRoleInput) (CreateRoleOutput, error) {
	if err := ctx.Err(); err != nil {
		return CreateRoleOutput{}, err
	}

	if input.ActorUserID == uuid.Nil {
		return CreateRoleOutput{}, ErrPermissionAssignmentForbidden
	}

	now := time.Now().UTC()

	// Only custom roles may be created through this workflow.
	role, err := domain.NewRole(
		uuid.New(),
		input.OrganizationID,
		input.Name,
		input.Slug,
		false,
		now,
	)

	if err != nil {
		return CreateRoleOutput{}, fmt.Errorf("create role entity: %w", err)
	}

	// Domain validation must pass before persistence
	if err := c.repository.Insert(ctx, input.ActorUserID, role); err != nil {
		return CreateRoleOutput{}, fmt.Errorf("persist role: %w", err)
	}

	return CreateRoleOutput{
		ID:             role.ID(),
		OrganizationID: role.OrganizationID(),
		Name:           role.Name(),
		Slug:           role.Slug(),
		IsSystem:       role.IsSystem(),
		CreatedAt:      role.CreatedAt(),
		UpdatedAt:      role.UpdatedAt(),
	}, nil

}
