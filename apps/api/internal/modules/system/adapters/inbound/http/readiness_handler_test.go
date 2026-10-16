package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type stubCheckReadiness struct {
	err error
}

func (s *stubCheckReadiness) Execute(
	ctx context.Context,
) error {
	return s.err
}

func TestReadinessHandler(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   int
		wantStatus string
	}{
		{
			name:       "ready",
			wantCode:   http.StatusOK,
			wantStatus: "ready",
		},
		{
			name:       "database unavailable",
			err:        errors.New("database password=secret"),
			wantCode:   http.StatusServiceUnavailable,
			wantStatus: "not_ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(
				slog.NewTextHandler(io.Discard, nil),
			)

			service := &stubCheckReadiness{
				err: tt.err,
			}

			handler := NewReadinessHandler(
				service,
				logger,
			)

			router := gin.New()
			handler.RegisterRoutes(router)

			req := httptest.NewRequest(
				http.MethodGet,
				"/v1/ready",
				nil,
			)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != tt.wantCode {
				t.Fatalf(
					"expected HTTP %d, got %d",
					tt.wantCode,
					recorder.Code,
				)
			}

			var response struct {
				Status string `json:"status"`
			}

			if err := json.Unmarshal(
				recorder.Body.Bytes(),
				&response,
			); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if response.Status != tt.wantStatus {
				t.Errorf(
					"expected %s, got %s",
					tt.wantStatus,
					response.Status,
				)
			}
		})
	}
}
