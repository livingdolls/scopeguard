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

type removeRoleContextKey struct{}

type fakeRemoveRoleService struct {
	gotContext context.Context
	gotInput   application.RemoveRoleFromMemberInput
}

func (f *fakeRemoveRoleService) Execute(
	ctx context.Context,
	input application.RemoveRoleFromMemberInput,
) (application.RemoveRoleFromMemberOutput, error) {
	f.gotContext = ctx
	f.gotInput = input

	return application.RemoveRoleFromMemberOutput{
		Removed: true,
	}, nil
}

func TestRemoveRoleHandlerPropagatesRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fakeService := &fakeRemoveRoleService{}
	handler := &Handler{
		removeRoleFromMember: fakeService,
	}

	actorID := uuid.New()
	organizationID := uuid.New()
	membershipID := uuid.New()
	roleID := uuid.New()

	router := gin.New()
	router.DELETE(
		"/v1/organizations/:organization_id/memberships/:membership_id/roles/:role_id",
		handler.RemoveRoleFromMember,
	)

	ctx := context.WithValue(
		context.Background(),
		removeRoleContextKey{},
		"request-value",
	)
	ctx = authctx.WithUserID(ctx, actorID)

	path := "/v1/organizations/" +
		organizationID.String() +
		"/memberships/" +
		membershipID.String() +
		"/roles/" +
		roleID.String()

	request := httptest.NewRequest(
		http.MethodDelete, path, nil,
	).WithContext(ctx)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected HTTP 200, got %d",
			response.Code,
		)
	}

	if fakeService.gotContext == nil {
		t.Fatal("service was not executed")
	}

	if got := fakeService.gotContext.Value(
		removeRoleContextKey{},
	); got != "request-value" {
		t.Fatal("request context was not propagated")
	}

	if fakeService.gotInput.ActorUserID != actorID ||
		fakeService.gotInput.OrganizationID != organizationID ||
		fakeService.gotInput.MembershipID != membershipID ||
		fakeService.gotInput.RoleID != roleID {
		t.Fatal("incorrect identifiers passed to service")
	}
}
