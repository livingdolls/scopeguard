package api

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	authorizationhttp "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/adapters/inbound/http"
	authorizationpostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/adapters/outbound/postgres"
	authorizationapp "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

type authorizationModule struct {
	handler *authorizationhttp.Handler
	guard   *authorizationhttp.PermissionGuard
}

func newAuthorization(pool *pgxpool.Pool, logger *slog.Logger) authorizationModule {
	permissionReader := authorizationpostgres.NewPermissionReader(pool)

	evaluator := authorizationapp.NewAuthorizationEvaluatorService(permissionReader)

	catalogRepository := authorizationpostgres.NewPermissionCatalogRepository(pool)

	listPermission := authorizationapp.NewListPermissionService(catalogRepository)

	roleReader := authorizationpostgres.NewRoleReader(pool)
	listRoles := authorizationapp.NewListRolesService(roleReader)

	roleRepository := authorizationpostgres.NewRoleRepository(pool)
	createRole := authorizationapp.NewCreateRoleService(roleRepository)

	rolePermissionRepository := authorizationpostgres.NewRolePermissionRepository(pool)
	grantRolePermission := authorizationapp.NewGrantRolePermissionService(rolePermissionRepository)

	return authorizationModule{
		handler: authorizationhttp.NewHandler(listPermission, listRoles, createRole, grantRolePermission, logger),
		guard:   authorizationhttp.NewPermissionGuard(evaluator, logger),
	}
}
