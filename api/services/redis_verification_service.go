package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisVerificationService Redis验证码服务
type RedisVerificationService struct {
	client       *redis.Client
	emailService *EmailService
}

// NewRedisVerificationService 创建Redis验证码服务
func NewRedisVerificationService(client *redis.Client, emailService *EmailService) *RedisVerificationService {
	return &RedisVerificationService{
		client:       client,
		emailService: emailService,
	}
}

// SendVerificationCode 发送验证码（存储到Redis并发送邮件）
func (s *RedisVerificationService) SendVerificationCode(email string) (string, error) {
	ctx := context.Background()
	
	// 检查是否在60秒内已发送过验证码
	ttl, err := s.client.TTL(ctx, s.getKey(email)).Result()
	if err != nil && err != redis.Nil {
		log.Printf("ERROR: Failed to check TTL for %s: %v", email, err)
		return "", fmt.Errorf("系统错误")
	}
	
	// 如果还有超过4分钟的有效期，说明刚发送过
	if ttl > 4*time.Minute {
		remainingTime := 60 - int((5*time.Minute-ttl).Seconds())
		log.Printf("WARN: Verification code already sent to %s, please wait %d seconds", email, remainingTime)
		return "", fmt.Errorf("验证码已发送，请等待%d秒后重试", remainingTime)
	}
	
	// 生成新验证码
	code := s.generateCode()
	
	// 存储验证码，有效期5分钟
	err = s.client.Set(ctx, s.getKey(email), code, 5*time.Minute).Err()
	if err != nil {
		log.Printf("ERROR: Failed to store verification code for %s: %v", email, err)
		return "", fmt.Errorf("系统错误")
	}
	
	// 发送邮件
	if s.emailService != nil {
		if err := s.emailService.SendVerificationCode(email, code); err != nil {
			// 邮件发送失败，但验证码已存储，返回错误但不影响验证码使用
			log.Printf("ERROR: Failed to send email to %s: %v", email, err)
			return code, fmt.Errorf("邮件发送失败，请稍后重试")
		}
	}
	
	log.Printf("INFO: Verification code generated and sent to %s: %s (expires in 5 minutes)", email, code)
	return code, nil
}

// VerifyCode 验证验证码
func (s *RedisVerificationService) VerifyCode(email, code string) bool {
	ctx := context.Background()
	
	stored, err := s.client.Get(ctx, s.getKey(email)).Result()
	if err != nil {
		if err == redis.Nil {
			log.Printf("WARN: No verification code found for email: %s", email)
		} else {
			log.Printf("ERROR: Failed to get verification code for %s: %v", email, err)
		}
		return false
	}
	
	// 验证码匹配
	if stored != code {
		log.Printf("WARN: Invalid verification code for email: %s", email)
		return false
	}
	
	log.Printf("INFO: Verification code validated successfully for email: %s", email)
	return true
}

// DeleteCode 删除验证码（验证成功后删除）
func (s *RedisVerificationService) DeleteCode(email string) {
	ctx := context.Background()
	
	err := s.client.Del(ctx, s.getKey(email)).Err()
	if err != nil {
		log.Printf("ERROR: Failed to delete verification code for %s: %v", email, err)
		return
	}
	
	log.Printf("INFO: Verification code deleted for email: %s", email)
}

// getKey 获取Redis key
func (s *RedisVerificationService) getKey(email string) string {
	return fmt.Sprintf("verification:email:%s", email)
}


// generateCode 生成6位数字验证码
func (s *RedisVerificationService) generateCode() string {
	// 使用crypto/rand生成安全的随机数
	const digits = "0123456789"
	b := make([]byte, 6)
	for i := range b {
		// 生成0-9的随机数字
		n := byte(0)
		for {
			randBytes := make([]byte, 1)
			if _, err := rand.Read(randBytes); err != nil {
				// 降级到时间戳方案
				now := time.Now().UnixNano()
				return fmt.Sprintf("%06d", now%1000000)
			}
			n = randBytes[0]
			if n < 250 { // 确保均匀分布 (250 = 25 * 10)
				break
			}
		}
		b[i] = digits[n%10]
	}
	return string(b)
}
