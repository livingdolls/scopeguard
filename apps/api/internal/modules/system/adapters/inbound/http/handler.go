package httpadapter

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/system/application"
)

type Handler struct {
	checkHealth application.CheckHealth
}

func NewHandler(checkHealth application.CheckHealth) *Handler {
	return &Handler{
		checkHealth: checkHealth,
	}
}

func (h *Handler) RegisterRoutes(router *gin.Engine) {
	router.GET("/v1/health", h.GetHealth)
}

func (h *Handler) GetHealth(c *gin.Context) {
	result := h.checkHealth.Execute(c.Request.Context())

	c.JSON(http.StatusOK, gin.H{
		"status": result.Status,
	})
}
