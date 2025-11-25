package middleware

import (
	"github.com/gin-gonic/gin"
)

// SetupMiddleware 设置所有全局中间件
func SetupMiddleware(router *gin.Engine) {
	// CORS中间件（必须在最前面）
	router.Use(CORS())
	
	// 其他全局中间件可以在这里添加
	// 例如：日志、恢复、限流等
}

// SetupMiddlewareWithConfig 使用自定义配置设置中间件
func SetupMiddlewareWithConfig(router *gin.Engine, allowOrigins []string) {
	// 自定义CORS配置
	router.Use(CORSWithConfig(allowOrigins))
}
