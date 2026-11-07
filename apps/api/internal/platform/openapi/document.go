// Package openapi serves the API contract used by clients such as Apidog.
package openapi

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.json
var document []byte

// RegisterRoutes exposes the same document exported by make api-export.
func RegisterRoutes(router gin.IRoutes) {
	router.GET("/openapi.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", document)
	})
}
