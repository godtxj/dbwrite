package controllers

import (
	"api/middleware"
	"api/models"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// VerificationService 验证码服务接口
type VerificationService interface {
	SendVerificationCode(email string) (string, error)
	VerifyCode(email, code string) bool
	DeleteCode(email string)
}

// CaptchaService 图形验证码服务接口
type CaptchaService interface {
	VerifyCaptcha(id, value string) bool
}

// UserController 用户控制器
type UserController struct {
	db           UserDB
	jwt          *middleware.JWTMiddleware
	verification VerificationService
	captcha      CaptchaService
}

// UserDB 用户数据库接口
type UserDB interface {
	// 根据邮箱获取用户
	GetUserByEmail(email string) (*models.User, error)
	// 根据ID获取用户
	GetUserByID(id int64) (*models.User, error)
	// 创建用户
	CreateUser(user *models.User) error
	// 更新用户
	UpdateUser(user *models.User) error
	// 检查邮箱是否存在
	EmailExists(email string) (bool, error)
	// 检查邀请码是否存在
	InviteCodeExists(code string) (bool, error)
}

// NewUserController 创建用户控制器
func NewUserController(db UserDB, jwtMiddleware *middleware.JWTMiddleware, verification VerificationService, captcha CaptchaService) *UserController {
	return &UserController{
		db:           db,
		jwt:          jwtMiddleware,
		verification: verification,
		captcha:      captcha,
	}
}

// RegisterRoutes 注册路由
func (uc *UserController) RegisterRoutes(router *gin.Engine) {
	// 公开路由（不需要登录）
	public := router.Group("/api/user")
	{
		public.POST("/send-code", uc.SendVerificationCode)
		public.POST("/register", uc.Register)
		public.POST("/login", uc.Login)
	}

	// 需要登录的路由
	authorized := router.Group("/api/user")
	authorized.Use(uc.jwt.JWTAuth())
	{
		authorized.GET("/info", uc.GetUserInfo)
		authorized.PUT("/password", uc.ChangePassword)
		authorized.POST("/membership", uc.ActivateMembership)
	}
}

// RegisterRoutesWithRateLimit 注册路由（包含限流）
func (uc *UserController) RegisterRoutesWithRateLimit(router *gin.Engine, rateLimiter interface{}) {
	// 类型断言获取限流中间件
	type RateLimiter interface {
		LoginLimit() gin.HandlerFunc
		RegisterLimit() gin.HandlerFunc
		SendCodeLimit() gin.HandlerFunc
	}
	
	rl, ok := rateLimiter.(RateLimiter)
	if !ok {
		// 如果类型断言失败，使用普通注册
		uc.RegisterRoutes(router)
		return
	}
	
	// 公开路由（不需要登录，但需要限流）
	public := router.Group("/api/user")
	{
		public.POST("/send-code", rl.SendCodeLimit(), uc.SendVerificationCode)
		public.POST("/register", rl.RegisterLimit(), uc.Register)
		public.POST("/login", rl.LoginLimit(), uc.Login)
	}

	// 需要登录的路由
	authorized := router.Group("/api/user")
	authorized.Use(uc.jwt.JWTAuth())
	{
		authorized.GET("/info", uc.GetUserInfo)
		authorized.PUT("/password", uc.ChangePassword)
		authorized.POST("/membership", uc.ActivateMembership)
	}
}

// ==================== 验证码 ====================

// SendVerificationCodeRequest 发送验证码请求
type SendVerificationCodeRequest struct {
	Email      string `json:"email" binding:"required,email"`
	CaptchaID  string `json:"captcha_id" binding:"required"`
	CaptchaValue string `json:"captcha_value" binding:"required"`
}

// SendVerificationCode 发送验证码
// @Summary 发送邮箱验证码
// @Description 发送验证码到指定邮箱，需要图形验证码
// @Tags 用户
// @Accept json
// @Produce json
// @Param request body SendVerificationCodeRequest true "发送验证码请求"
// @Success 200 {object} map[string]interface{}
// @Router /api/user/send-code [post]
func (uc *UserController) SendVerificationCode(c *gin.Context) {
	var req SendVerificationCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 验证图形验证码
	if !uc.captcha.VerifyCaptcha(req.CaptchaID, req.CaptchaValue) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "图形验证码错误或已过期",
		})
		return
	}

	// 检查邮箱是否已注册
	exists, err := uc.db.EmailExists(req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "系统错误",
			"error":   err.Error(),
		})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "该邮箱已被注册",
		})
		return
	}

	// 发送验证码
	code, err := uc.verification.SendVerificationCode(req.Email)
	if err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":    429,
			"message": err.Error(),
		})
		return
	}

	// 开发环境返回验证码，生产环境不返回
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "验证码已发送到您的邮箱",
		"data": gin.H{
			"code": code, // TODO: 生产环境删除此行
		},
	})
}

// ==================== 注册 ====================

// RegisterRequest 注册请求
type RegisterRequest struct {
	Email            string `json:"email" binding:"required,email"`
	Nickname         string `json:"nickname" binding:"required,min=2,max=50"`
	Password         string `json:"password" binding:"required,min=6,max=32"`
	VerificationCode string `json:"verification_code" binding:"required,len=6"`
	CaptchaID        string `json:"captcha_id" binding:"required"`
	CaptchaValue     string `json:"captcha_value" binding:"required"`
}

// Register 用户注册
// @Summary 用户注册
// @Description 用户注册接口，不需要登录
// @Tags 用户
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "注册信息"
// @Success 200 {object} map[string]interface{}
// @Router /api/user/register [post]
func (uc *UserController) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 验证图形验证码
	if !uc.captcha.VerifyCaptcha(req.CaptchaID, req.CaptchaValue) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "图形验证码错误或已过期",
		})
		return
	}

	// 验证邮箱验证码
	if !uc.verification.VerifyCode(req.Email, req.VerificationCode) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "验证码错误或已过期",
		})
		return
	}

	// 检查邮箱是否已存在
	exists, err := uc.db.EmailExists(req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "系统错误",
			"error":   err.Error(),
		})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "该邮箱已被注册",
		})
		return
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "密码加密失败",
		})
		return
	}

	// 生成唯一邀请码
	inviteCode, err := uc.generateUniqueInviteCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成邀请码失败",
		})
		return
	}

	// 创建用户
	now := time.Now()
	defaultLevel := 0
	user := &models.User{
		Email:       req.Email,
		Nickname:    req.Nickname,
		Password:    string(hashedPassword),
		MemberLevel: &defaultLevel,
		InviteCode:  inviteCode,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := uc.db.CreateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "注册失败",
			"error":   err.Error(),
		})
		return
	}

	// 注册成功后删除验证码
	uc.verification.DeleteCode(req.Email)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "注册成功",
		"data": gin.H{
			"email":       user.Email,
			"nickname":    user.Nickname,
			"invite_code": user.InviteCode,
		},
	})
}

// ==================== 登录 ====================

// LoginRequest 登录请求
type LoginRequest struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required"`
	CaptchaID    string `json:"captcha_id" binding:"required"`
	CaptchaValue string `json:"captcha_value" binding:"required"`
}

// Login 用户登录
// @Summary 用户登录
// @Description 用户登录接口，不需要登录
// @Tags 用户
// @Accept json
// @Produce json
// @Param request body LoginRequest true "登录信息"
// @Success 200 {object} map[string]interface{}
// @Router /api/user/login [post]
func (uc *UserController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 验证图形验证码
	if !uc.captcha.VerifyCaptcha(req.CaptchaID, req.CaptchaValue) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "图形验证码错误或已过期",
		})
		return
	}

	// 根据邮箱获取用户
	user, err := uc.db.GetUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "邮箱或密码错误",
		})
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "邮箱或密码错误",
		})
		return
	}

	// 获取会员等级（处理可空字段）
	memberLevel := 0
	if user.MemberLevel != nil {
		memberLevel = *user.MemberLevel
	}

	// 生成JWT token
	token, err := uc.jwt.GenerateToken(
		user.ID,
		user.Email,
		user.Nickname,
		memberLevel,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成token失败",
		})
		return
	}

	// 返回用户信息和token
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "登录成功",
		"data": gin.H{
			"token": token,
			"user": gin.H{
				"id":               user.ID,
				"email":            user.Email,
				"nickname":         user.Nickname,
				"member_level":     memberLevel,
				"member_expire_at": user.MemberExpireAt,
				"invite_code":      user.InviteCode,
				"created_at":       user.CreatedAt,
				"updated_at":       user.UpdatedAt,
			},
		},
	})
}

// ==================== 获取用户信息 ====================

// GetUserInfo 获取用户信息（需要登录）
// @Summary 获取用户信息
// @Description 获取当前登录用户的信息，需要JWT认证
// @Tags 用户
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/user/info [get]
func (uc *UserController) GetUserInfo(c *gin.Context) {
	// 从JWT中获取用户ID
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}

	// 从数据库获取最新的用户信息
	user, err := uc.db.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "用户不存在",
		})
		return
	}

	// 处理可空字段
	memberLevel := 0
	if user.MemberLevel != nil {
		memberLevel = *user.MemberLevel
	}

	// 检查会员是否过期
	isMemberActive := false
	if user.MemberExpireAt != nil && *user.MemberExpireAt > time.Now().Unix() {
		isMemberActive = true
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"id":                user.ID,
			"email":             user.Email,
			"nickname":          user.Nickname,
			"member_level":      memberLevel,
			"member_expire_at":  user.MemberExpireAt,
			"is_member_active":  isMemberActive,
			"invite_code":       user.InviteCode,
			"created_at":        user.CreatedAt,
			"updated_at":        user.UpdatedAt,
		},
	})
}

// ==================== 修改密码 ====================

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=32"`
}

// ChangePassword 修改密码（需要登录）
// @Summary 修改密码
// @Description 修改当前登录用户的密码，需要JWT认证
// @Tags 用户
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body ChangePasswordRequest true "密码信息"
// @Success 200 {object} map[string]interface{}
// @Router /api/user/password [put]
func (uc *UserController) ChangePassword(c *gin.Context) {
	// 从JWT中获取用户ID
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 获取用户信息
	user, err := uc.db.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "用户不存在",
		})
		return
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "原密码错误",
		})
		return
	}

	// 检查新密码是否与旧密码相同
	if req.OldPassword == req.NewPassword {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "新密码不能与原密码相同",
		})
		return
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "密码加密失败",
		})
		return
	}

	// 更新密码
	user.Password = string(hashedPassword)
	user.UpdatedAt = time.Now()

	if err := uc.db.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "修改密码失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "密码修改成功，请重新登录",
	})
}

// ==================== 开通会员 ====================

// ActivateMembershipRequest 开通会员请求
type ActivateMembershipRequest struct {
	Level   int `json:"level" binding:"required,min=1,max=10"`   // 会员等级 1-10
	Months  int `json:"months" binding:"required,min=1,max=120"` // 开通月数 1-120
	PayType int `json:"pay_type" binding:"required"`             // 支付方式（预留字段）
}

// ActivateMembership 开通会员（需要登录）
// @Summary 开通会员
// @Description 为当前登录用户开通会员，需要JWT认证
// @Tags 用户
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body ActivateMembershipRequest true "会员信息"
// @Success 200 {object} map[string]interface{}
// @Router /api/user/membership [post]
func (uc *UserController) ActivateMembership(c *gin.Context) {
	// 从JWT中获取用户ID
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}

	var req ActivateMembershipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 获取用户信息
	user, err := uc.db.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "用户不存在",
		})
		return
	}

	// TODO: 这里应该先进行支付验证
	// 实际项目中需要：
	// 1. 创建订单
	// 2. 调用支付接口
	// 3. 支付成功后再开通会员
	// 这里为了演示，直接开通

	// 计算会员到期时间
	var expireAt int64
	now := time.Now()

	// 如果已有会员且未过期，在原有基础上延长
	if user.MemberExpireAt != nil && *user.MemberExpireAt > now.Unix() {
		expireTime := time.Unix(*user.MemberExpireAt, 0)
		expireAt = expireTime.AddDate(0, req.Months, 0).Unix()
	} else {
		// 否则从现在开始计算
		expireAt = now.AddDate(0, req.Months, 0).Unix()
	}

	// 更新用户会员信息
	user.MemberLevel = &req.Level
	user.MemberExpireAt = &expireAt
	user.UpdatedAt = now

	if err := uc.db.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "开通会员失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "会员开通成功",
		"data": gin.H{
			"member_level":     req.Level,
			"member_expire_at": expireAt,
			"expire_time":      time.Unix(expireAt, 0).Format("2006-01-02 15:04:05"),
		},
	})
}

// ==================== 辅助函数 ====================

// generateUniqueInviteCode 生成唯一邀请码
func (uc *UserController) generateUniqueInviteCode() (string, error) {
	maxAttempts := 10
	for i := 0; i < maxAttempts; i++ {
		// 生成8位随机邀请码
		code, err := generateRandomCode(8)
		if err != nil {
			return "", err
		}

		// 检查是否已存在
		exists, err := uc.db.InviteCodeExists(code)
		if err != nil {
			return "", err
		}

		if !exists {
			return code, nil
		}
	}

	return "", fmt.Errorf("生成邀请码失败,请重试")
}

// generateRandomCode 生成随机码
func generateRandomCode(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

