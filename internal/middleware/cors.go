package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS returns a CORS middleware handler.
// allowedOrigins can be "*" for all origins or a comma-separated list of specific origins.
//
// With "*" the response is a plain wildcard WITHOUT Allow-Credentials: reflecting
// any origin together with credentials would let every website make credentialed
// requests. The API authenticates with a Bearer header, which does not need
// credentials mode. Credentials are only allowed for an explicit origin list.
func CORS(allowedOrigins string) gin.HandlerFunc {
	allowAll := strings.TrimSpace(allowedOrigins) == "*"
	origins := make(map[string]bool)
	for _, o := range strings.Split(allowedOrigins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins[o] = true
		}
	}

	const allowHeaders = "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With"
	const allowMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if origin != "" {
			switch {
			case allowAll:
				c.Header("Access-Control-Allow-Origin", "*")
				c.Header("Access-Control-Allow-Headers", allowHeaders)
				c.Header("Access-Control-Allow-Methods", allowMethods)
			case origins[origin]:
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Allow-Headers", allowHeaders)
				c.Header("Access-Control-Allow-Methods", allowMethods)
				c.Header("Vary", "Origin")
			}
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
