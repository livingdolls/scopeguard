package ratelimit

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type Rule struct {
	Scope  string
	Limit  int
	Window time.Duration
}

func Reject(c *gin.Context, decision Decision) {
	seconds := int(math.Ceil(
		decision.RetryAfter.Seconds(),
	))

	if seconds < 1 {
		seconds = 1
	}

	c.Header("Retry-After", strconv.Itoa(seconds))
	c.Header("Cache-Control", "no-store")

	c.AbortWithStatusJSON(
		http.StatusTooManyRequests,
		gin.H{
			"error": "too many requests",
		})
}

func Unavailable(c *gin.Context) {
	c.Header("Cache-Control", "no-store")

	c.AbortWithStatusJSON(
		http.StatusServiceUnavailable,
		gin.H{
			"error": "service unavailable",
		})
}

func LimitIP(limiter Limiter, rules map[string]Rule) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Request.Method + " " + c.Request.URL.Path

		rule, found := rules[key]

		if !found {
			c.Next()
			return
		}

		clientIP := net.ParseIP(c.ClientIP())

		if clientIP == nil {
			Unavailable(c)
			return
		}

		decision, err := limiter.Allow(
			c.Request.Context(),
			rule.Scope,
			clientIP.String(),
			rule.Limit,
			rule.Window,
		)

		if err != nil {
			Unavailable(c)
			return
		}

		if !decision.Allowed {
			Reject(c, decision)
			return
		}

		c.Next()
	}
}
