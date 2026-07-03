package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// InternalServiceAuth Middleware xác thực dịch vụ nội bộ.
// Yêu cầu: Authorization: Bearer <token>
func InternalServiceAuth(expectedToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Thiếu thông tin xác thực dịch vụ nội bộ"})
			c.Abort()
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if token == "" || token != expectedToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Thông tin xác thực dịch vụ nội bộ không hợp lệ"})
			c.Abort()
			return
		}

		c.Next()
	}
}