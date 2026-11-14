package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

type MembershipRolesReader struct {
	pool *pgxpool.Pool
}

var _ application.MembershipRolesReader = (*MembershipRolesReader)(nil)

func NewMembershipRolesReader(pool *pgxpool.Pool) *MembershipRolesReader {
	return &MembershipRolesReader{
		pool: pool,
	}
}

func (m *MembershipRolesReader) ListByMembership(ctx context.Context, organizationID uuid.UUID, membershipID uuid.UUID) ([]application.RoleListItem, error) {
	const query = `
		SELECT
		    m.id,
		    mr.role_id,
		    r.id,
		    r.organization_id,
		    r.name,
		    r.slug,
		    r.is_system,
		    r.created_at,
		    r.updated_at
		FROM organization_memberships AS m 
		LEFT JOIN membership_roles AS mr
			ON mr.organization_id = m.organization_id
			AND mr.membership_id = m.id
		LEFT JOIN roles AS r
			ON r.organization_id = mr.organization_id
			AND r.id = mr.role_id
		WHERE m.organization_id = $1
			AND m.id = $2
		ORDER BY
		    r.is_system DESC NULLS LAST,
		    r.slug ASC NULLS LAST,
		    r.id ASC NULLS LAST
	`

	rows, err := m.pool.Query(ctx, query, organizationID, membershipID)
	if err != nil {
		return nil, fmt.Errorf("query list membership roles: %w", err)
	}
	defer rows.Close()

	roles := make([]application.RoleListItem, 0)
	membershipFound := false

	for rows.Next() {
		membershipFound = true

		var rowMembershipID uuid.UUID
		var assignedRoleID *uuid.UUID
		var roleID *uuid.UUID
		var roleOrganizationID *uuid.UUID
		var name *string
		var slug *string
		var isSystem *bool
		var createdAt *time.Time
		var updatedAt *time.Time

		err = rows.Scan(
			&rowMembershipID,
			&assignedRoleID,
			&roleID,
			&roleOrganizationID,
			&name,
			&slug,
			&isSystem,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("scan list membership roles: %w", err)
		}

		//membership exist but has no assigned roles
		if assignedRoleID == nil {
			continue
		}

		if roleID == nil || roleOrganizationID == nil || name == nil || slug == nil || isSystem == nil || createdAt == nil || updatedAt == nil {
			return nil, fmt.Errorf("incomplete role assignment for membership %s", rowMembershipID)
		}

		roles = append(roles, application.RoleListItem{
			ID:             *roleID,
			OrganizationID: *roleOrganizationID,
			Name:           *name,
			Slug:           *slug,
			IsSystem:       *isSystem,
			CreatedAt:      *createdAt,
			UpdatedAt:      *updatedAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate list membership roles: %w", err)
	}

	if !membershipFound {
		return nil, application.ErrMembershipNotFound
	}

	return roles, nil
}
