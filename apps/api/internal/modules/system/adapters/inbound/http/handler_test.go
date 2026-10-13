package httpadapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/system/application"
)

// Stub implementation of the input port.
type stubCheckHealth struct {
	status string
}

func (s stubCheckHealth) Execute(
	ctx context.Context,
) application.HealthResult {
	return application.HealthResult{
		Status: s.status,
	}
}

func TestGetHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Inject the stub instead of the real service.
	handler := NewHandler(stubCheckHealth{
		status: "ok",
	})

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/health",
		nil,
	)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected HTTP 200, got %d",
			response.Code,
		)
	}

	var body struct {
		Status string `json:"status"`
	}

	if err := json.Unmarshal(
		response.Body.Bytes(),
		&body,
	); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf(
			"expected status ok, got %s",
			body.Status,
		)
	}
}
