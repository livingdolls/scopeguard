package httpadapter

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

const maxJSONBodySize = 16 << 10

var errUnsupportedMediaType = errors.New("unsupported media type")

func decodeStrictJSON(c *gin.Context, dst any) error {
	if c.ContentType() != "application/json" {
		return errUnsupportedMediaType
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodySize)

	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	// require OEF after the first JSON value
	var extra any

	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}

		return err
	}

	return nil
}

func writeJSONDecoderError(c *gin.Context, err error) {
	var maxBytesErr *http.MaxBytesError

	switch {
	case errors.Is(err, errUnsupportedMediaType):
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "application/json required"})

	case errors.As(err, &maxBytesErr):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON request"})
	}

}
