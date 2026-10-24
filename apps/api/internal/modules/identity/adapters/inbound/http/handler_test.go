package httpadapter

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/authctx"
)

type registerStub struct {
	calls int
	input application.RegisterUserInput
	err   error
}

func (s *registerStub) Execute(
	ctx context.Context,
	input application.RegisterUserInput,
) (application.RegisterUserOutput, error) {
	s.calls++
	s.input = input
	return application.RegisterUserOutput{}, s.err
}

type verifyStub struct {
	calls int
	token string
	err   error
}

func (s *verifyStub) Execute(
	ctx context.Context,
	token string,
) error {
	s.calls++
	s.token = token
	return s.err
}

type loginStub struct {
	calls  int
	input  application.LoginUserInput
	output application.LoginUserOutput
	err    error
}

func (s *loginStub) Execute(
	ctx context.Context,
	input application.LoginUserInput,
) (application.LoginUserOutput, error) {
	s.calls++
	s.input = input
	return s.output, s.err
}

type profileStub struct {
	calls  int
	userID uuid.UUID
	output application.UserProfile
	err    error
}

func (s *profileStub) Execute(
	ctx context.Context,
	userID uuid.UUID,
) (application.UserProfile, error) {
	s.calls++
	s.userID = userID
	return s.output, s.err
}

func newAuthTestRouter(
	register *registerStub,
	verify *verifyStub,
	login *loginStub,
	profile *profileStub,
	authenticated bool,
	userID uuid.UUID,
) *gin.Engine {
	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	handler := NewHandler(
		register, verify, login, profile, logger,
	)

	router := gin.New()

	v1 := router.Group("/v1")
	handler.RegisterPublicRoutes(v1)

	protected := router.Group("/v1")

	if authenticated {
		// Test-only trusted identity, not production auth.
		protected.Use(func(c *gin.Context) {
			ctx := authctx.WithUserID(
				c.Request.Context(),
				userID,
			)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
		})
	}

	handler.RegisterProtectedRoutes(protected)

	return router
}

func performAuthRequest(
	router *gin.Engine,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(
		method,
		path,
		strings.NewReader(body),
	)

	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func TestRegisterHTTP(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		serviceErr error
		wantStatus int
		wantCalls  int
	}{
		{
			name:       "accepted",
			body:       `{"email":"user@example.com","password":"correct horse battery staple"}`,
			wantStatus: http.StatusAccepted,
			wantCalls:  1,
		},
		{
			name:       "duplicate email has generic response",
			body:       `{"email":"user@example.com","password":"correct horse battery staple"}`,
			serviceErr: application.ErrEmailAlreadyExists,
			wantStatus: http.StatusAccepted,
			wantCalls:  1,
		},
		{
			name:       "unknown JSON field",
			body:       `{"email":"user@example.com","password":"correct horse battery staple","is_admin":true}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "multiple JSON values",
			body:       `{} {}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "oversized body",
			body:       `{"email":"` + strings.Repeat("a", 17000) + `"}`,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			register := &registerStub{err: tt.serviceErr}

			router := newAuthTestRouter(
				register,
				&verifyStub{},
				&loginStub{},
				&profileStub{},
				false,
				uuid.Nil,
			)

			response := performAuthRequest(
				router,
				http.MethodPost,
				"/v1/auth/register",
				tt.body,
			)

			if response.Code != tt.wantStatus {
				t.Fatalf(
					"expected %d, got %d: %s",
					tt.wantStatus,
					response.Code,
					response.Body.String(),
				)
			}

			if register.calls != tt.wantCalls {
				t.Fatalf(
					"expected %d service calls, got %d",
					tt.wantCalls,
					register.calls,
				)
			}

			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("expected no-store response header")
			}
		})
	}
}

func TestLoginHTTP(t *testing.T) {
	login := &loginStub{
		output: application.LoginUserOutput{
			AccessToken: "test-jwt",
			TokenType:   "Bearer",
			ExpiresAt:   time.Now().Add(15 * time.Minute),
		},
	}

	router := newAuthTestRouter(
		&registerStub{},
		&verifyStub{},
		login,
		&profileStub{},
		false,
		uuid.Nil,
	)

	response := performAuthRequest(
		router,
		http.MethodPost,
		"/v1/auth/login",
		`{"email":"user@example.com","password":"correct horse battery staple"}`,
	)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}

	if !strings.Contains(response.Body.String(), `"test-jwt"`) {
		t.Fatal("missing access token")
	}

	login.err = application.ErrInvalidCredentials

	response = performAuthRequest(
		router,
		http.MethodPost,
		"/v1/auth/login",
		`{"email":"user@example.com","password":"wrong password"}`,
	)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected HTTP 401, got %d", response.Code)
	}

	if strings.Contains(response.Body.String(), "user not found") {
		t.Fatal("login response leaked account existence")
	}
}

func TestVerifyEmailHTTP(t *testing.T) {
	verify := &verifyStub{}

	router := newAuthTestRouter(
		&registerStub{},
		verify,
		&loginStub{},
		&profileStub{},
		false,
		uuid.Nil,
	)

	response := performAuthRequest(
		router,
		http.MethodPost,
		"/v1/auth/verify-email",
		`{"token":"test-verification-token"}`,
	)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", response.Code)
	}

	if verify.token != "test-verification-token" {
		t.Fatal("verification service received incorrect token")
	}

	verify.err = application.ErrInvalidVerificationToken

	response = performAuthRequest(
		router,
		http.MethodPost,
		"/v1/auth/verify-email",
		`{"token":"expired-token"}`,
	)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

func TestMeRequiresAuthentication(t *testing.T) {
	profile := &profileStub{}

	router := newAuthTestRouter(
		&registerStub{},
		&verifyStub{},
		&loginStub{},
		profile,
		false,
		uuid.Nil,
	)

	response := performAuthRequest(
		router,
		http.MethodGet,
		"/v1/auth/me",
		"",
	)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected HTTP 401, got %d", response.Code)
	}

	if profile.calls != 0 {
		t.Fatal("unauthenticated request called profile service")
	}
}

func TestMeAuthenticated(t *testing.T) {
	userID := uuid.New()

	profile := &profileStub{
		output: application.UserProfile{
			ID:    userID,
			Email: "user@example.com",
		},
	}

	router := newAuthTestRouter(
		&registerStub{},
		&verifyStub{},
		&loginStub{},
		profile,
		true,
		userID,
	)

	response := performAuthRequest(
		router,
		http.MethodGet,
		"/v1/auth/me",
		"",
	)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}

	if profile.userID != userID {
		t.Fatal("profile service received incorrect user ID")
	}

	if !strings.Contains(
		response.Body.String(),
		"user@example.com",
	) {
		t.Fatal("missing user email")
	}
}
