package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimiter 限流中间件
type RateLimiter struct {
	client *redis.Client
}

// NewRateLimiter 创建限流中间件
func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{
		client: client,
	}
}

// GlobalLimit 全局限流中间件（100请求/分钟/IP）
func (rl *RateLimiter) GlobalLimit() gin.HandlerFunc {
	return rl.limitByIP("global", 100, time.Minute)
}

// LoginLimit 登录限流中间件（5次/分钟/IP）
func (rl *RateLimiter) LoginLimit() gin.HandlerFunc {
	return rl.limitByIP("login", 5, time.Minute)
}

// RegisterLimit 注册限流中间件（3次/分钟/IP）
func (rl *RateLimiter) RegisterLimit() gin.HandlerFunc {
	return rl.limitByIP("register", 3, time.Minute)
}

// SendCodeLimit 发送验证码限流（基于IP，3次/分钟）
func (rl *RateLimiter) SendCodeLimit() gin.HandlerFunc {
	return rl.limitByIP("send_code", 3, time.Minute)
}

// limitByIP 基于IP的限流
func (rl *RateLimiter) limitByIP(prefix string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("rate_limit:%s:%s", prefix, ip)
		
		if !rl.checkLimit(key, limit, window) {
			log.Printf("WARN: Rate limit exceeded for %s from IP: %s", prefix, ip)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "请求过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// checkLimit 检查限流（滑动窗口算法）
func (rl *RateLimiter) checkLimit(key string, limit int, window time.Duration) bool {
	ctx := context.Background()
	now := time.Now().UnixNano()
	windowStart := now - window.Nanoseconds()
	
	// 使用Redis的有序集合实现滑动窗口
	pipe := rl.client.Pipeline()
	
	// 1. 删除窗口外的旧记录
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	
	// 2. 统计当前窗口内的请求数
	countCmd := pipe.ZCard(ctx, key)
	
	// 3. 设置过期时间
	pipe.Expire(ctx, key, window)
	
	// 执行管道
	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("ERROR: Failed to check rate limit for key %s: %v", key, err)
		// 发生错误时允许请求通过（降级策略）
		return true
	}
	
	// 检查是否超过限制（在添加之前检查）
	count := countCmd.Val()
	if count >= int64(limit) {
		return false
	}
	
	// 4. 添加当前请求（只有在未超限时才添加）
	err = rl.client.ZAdd(ctx, key, redis.Z{
		Score:  float64(now),
		Member: now,
	}).Err()
	if err != nil {
		log.Printf("ERROR: Failed to add request to rate limit key %s: %v", key, err)
	}
	
	return true
}
