package controllers

import (
	"api/services"
	"bytes"
	"net/http"

	"github.com/dchest/captcha"
	"github.com/gin-gonic/gin"
)

// CaptchaController 图形验证码控制器
type CaptchaController struct {
	service *services.CaptchaService
}

// NewCaptchaController 创建验证码控制器
func NewCaptchaController(service *services.CaptchaService) *CaptchaController {
	return &CaptchaController{
		service: service,
	}
}

// RegisterRoutes 注册路由
func (cc *CaptchaController) RegisterRoutes(router *gin.Engine) {
	captcha := router.Group("/api/captcha")
	{
		captcha.GET("/generate", cc.GenerateCaptcha)
		captcha.GET("/image/:id", cc.GetCaptchaImage)
	}
}

// GenerateCaptcha 生成验证码
// @Summary 生成图形验证码
// @Description 生成一个新的图形验证码ID
// @Tags 验证码
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "验证码ID"
// @Failure 500 {object} map[string]interface{} "系统错误"
// @Router /api/captcha/generate [get]
func (cc *CaptchaController) GenerateCaptcha(c *gin.Context) {
	id, err := cc.service.GenerateCaptcha()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成验证码失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "验证码生成成功",
		"data": gin.H{
			"captcha_id": id,
		},
	})
}

// GetCaptchaImage 获取验证码图片
// @Summary 获取验证码图片
// @Description 根据验证码ID获取图片
// @Tags 验证码
// @Accept json
// @Produce image/png
// @Param id path string true "验证码ID"
// @Success 200 {file} binary "验证码图片"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Router /api/captcha/image/{id} [get]
func (cc *CaptchaController) GetCaptchaImage(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "验证码ID不能为空",
		})
		return
	}

	// 生成验证码图片
	var buf bytes.Buffer
	// Increase resolution for high DPI screens (2x scale)
	err := captcha.WriteImage(&buf, id, 480, 160)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成验证码图片失败",
		})
		return
	}

	// 返回图片
	c.Data(http.StatusOK, "image/png", buf.Bytes())
}
