package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS permits only configured browser origins and handles preflight before auth.
// Requests without Origin, such as native apps and Bruno, still require normal auth.
func CORS(origins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(origins))
	for _, origin := range origins {
		allowed[origin] = true
	}
	return func(c *gin.Context) {
		c.Writer.Header().Add("Vary", "Origin")
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		// Same-origin Swagger/native web writes do not need a cross-origin grant.
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		if origin == scheme+"://"+c.Request.Host {
			c.Next()
			return
		}
		if !allowed[origin] {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Header("Access-Control-Allow-Origin", origin)
		if c.Request.Method == http.MethodOptions && c.GetHeader("Access-Control-Request-Method") != "" {
			c.Writer.Header().Add("Vary", "Access-Control-Request-Method")
			c.Writer.Header().Add("Vary", "Access-Control-Request-Headers")
			switch c.GetHeader("Access-Control-Request-Method") {
			case "GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS":
			default:
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			for _, header := range strings.Split(c.GetHeader("Access-Control-Request-Headers"), ",") {
				switch strings.ToLower(strings.TrimSpace(header)) {
				case "", "authorization", "content-type", "accept":
				default:
					c.AbortWithStatus(http.StatusForbidden)
					return
				}
			}
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
