package httpadapter

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/system/application"
)

type ReadinessHandler struct {
	checkReadiness application.CheckReadiness
	logger         *slog.Logger
}

func NewReadinessHandler(
	checkReadiness application.CheckReadiness,
	logger *slog.Logger,
) *ReadinessHandler {
	return &ReadinessHandler{
		checkReadiness: checkReadiness,
		logger:         logger,
	}
}

func (h *ReadinessHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/v1/ready", h.GetReadiness)
}

func (h *ReadinessHandler) GetReadiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.checkReadiness.Execute(ctx); err != nil {
		h.logger.Warn("readiness check failed", "error", err)

		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}
