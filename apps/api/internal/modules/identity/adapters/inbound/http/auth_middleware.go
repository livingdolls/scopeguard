package httpadapter

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/authctx"
)

func RequireBearerAuth(
	verifier application.AccessTokenVerifier,
	users application.UserAccessChecker,
	logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		headers := c.Request.Header.Values("Authorization")

		// require exactly one Authorization header
		// example value Bearer raw_token
		if len(headers) != 1 {
			rejectUnauthorized(c)
			return
		}

		parts := strings.Fields(headers[0])

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			rejectUnauthorized(c)
			return
		}

		rawToken := parts[1]

		// verify cryptographic signature and jwt claims
		userID, err := verifier.Verify(rawToken)

		if err != nil || userID == uuid.Nil {
			rejectUnauthorized(c)
			return
		}

		// ensure the user is active
		allowed, err := users.CanAuthenticate(c.Request.Context(), userID)
		if err != nil {
			logger.Error("authentication user lookup failed", "error", err)

			c.AbortWithStatusJSON(
				http.StatusServiceUnavailable,
				gin.H{"error": "authentication temporary unavailable"},
			)

			return
		}

		if !allowed {
			rejectUnauthorized(c)
			return
		}

		// inject authenticated identity into request context
		ctx := authctx.WithUserID(c.Request.Context(), userID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func rejectUnauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", "Bearer")

	c.AbortWithStatusJSON(
		http.StatusUnauthorized,
		gin.H{
			"error": "Unauthorized",
		})
}
