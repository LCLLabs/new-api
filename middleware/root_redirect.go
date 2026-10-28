package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// RootRedirect issues a 302 for exact "/" when the site option is enabled and
// the configured destination URL is valid. Other paths are untouched.
func RootRedirect() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Next()
			return
		}
		if c.Request.URL.Path != "/" {
			c.Next()
			return
		}
		if !common.RootRedirectEnabled {
			c.Next()
			return
		}
		target := common.RootRedirectURL
		if err := common.ValidateRootRedirectURL(target); err != nil {
			c.Next()
			return
		}
		c.Redirect(http.StatusFound, target)
		c.Abort()
	}
}
