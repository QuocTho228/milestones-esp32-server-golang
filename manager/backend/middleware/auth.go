package middleware

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

// Hàm min dùng để lấy giá trị nhỏ nhất giữa hai số nguyên
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

var jwtSecret = []byte("milestones_admin_secret_key")

// Tạo JWT Token
func GenerateToken(userID uint, username, role string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// Phân tích JWT Token
func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrInvalidKey
}

// Middleware xác thực JWT
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Thêm log debug
		log.Printf("[JWTAuth] Xử lý yêu cầu: %s %s, IP client: %s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
		
		authHeader := c.GetHeader("Authorization")
		log.Printf("[JWTAuth] Header Authorization: %s", authHeader)
		
		if authHeader == "" {
			log.Printf("[JWTAuth] ❌ Thiếu header xác thực")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Thiếu header xác thực"})
			c.Abort()
			return
		}

		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)
		log.Printf("[JWTAuth] Độ dài token đã trích xuất: %d, Tiền tố: %s", len(tokenString), tokenString[:min(20, len(tokenString))])
		
		claims, err := ParseToken(tokenString)
		if err != nil {
			log.Printf("[JWTAuth] ❌ Phân tích token thất bại: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token không hợp lệ"})
			c.Abort()
			return
		}

		log.Printf("[JWTAuth] ✅ Xác thực token thành công - ID người dùng: %d, Tên đăng nhập: %s, Vai trò: %s", claims.UserID, claims.Username, claims.Role)
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// Middleware quyền quản trị viên
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Yêu cầu quyền quản trị viên"})
			c.Abort()
			return
		}
		c.Next()
	}
}