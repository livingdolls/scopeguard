package application

import (
	"context"
	"testing"
)

func TestHealthServiceExecute(t *testing.T) {
	service := NewHealthService()

	result := service.Execute(context.Background())

	if result.Status != "ok" {
		t.Errorf(
			"expected status ok, got %s",
			result.Status,
		)
	}
}
