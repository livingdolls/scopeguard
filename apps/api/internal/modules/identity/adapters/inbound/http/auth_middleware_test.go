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

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/authctx"
)

type stubTokenVerifier struct {
	userID uuid.UUID
	calls  int
}

func (s *stubTokenVerifier) Verify(
	rawToken string,
) (uuid.UUID, error) {
	s.calls++

	if rawToken != "valid-test-token" {
		return uuid.Nil, application.ErrInvalidAccessToken
	}

	return s.userID, nil
}

type stubUserChecker struct {
	allowed bool
	err     error
	calls   int
}

func (s *stubUserChecker) CanAuthenticate(
	ctx context.Context,
	id uuid.UUID,
) (bool, error) {
	s.calls++
	return s.allowed, s.err
}

func TestRequireBearerAuth(t *testing.T) {
	tests := []struct {
		name      string
		headers   []string
		allowed   bool
		checkErr  error
		want      int
		wantCalls int
	}{
		{
			name:      "valid token",
			headers:   []string{"Bearer valid-test-token"},
			allowed:   true,
			want:      http.StatusOK,
			wantCalls: 1,
		},
		{
			name: "missing Authorization header",
			want: http.StatusUnauthorized,
		},
		{
			name:    "wrong scheme",
			headers: []string{"Basic abc123"},
			want:    http.StatusUnauthorized,
		},
		{
			name:    "malformed Bearer header",
			headers: []string{"Bearer token-a token-b"},
			want:    http.StatusUnauthorized,
		},
		{
			name:    "invalid token",
			headers: []string{"Bearer invalid-token"},
			want:    http.StatusUnauthorized,
		},
		{
			name:      "user deleted",
			headers:   []string{"Bearer valid-test-token"},
			allowed:   false,
			want:      http.StatusUnauthorized,
			wantCalls: 1,
		},
		{
			name:      "pending verification",
			headers:   []string{"Bearer valid-test-token"},
			allowed:   false,
			want:      http.StatusUnauthorized,
			wantCalls: 1,
		},
		{
			name:      "suspended account",
			headers:   []string{"Bearer valid-test-token"},
			allowed:   false,
			want:      http.StatusUnauthorized,
			wantCalls: 1,
		},
		{
			name:      "database unavailable",
			headers:   []string{"Bearer valid-test-token"},
			checkErr:  errors.New("database unavailable"),
			want:      http.StatusServiceUnavailable,
			wantCalls: 1,
		},
		{
			name: "duplicate Authorization headers",
			headers: []string{
				"Bearer valid-test-token",
				"Bearer valid-test-token",
			},
			want: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := uuid.New()

			verifier := &stubTokenVerifier{
				userID: userID,
			}
			checker := &stubUserChecker{
				allowed: tt.allowed,
				err:     tt.checkErr,
			}

			logger := slog.New(
				slog.NewTextHandler(io.Discard, nil),
			)

			router := gin.New()

			router.GET(
				"/protected",
				RequireBearerAuth(verifier, checker, logger),
				func(c *gin.Context) {
					id, ok := authctx.UserIDFromContext(
						c.Request.Context(),
					)

					if !ok || id != userID {
						c.Status(http.StatusInternalServerError)
						return
					}

					c.Status(http.StatusOK)
				},
			)

			req := httptest.NewRequest(
				http.MethodGet,
				"/protected",
				nil,
			)

			for _, value := range tt.headers {
				req.Header.Add("Authorization", value)
			}

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != tt.want {
				t.Fatalf(
					"expected %d, got %d: %s",
					tt.want,
					recorder.Code,
					recorder.Body.String(),
				)
			}

			if checker.calls != tt.wantCalls {
				t.Fatalf(
					"expected %d user checks, got %d",
					tt.wantCalls,
					checker.calls,
				)
			}
		})
	}
}
