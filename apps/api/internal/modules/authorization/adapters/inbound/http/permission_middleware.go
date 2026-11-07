package httpadapter

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/authctx"
)

type PermissionGuard struct {
	evaluator application.AuthorizationEvaluator
	logger    *slog.Logger
}

func NewPermissionGuard(evaluator application.AuthorizationEvaluator, logger *slog.Logger) *PermissionGuard {
	if logger == nil {
		logger = slog.Default()
	}
	return &PermissionGuard{
		evaluator: evaluator,
		logger:    logger,
	}
}

func (g *PermissionGuard) RequirePermission(permissionCode string) (gin.HandlerFunc, error) {
	permission, err := domain.ParsePermission(permissionCode)
	if err != nil {
		return nil, fmt.Errorf("invalid route permission: %w", err)
	}

	// validate at route registration tim
	code := permission.Code()

	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")

		userID, authenticated := authctx.UserIDFromContext(c.Request.Context())

		if !authenticated {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		organizationID, err := uuid.Parse(c.Param("organization_id"))

		if err != nil || organizationID == uuid.Nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid organization_id"})
			return
		}

		allowed, err := g.evaluator.Can(c.Request.Context(), userID, organizationID, code)
		if err != nil {
			g.logger.Error("authorization evaluation failed", "err", err)

			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "authorization unavailable"})

			return
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		c.Next()
	}, nil
}
