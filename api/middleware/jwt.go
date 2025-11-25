package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTMiddleware JWT中间件配置
type JWTMiddleware struct {
	Secret              []byte
	TokenExpireDuration time.Duration
}

// NewJWTMiddleware 创建JWT中间件
func NewJWTMiddleware(secret []byte, expireDuration time.Duration) *JWTMiddleware {
	return &JWTMiddleware{
		Secret:              secret,
		TokenExpireDuration: expireDuration,
	}
}

// Claims JWT声明
type Claims struct {
	UserID       int64  `json:"user_id"`
	Email        string `json:"email"`
	Nickname     string `json:"nickname"`
	MemberLevel  int    `json:"member_level"`
	jwt.RegisteredClaims
}

// GenerateToken 生成JWT Token
func (jm *JWTMiddleware) GenerateToken(userID int64, email, nickname string, memberLevel int) (string, error) {
	claims := Claims{
		UserID:      userID,
		Email:       email,
		Nickname:    nickname,
		MemberLevel: memberLevel,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jm.TokenExpireDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jm.Secret)
}

// ParseToken 解析JWT Token
func (jm *JWTMiddleware) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jm.Secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// JWTAuth JWT认证中间件（必须登录）
func (jm *JWTMiddleware) JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// 尝试从query参数获取
			authHeader = c.Query("token")
			if authHeader == "" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code":    401,
					"message": "请求未携带token，无权限访问",
				})
				c.Abort()
				return
			}
		}

		// 按空格分割
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			// 如果不是Bearer格式，直接使用整个字符串作为token
			parts = []string{"", authHeader}
		}

		// 解析token
		claims, err := jm.ParseToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "无效的token",
				"error":   err.Error(),
			})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("nickname", claims.Nickname)
		c.Set("member_level", claims.MemberLevel)
		c.Set("claims", claims)

		c.Next()
	}
}

// OptionalAuth 可选认证中间件（不强制登录，但如果有token会解析）
func (jm *JWTMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			authHeader = c.Query("token")
		}

		if authHeader != "" {
			// 按空格分割
			parts := strings.SplitN(authHeader, " ", 2)
			if !(len(parts) == 2 && parts[0] == "Bearer") {
				parts = []string{"", authHeader}
			}

			// 解析token
			claims, err := jm.ParseToken(parts[1])
			if err == nil {
				// token有效，存储用户信息
				c.Set("user_id", claims.UserID)
				c.Set("email", claims.Email)
				c.Set("nickname", claims.Nickname)
				c.Set("member_level", claims.MemberLevel)
				c.Set("claims", claims)
			}
		}

		c.Next()
	}
}

// GetUserID 从上下文获取用户ID
func GetUserID(c *gin.Context) (int64, bool) {
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(int64); ok {
			return id, true
		}
	}
	return 0, false
}

// GetUserEmail 从上下文获取用户邮箱
func GetUserEmail(c *gin.Context) (string, bool) {
	if email, exists := c.Get("email"); exists {
		if e, ok := email.(string); ok {
			return e, true
		}
	}
	return "", false
}

// GetUserClaims 从上下文获取完整的用户声明
func GetUserClaims(c *gin.Context) (*Claims, bool) {
	if claims, exists := c.Get("claims"); exists {
		if c, ok := claims.(*Claims); ok {
			return c, true
		}
	}
	return nil, false
}

// RequireMemberLevel 要求特定会员等级的中间件
func RequireMemberLevel(minLevel int) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberLevel, exists := c.Get("member_level")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "未登录",
			})
			c.Abort()
			return
		}

		level, ok := memberLevel.(int)
		if !ok || level < minLevel {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "会员等级不足",
				"required": minLevel,
				"current":  level,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
