package httpadapter

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterRoutes(authorized *gin.RouterGroup) {
	authorized.POST("/organizations", h.CreateOrganization)
}
