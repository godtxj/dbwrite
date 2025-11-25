package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dchest/captcha"
	"github.com/redis/go-redis/v9"
)

// CaptchaService 图形验证码服务
type CaptchaService struct {
	client *redis.Client
}

// NewCaptchaService 创建图形验证码服务
func NewCaptchaService(client *redis.Client) *CaptchaService {
	return &CaptchaService{
		client: client,
	}
}

// GenerateCaptcha 生成图形验证码
func (s *CaptchaService) GenerateCaptcha() (id string, err error) {
	// 生成验证码ID（使用captcha库）
	id = captcha.NewLen(6) // 6位数字+字母
	
	// 存储到Redis，有效期5分钟
	ctx := context.Background()
	key := s.getKey(id)
	
	// 标记验证码已生成（值为生成时间）
	err = s.client.Set(ctx, key, time.Now().Unix(), 5*time.Minute).Err()
	if err != nil {
		log.Printf("ERROR: Failed to store captcha id %s: %v", id, err)
		return "", fmt.Errorf("生成验证码失败")
	}
	
	log.Printf("INFO: Captcha generated: %s", id)
	return id, nil
}

// VerifyCaptcha 验证图形验证码
func (s *CaptchaService) VerifyCaptcha(id, value string) bool {
	// 检查验证码是否存在
	ctx := context.Background()
	key := s.getKey(id)
	
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		log.Printf("ERROR: Failed to check captcha %s: %v", id, err)
		return false
	}
	
	if exists == 0 {
		log.Printf("WARN: Captcha not found or expired: %s", id)
		return false
	}
	
	// 验证验证码（不区分大小写）
	if !captcha.VerifyString(id, value) {
		log.Printf("WARN: Invalid captcha value for id: %s", id)
		return false
	}
	
	// 验证成功后删除验证码（一次性使用）
	s.client.Del(ctx, key)
	
	log.Printf("INFO: Captcha verified successfully: %s", id)
	return true
}

// getKey 获取Redis key
func (s *CaptchaService) getKey(id string) string {
	return fmt.Sprintf("captcha:%s", id)
}
