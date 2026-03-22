package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders adds standard security headers to every response.
func SecurityHeaders(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
	c.Header("Content-Security-Policy",
		"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:;")
	c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	if c.Request.TLS != nil {
		c.Header("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
	}
	c.Next()
}
