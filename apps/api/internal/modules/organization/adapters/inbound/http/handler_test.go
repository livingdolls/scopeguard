package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/domain"
)

type stubCreateOrganization struct {
	output   application.CreateOrganizationOutput
	err      error
	calls    int
	received application.CreateOrganizationInput
}

func (s *stubCreateOrganization) Execute(
	ctx context.Context,
	input application.CreateOrganizationInput,
) (application.CreateOrganizationOutput, error) {
	s.calls++
	s.received = input

	return s.output, s.err
}

func newTestRouter(
	service application.CreateOrganization,
) *gin.Engine {
	router := gin.New()

	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	handler := NewHandler(service, logger)

	// Test-only route group.
	// Production requires authentication and authorization.
	group := router.Group("/v1")
	handler.RegisterRoutes(group)

	return router
}

func performCreateRequest(
	router *gin.Engine,
	body string,
	contentType string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/organizations",
		strings.NewReader(body),
	)

	request.Header.Set("Content-Type", contentType)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	return recorder
}

func TestCreateOrganizationSuccess(t *testing.T) {
	id := uuid.New()
	now := time.Date(
		2026, 10, 17, 8, 0, 0, 0, time.UTC,
	)

	service := &stubCreateOrganization{
		output: application.CreateOrganizationOutput{
			ID:        id,
			Name:      "Acme Corporation",
			Slug:      "acme-corp",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	router := newTestRouter(service)

	response := performCreateRequest(
		router,
		`{"name":"Acme Corporation","slug":"acme-corp"}`,
		"application/json",
	)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected HTTP 201, got %d: %s",
			response.Code,
			response.Body.String(),
		)
	}

	if service.calls != 1 {
		t.Fatalf(
			"expected 1 use case call, got %d",
			service.calls,
		)
	}

	if service.received.Name != "Acme Corporation" {
		t.Error("incorrect name passed to use case")
	}

	if service.received.Slug != "acme-corp" {
		t.Error("incorrect slug passed to use case")
	}

	var result organizationResponse

	if err := json.Unmarshal(
		response.Body.Bytes(),
		&result,
	); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.ID != id {
		t.Error("response ID does not match")
	}

	if result.Name != "Acme Corporation" {
		t.Error("incorrect response name")
	}

	if result.Slug != "acme-corp" {
		t.Error("incorrect response slug")
	}

	if !result.CreatedAt.Equal(now) {
		t.Error("incorrect created_at")
	}
}

func TestCreateOrganizationApplicationErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "invalid organization name",
			err:        domain.ErrInvalidOrganizationName,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid organization slug",
			err:        domain.ErrInvalidOrganizationSlug,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "duplicate slug",
			err:        application.ErrOrganizationSlugAlreadyExists,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "internal server error",
			err:        errors.New("database secret error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &stubCreateOrganization{
				err: tt.err,
			}

			router := newTestRouter(service)

			response := performCreateRequest(
				router,
				`{"name":"Acme","slug":"acme"}`,
				"application/json",
			)

			if response.Code != tt.wantStatus {
				t.Fatalf(
					"expected HTTP %d, got %d",
					tt.wantStatus,
					response.Code,
				)
			}

			if service.calls != 1 {
				t.Fatal("expected one use case call")
			}

			if strings.Contains(
				response.Body.String(),
				"database secret",
			) {
				t.Fatal("internal error leaked to client")
			}
		})
	}
}

func TestCreateOrganizationInvalidRequests(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		wantStatus  int
	}{
		{
			name:        "malformed JSON",
			body:        `{"name":`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name: "unknown JSON field",
			body: `{
                "name":"Acme",
                "slug":"acme",
                "is_admin":true
            }`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "multiple JSON values",
			body:        `{} {}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "unsupported content type",
			body:        `{"name":"Acme","slug":"acme"}`,
			contentType: "text/plain",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name: "request body too large",
			body: `{"name":"` +
				strings.Repeat("a", 17*1024) +
				`","slug":"acme"}`,
			contentType: "application/json",
			wantStatus:  http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &stubCreateOrganization{}
			router := newTestRouter(service)

			response := performCreateRequest(
				router,
				tt.body,
				tt.contentType,
			)

			if response.Code != tt.wantStatus {
				t.Fatalf(
					"expected HTTP %d, got %d: %s",
					tt.wantStatus,
					response.Code,
					response.Body.String(),
				)
			}

			if service.calls != 0 {
				t.Fatal(
					"invalid request must not invoke use case",
				)
			}
		})
	}
}
