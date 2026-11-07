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

type assignmentContextKey struct{}

type fakeAssignRoleToMemberService struct {
	gotContext context.Context
}

func (f *fakeAssignRoleToMemberService) Execute(
	ctx context.Context,
	input application.AssignRoleToMemberInput,
) (application.AssignRoleToMemberOutput, error) {
	f.gotContext = ctx

	return application.AssignRoleToMemberOutput{
		Assigned: true,
	}, nil
}

func TestAssignRoleHandlerPropagatesRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fakeService := &fakeAssignRoleToMemberService{}
	handler := &Handler{
		assignRoleToMember: fakeService,
	}

	actorID := uuid.New()
	orgID := uuid.New()
	membershipID := uuid.New()
	roleID := uuid.New()

	router := gin.New()
	router.POST(
		"/v1/organizations/:organization_id/memberships/:membership_id/roles/:role_id",
		handler.AssignRoleToMember,
	)

	ctx := context.WithValue(
		context.Background(),
		assignmentContextKey{},
		"request-value",
	)
	ctx = authctx.WithUserID(ctx, actorID)

	path := "/v1/organizations/" +
		orgID.String() +
		"/memberships/" +
		membershipID.String() +
		"/roles/" +
		roleID.String()

	req := httptest.NewRequest(http.MethodPost, path, nil)
	req = req.WithContext(ctx)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected HTTP 201, got %d",
			response.Code,
		)
	}

	if fakeService.gotContext == nil {
		t.Fatal("service was not called")
	}

	if got := fakeService.gotContext.Value(
		assignmentContextKey{},
	); got != "request-value" {
		t.Fatal("handler did not propagate request context")
	}
}
