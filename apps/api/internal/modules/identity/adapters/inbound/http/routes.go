package httpadapter

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterPublicRoutes(v1 *gin.RouterGroup) {
	auth := v1.Group("/auth")

	auth.POST("/register", h.Register)
	auth.POST("/verify-email", h.VerifyEmail)
	auth.POST("/login", h.Login)
}

func (h *Handler) RegisterProtectedRoutes(authorized *gin.RouterGroup) {
	authorized.GET("/auth/me", h.Me)
}
