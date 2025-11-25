package services

import (
	"crypto/tls"
	"fmt"
	"log"

	"gopkg.in/gomail.v2"
)

// EmailService 邮件服务
type EmailService struct {
	host     string
	port     int
	user     string
	password string
	from     string
}

// NewEmailService 创建邮件服务
func NewEmailService(host string, port int, user, password, from string) *EmailService {
	return &EmailService{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		from:     from,
	}
}

// SendVerificationCode 发送验证码邮件
func (s *EmailService) SendVerificationCode(to, code string) error {
	subject := "【MT4交易平台】邮箱验证码"
	body := s.buildVerificationEmailHTML(code)
	
	return s.sendEmail(to, subject, body)
}

// sendEmail 发送邮件
func (s *EmailService) sendEmail(to, subject, body string) error {
	m := gomail.NewMessage()
	
	// 设置发件人
	m.SetHeader("From", s.from)
	
	// 设置收件人
	m.SetHeader("To", to)
	
	// 设置主题
	m.SetHeader("Subject", subject)
	
	// 设置邮件正文（HTML格式）
	m.SetBody("text/html", body)
	
	// 创建SMTP拨号器
	d := gomail.NewDialer(s.host, s.port, s.user, s.password)
	
	// QQ邮箱需要TLS
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	
	// 发送邮件
	if err := d.DialAndSend(m); err != nil {
		log.Printf("ERROR: Failed to send email to %s: %v", to, err)
		return fmt.Errorf("发送邮件失败")
	}
	
	log.Printf("INFO: Email sent successfully to %s", to)
	return nil
}

// buildVerificationEmailHTML 构建验证码邮件HTML
func (s *EmailService) buildVerificationEmailHTML(code string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body {
            font-family: Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .container {
            background-color: #f9f9f9;
            border-radius: 10px;
            padding: 30px;
            box-shadow: 0 2px 5px rgba(0,0,0,0.1);
        }
        .header {
            text-align: center;
            color: #4CAF50;
            margin-bottom: 30px;
        }
        .code-box {
            background-color: #fff;
            border: 2px dashed #4CAF50;
            border-radius: 5px;
            padding: 20px;
            text-align: center;
            margin: 20px 0;
        }
        .code {
            font-size: 32px;
            font-weight: bold;
            color: #4CAF50;
            letter-spacing: 5px;
        }
        .info {
            color: #666;
            font-size: 14px;
            margin-top: 20px;
        }
        .footer {
            text-align: center;
            color: #999;
            font-size: 12px;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #ddd;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>MT4交易平台</h1>
            <h2>邮箱验证码</h2>
        </div>
        
        <p>您好！</p>
        <p>您正在进行邮箱验证，您的验证码是：</p>
        
        <div class="code-box">
            <div class="code">%s</div>
        </div>
        
        <div class="info">
            <p>• 验证码有效期为 <strong>5分钟</strong></p>
            <p>• 请勿将验证码告知他人</p>
            <p>• 如非本人操作，请忽略此邮件</p>
        </div>
        
        <div class="footer">
            <p>此邮件由系统自动发送，请勿回复</p>
            <p>© 2024 MT4交易平台 版权所有</p>
        </div>
    </div>
</body>
</html>
`, code)
}
