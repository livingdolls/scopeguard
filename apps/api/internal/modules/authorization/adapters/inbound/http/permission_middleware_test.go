package httpadapter

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/authctx"
)

type fakeEvaluator struct {
	calls int

	userID uuid.UUID
	orgID  uuid.UUID
	code   string

	allowed bool
	err     error
}

func (f *fakeEvaluator) Can(
	ctx context.Context,
	userID uuid.UUID,
	orgID uuid.UUID,
	code string,
) (bool, error) {
	f.calls++
	f.userID = userID
	f.orgID = orgID
	f.code = code

	return f.allowed, f.err
}

func testPermissionRouter(
	t *testing.T,
	evaluator *fakeEvaluator,
	authenticatedUser uuid.UUID,
) (*gin.Engine, *bool) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	guard := NewPermissionGuard(evaluator, logger)

	middleware, err := guard.RequirePermission(
		domain.PermissionRoleRead,
	)
	if err != nil {
		t.Fatal(err)
	}

	reachedHandler := false

	router.GET(
		"/v1/organizations/:organization_id/roles",
		func(c *gin.Context) {
			if authenticatedUser != uuid.Nil {
				ctx := authctx.WithUserID(
					c.Request.Context(),
					authenticatedUser,
				)
				c.Request = c.Request.WithContext(ctx)
			}
			c.Next()
		},
		middleware,
		func(c *gin.Context) {
			reachedHandler = true
			c.Status(http.StatusNoContent)
		},
	)

	return router, &reachedHandler
}

func TestRequirePermissionRejectsWildcardAtRegistration(t *testing.T) {
	guard := NewPermissionGuard(&fakeEvaluator{}, nil)

	_, err := guard.RequirePermission("role:*")

	if !errors.Is(err, domain.ErrInvalidPermissionCode) {
		t.Fatalf("expected invalid permission, got %v", err)
	}
}

func TestRequirePermissionAllowed(t *testing.T) {
	userID := uuid.New()
	organizationID := uuid.New()

	evaluator := &fakeEvaluator{allowed: true}

	router, reached := testPermissionRouter(
		t, evaluator, userID,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/organizations/"+organizationID.String()+"/roles",
		nil,
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	if response.Code != http.StatusNoContent {
		t.Fatalf("unexpected status %d", response.Code)
	}

	if !*reached {
		t.Fatal("authorized handler was not executed")
	}

	if evaluator.calls != 1 ||
		evaluator.userID != userID ||
		evaluator.orgID != organizationID ||
		evaluator.code != domain.PermissionRoleRead {
		t.Fatal("evaluator received incorrect arguments")
	}
}

func TestRequirePermissionFailsClosed(t *testing.T) {
	evaluator := &fakeEvaluator{
		allowed: true,
		err:     errors.New("database unavailable"),
	}

	router, reached := testPermissionRouter(
		t, evaluator, uuid.New(),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/organizations/"+uuid.NewString()+"/roles",
		nil,
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"expected 503, got %d",
			response.Code,
		)
	}

	if *reached {
		t.Fatal("handler ran despite evaluator failure")
	}
}
