package httpadapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/authctx"
)

type membershipListContextKey struct{}

type fakeListMembershipRolesService struct {
	gotContext context.Context
}

func (f *fakeListMembershipRolesService) Execute(
	ctx context.Context,
	organizationID uuid.UUID,
	membershipID uuid.UUID,
) ([]application.RoleListItem, error) {
	f.gotContext = ctx
	return []application.RoleListItem{}, nil
}

func TestListMembershipRolesPropagatesContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeListMembershipRolesService{}
	handler := &Handler{listMembershipRoles: service}

	router := gin.New()
	router.GET(
		"/v1/organizations/:organization_id/memberships/:membership_id/roles",
		handler.ListMembershipRoles,
	)

	ctx := context.WithValue(
		context.Background(),
		membershipListContextKey{},
		"expected",
	)
	ctx = authctx.WithUserID(ctx, uuid.New())

	path := "/v1/organizations/" + uuid.NewString() +
		"/memberships/" + uuid.NewString() + "/roles"

	request := httptest.NewRequest(
		http.MethodGet, path, nil,
	).WithContext(ctx)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	if service.gotContext == nil {
		t.Fatal("service was not called")
	}

	if got := service.gotContext.Value(
		membershipListContextKey{},
	); got != "expected" {
		t.Fatal("request context was not propagated")
	}
}
