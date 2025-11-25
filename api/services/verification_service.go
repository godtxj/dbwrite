package services

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"
)

// VerificationCode 验证码结构
type VerificationCode struct {
	Code      string
	ExpiresAt time.Time
}

// VerificationService 验证码服务
type VerificationService struct {
	codes map[string]*VerificationCode // email -> code
	mu    sync.RWMutex
}

// NewVerificationService 创建验证码服务
func NewVerificationService() *VerificationService {
	vs := &VerificationService{
		codes: make(map[string]*VerificationCode),
	}
	
	// 启动清理过期验证码的goroutine
	go vs.cleanupExpiredCodes()
	
	return vs
}

// GenerateCode 生成6位数字验证码
func (vs *VerificationService) GenerateCode() string {
	const digits = "0123456789"
	code := make([]byte, 6)
	
	for i := range code {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		code[i] = digits[num.Int64()]
	}
	
	return string(code)
}

// SendVerificationCode 发送验证码（存储到内存）
func (vs *VerificationService) SendVerificationCode(email string) (string, error) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	
	// 检查是否在60秒内已发送过验证码
	if existing, ok := vs.codes[email]; ok {
		if time.Now().Before(existing.ExpiresAt.Add(-4 * time.Minute)) {
			remainingTime := 60 - int(time.Since(existing.ExpiresAt.Add(-5*time.Minute)).Seconds())
			log.Printf("WARN: Verification code already sent to %s, please wait %d seconds", email, remainingTime)
			return "", fmt.Errorf("验证码已发送，请等待%d秒后重试", remainingTime)
		}
	}
	
	// 生成新验证码
	code := vs.GenerateCode()
	
	// 存储验证码，有效期5分钟
	vs.codes[email] = &VerificationCode{
		Code:      code,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	
	log.Printf("INFO: Verification code generated for %s: %s (expires in 5 minutes)", email, code)
	
	// TODO: 这里应该调用邮件服务发送验证码
	// 目前只是记录日志，实际生产环境需要集成邮件服务
	// 例如：使用 SMTP、SendGrid、阿里云邮件服务等
	
	return code, nil
}

// VerifyCode 验证验证码
func (vs *VerificationService) VerifyCode(email, code string) bool {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	
	stored, ok := vs.codes[email]
	if !ok {
		log.Printf("WARN: No verification code found for email: %s", email)
		return false
	}
	
	// 检查是否过期
	if time.Now().After(stored.ExpiresAt) {
		log.Printf("WARN: Verification code expired for email: %s", email)
		return false
	}
	
	// 验证码匹配
	if stored.Code != code {
		log.Printf("WARN: Invalid verification code for email: %s", email)
		return false
	}
	
	log.Printf("INFO: Verification code validated successfully for email: %s", email)
	return true
}

// DeleteCode 删除验证码（验证成功后删除）
func (vs *VerificationService) DeleteCode(email string) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	
	delete(vs.codes, email)
	log.Printf("INFO: Verification code deleted for email: %s", email)
}

// cleanupExpiredCodes 定期清理过期的验证码
func (vs *VerificationService) cleanupExpiredCodes() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		vs.mu.Lock()
		now := time.Now()
		count := 0
		
		for email, code := range vs.codes {
			if now.After(code.ExpiresAt) {
				delete(vs.codes, email)
				count++
			}
		}
		
		if count > 0 {
			log.Printf("INFO: Cleaned up %d expired verification codes", count)
		}
		vs.mu.Unlock()
	}
}

// GetRemainingTime 获取验证码剩余有效时间（秒）
func (vs *VerificationService) GetRemainingTime(email string) int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	
	stored, ok := vs.codes[email]
	if !ok {
		return 0
	}
	
	remaining := int(time.Until(stored.ExpiresAt).Seconds())
	if remaining < 0 {
		return 0
	}
	
	return remaining
}
