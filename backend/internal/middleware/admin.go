package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RequireAdminKey(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if key == "" {
			c.Next()
			return
		}

		provided := c.GetHeader("X-Admin-Key")
		if provided == "" {
			provided = c.Query("admin_key")
		}

		if provided != key {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		c.Next()
	}
}
