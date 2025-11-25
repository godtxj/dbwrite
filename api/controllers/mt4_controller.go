package controllers

import (
	"api/middleware"
	"api/models"
	"api/services"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// MT4Controller MT4控制器
type MT4Controller struct {
	service   MT4Service
	jwt       *middleware.JWTMiddleware
	earuntime *services.EARuntimeService // EA运行时服务
}

// MT4Service MT4服务接口
type MT4Service interface {
	// 平台管理
	GetPlatforms() ([]*models.Platform, error)
	GetTopLevelPlatforms() ([]*models.Platform, error)
	GetSubPlatforms(parentID int64) ([]*models.Platform, error)
	GetPlatformByID(id int64) (*models.Platform, error)

	// MT4账户管理（支持分页）
	GetMT4AccountsByUserID(userID int64, limit, offset int) ([]*models.MT4Account, error)
	CountMT4AccountsByUserID(userID int64) (int, error)
	GetMT4AccountByID(id int64) (*models.MT4Account, error)
	CreateMT4Account(account *models.MT4Account) error
	UpdateMT4Account(account *models.MT4Account) error
	DeleteMT4Account(id int64) error
	CheckMT4AccountOwner(accountID, userID int64) (bool, error)

	// EA管理（支持分页）
	GetEAs(limit, offset int) ([]*models.EA, error)
	CountEAs() (int, error)
	GetEAByID(id int64) (*models.EA, error)
	GetEAParams(eaID int64) ([]*models.EAParam, error)

	// 订单管理（支持分页）
	GetUserOrders(userID int64, limit, offset int) ([]*models.Order, error)
	CountUserOrders(userID int64) (int, error)
	GetOrderByID(id int64) (*models.Order, error)
	CreateOrder(order *models.Order) error
	UpdateOrderStatus(id int64, status int) error
	DeleteOrder(id int64) error
	CheckOrderOwner(orderID, userID int64) (bool, error)

	// 订单列表（支持分页）
	GetOrderList(orderID int64, limit, offset int) ([]*models.OrderList, error)
	CountOrderList(orderID int64) (int, error)

	// 货币对（支持分页）
	GetSymbols(limit, offset int) ([]*models.Symbol, error)
	CountSymbols() (int, error)

	// 用户信息
	GetUserByID(userID int64) (*models.User, error)
}

// NewMT4Controller 创建MT4控制器
func NewMT4Controller(service MT4Service, jwtMiddleware *middleware.JWTMiddleware, earuntime *services.EARuntimeService) *MT4Controller {
	return &MT4Controller{
		service:   service,
		jwt:       jwtMiddleware,
		earuntime: earuntime,
	}
}

// RegisterRoutes 注册路由
func (mc *MT4Controller) RegisterRoutes(router *gin.Engine) {
	// 所有MT4路由都需要登录
	authorized := router.Group("/api/mt4")
	authorized.Use(mc.jwt.JWTAuth())
	{
		// 平台和基础数据查询
		authorized.GET("/platforms", mc.GetPlatforms)
		authorized.GET("/platforms/:id/children", mc.GetSubPlatforms)
		authorized.GET("/eas", mc.GetEAs)
		authorized.GET("/eas/:id", mc.GetEADetail)
		authorized.GET("/symbols", mc.GetSymbols)

		// MT4账户管理
		authorized.GET("/accounts", mc.GetMT4Accounts)
		authorized.POST("/accounts", mc.CreateMT4Account)
		authorized.PUT("/accounts/:id", mc.UpdateMT4Account)
		authorized.DELETE("/accounts/:id", mc.DeleteMT4Account)

		// 订单管理（EA操作）
		authorized.GET("/orders", mc.GetOrders)
		authorized.POST("/orders", mc.StartEA)           // 启动EA
		authorized.PUT("/orders/:id/pause", mc.PauseEA)  // 暂停EA
		authorized.PUT("/orders/:id/resume", mc.ResumeEA) // 恢复EA
		authorized.DELETE("/orders/:id", mc.DeleteEA)    // 删除EA

		// 订单列表
		authorized.GET("/orders/:id/list", mc.GetOrderList)
	}
}

// getPagination 从请求中获取分页参数
func getPagination(c *gin.Context) (limit, offset int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	limit = pageSize
	offset = (page - 1) * pageSize
	return
}

// buildPaginationResponse 构建分页响应
func buildPaginationResponse(data interface{}, total, page, pageSize int) gin.H {
	totalPages := (total + pageSize - 1) / pageSize
	return gin.H{
		"code": 200,
		"data": data,
		"pagination": gin.H{
			"total":       total,
			"page":        page,
			"page_size":   pageSize,
			"total_pages": totalPages,
		},
	}
}

// ==================== 平台管理 ====================

// GetPlatforms 获取平台列表（支持层级结构）
// @Summary 获取MT4平台列表
// @Description 获取MT4平台列表，支持层级结构和仅顶级平台
// @Tags MT4
// @Security ApiKeyAuth
// @Param top_only query boolean false "只获取顶级平台"
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/platforms [get]
func (mc *MT4Controller) GetPlatforms(c *gin.Context) {
	topOnly := c.Query("top_only") == "true"

	var platforms []*models.Platform
	var err error

	if topOnly {
		platforms, err = mc.service.GetTopLevelPlatforms()
	} else {
		platforms, err = mc.service.GetPlatforms()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取平台列表失败",
			"error":   err.Error(),
		})
		return
	}

	if !topOnly {
		platformTree := mc.buildPlatformTree(platforms)
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"data": platformTree,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": platforms,
	})
}

// GetSubPlatforms 获取下级平台
// @Summary 获取下级平台
// @Description 根据父平台ID获取下级平台列表
// @Tags MT4
// @Security ApiKeyAuth
// @Param id path int true "父平台ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/platforms/{id}/children [get]
func (mc *MT4Controller) GetSubPlatforms(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的平台ID",
		})
		return
	}

	platforms, err := mc.service.GetSubPlatforms(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取下级平台失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": platforms,
	})
}

// buildPlatformTree 构建平台树形结构
func (mc *MT4Controller) buildPlatformTree(platforms []*models.Platform) []map[string]interface{} {
	platformMap := make(map[int64]*models.Platform)
	for _, p := range platforms {
		platformMap[p.ID] = p
	}

	var tree []map[string]interface{}
	for _, p := range platforms {
		parentID := int64(0)
		if p.ParentID != nil {
			parentID = *p.ParentID
		}

		if parentID == 0 {
			node := mc.platformToNode(p, platformMap)
			tree = append(tree, node)
		}
	}

	return tree
}

func (mc *MT4Controller) platformToNode(p *models.Platform, platformMap map[int64]*models.Platform) map[string]interface{} {
	node := map[string]interface{}{
		"id":         p.ID,
		"parent_id":  p.ParentID,
		"title":      p.Title,
		"status":     p.Status,
		"server":     p.Server,
		"remark":     p.Remark,
		"created_at": p.CreatedAt,
		"updated_at": p.UpdatedAt,
		"children":   []map[string]interface{}{},
	}

	for _, child := range platformMap {
		if child.ParentID != nil && *child.ParentID == p.ID {
			childNode := mc.platformToNode(child, platformMap)
			node["children"] = append(node["children"].([]map[string]interface{}), childNode)
		}
	}

	return node
}

// ==================== MT4账户管理 ====================

// GetMT4Accounts 获取用户的MT4账户列表（支持分页）
// @Summary 获取MT4账户列表
// @Description 获取当前用户的MT4账户列表，支持分页
// @Tags MT4
// @Security ApiKeyAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/accounts [get]
func (mc *MT4Controller) GetMT4Accounts(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	limit, offset := getPagination(c)

	accounts, err := mc.service.GetMT4AccountsByUserID(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取MT4账户列表失败",
			"error":   err.Error(),
		})
		return
	}

	total, err := mc.service.CountMT4AccountsByUserID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取账户总数失败",
			"error":   err.Error(),
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	c.JSON(http.StatusOK, buildPaginationResponse(accounts, total, page, pageSize))
}

// CreateMT4AccountRequest 创建MT4账户请求
type CreateMT4AccountRequest struct {
	PlatformID int64   `json:"platform_id" binding:"required"`
	Account    string  `json:"account" binding:"required"`
	Password   string  `json:"password" binding:"required"`
	Type       *int    `json:"type"`
	Remark     *string `json:"remark"`
}

// CreateMT4Account 创建MT4账户
// @Summary 创建MT4账户
// @Description 为当前用户创建新的MT4账户
// @Tags MT4
// @Security ApiKeyAuth
// @Param request body CreateMT4AccountRequest true "MT4账户信息"
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/accounts [post]
func (mc *MT4Controller) CreateMT4Account(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)

	var req CreateMT4AccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	_, err := mc.service.GetPlatformByID(req.PlatformID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "平台不存在",
		})
		return
	}

	now := time.Now()
	defaultAmount := 0.00
	defaultProfit := 0.00

	account := &models.MT4Account{
		UserID:     userID,
		PlatformID: req.PlatformID,
		Account:    req.Account,
		Password:   req.Password,
		Type:       req.Type,
		Amount:     &defaultAmount,
		Profit:     &defaultProfit,
		Status:     0,
		Remark:     req.Remark,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := mc.service.CreateMT4Account(account); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建MT4账户失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "创建成功",
		"data": gin.H{
			"id": account.ID,
		},
	})
}

// UpdateMT4AccountRequest 更新MT4账户请求
type UpdateMT4AccountRequest struct {
	PlatformID *int64  `json:"platform_id"`
	Account    *string `json:"account"`
	Password   *string `json:"password"`
	Type       *int    `json:"type"`
	Remark     *string `json:"remark"`
}

// UpdateMT4Account 更新MT4账户
// @Summary 更新MT4账户
// @Description 更新指定的MT4账户信息
// @Tags MT4
// @Security ApiKeyAuth
// @Param id path int true "账户ID"
// @Param request body UpdateMT4AccountRequest true "更新信息"
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/accounts/{id} [put]
func (mc *MT4Controller) UpdateMT4Account(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的账户ID",
		})
		return
	}

	isOwner, err := mc.service.CheckMT4AccountOwner(id, userID)
	if err != nil || !isOwner {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "无权操作此账户",
		})
		return
	}

	var req UpdateMT4AccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	account, err := mc.service.GetMT4AccountByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "账户不存在",
		})
		return
	}

	if req.PlatformID != nil {
		account.PlatformID = *req.PlatformID
	}
	if req.Account != nil {
		account.Account = *req.Account
	}
	if req.Password != nil {
		account.Password = *req.Password
	}
	if req.Type != nil {
		account.Type = req.Type
	}
	if req.Remark != nil {
		account.Remark = req.Remark
	}
	account.UpdatedAt = time.Now()

	if err := mc.service.UpdateMT4Account(account); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "更新成功",
	})
}

// DeleteMT4Account 删除MT4账户（软删除）
// @Summary 删除MT4账户
// @Description 删除指定的MT4账户（软删除）
// @Tags MT4
// @Security ApiKeyAuth
// @Param id path int true "账户ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/accounts/{id} [delete]
func (mc *MT4Controller) DeleteMT4Account(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的账户ID",
		})
		return
	}

	isOwner, err := mc.service.CheckMT4AccountOwner(id, userID)
	if err != nil || !isOwner {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "无权操作此账户",
		})
		return
	}

	if err := mc.service.DeleteMT4Account(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "删除成功",
	})
}

// ==================== EA管理 ====================

// GetEAs 获取EA列表（支持分页）
// @Summary 获取EA列表
// @Description 获取可用的EA列表，支持分页
// @Tags MT4
// @Security ApiKeyAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/eas [get]
func (mc *MT4Controller) GetEAs(c *gin.Context) {
	limit, offset := getPagination(c)

	eas, err := mc.service.GetEAs(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取EA列表失败",
			"error":   err.Error(),
		})
		return
	}

	total, err := mc.service.CountEAs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取EA总数失败",
			"error":   err.Error(),
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	c.JSON(http.StatusOK, buildPaginationResponse(eas, total, page, pageSize))
}

// GetEADetail 获取EA详情（包含参数）
// @Summary 获取EA详情
// @Description 获取指定EA的详细信息，包括参数配置
// @Tags MT4
// @Security ApiKeyAuth
// @Param id path int true "EA ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/eas/{id} [get]
func (mc *MT4Controller) GetEADetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的EA ID",
		})
		return
	}

	ea, err := mc.service.GetEAByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "EA不存在",
		})
		return
	}

	params, err := mc.service.GetEAParams(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取EA参数失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"ea":     ea,
			"params": params,
		},
	})
}

// ==================== 订单管理（EA操作） ====================

// GetOrders 获取用户的订单列表（支持分页）
// @Summary 获取订单列表
// @Description 获取当前用户的EA订单列表，支持分页
// @Tags MT4
// @Security ApiKeyAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/orders [get]
func (mc *MT4Controller) GetOrders(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	limit, offset := getPagination(c)

	orders, err := mc.service.GetUserOrders(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取订单列表失败",
			"error":   err.Error(),
		})
		return
	}

	total, err := mc.service.CountUserOrders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取订单总数失败",
			"error":   err.Error(),
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	c.JSON(http.StatusOK, buildPaginationResponse(orders, total, page, pageSize))
}

// StartEARequest 启动EA请求
type StartEARequest struct {
	EAID         int64  `json:"ea_id" binding:"required"`
	MT4AccountID int64  `json:"mt4_account_id" binding:"required"`
	Symbol       string `json:"symbol" binding:"required"`
	Params       string `json:"params"`
}

// StartEA 启动EA
// @Summary 启动EA
// @Description 在指定MT4账户上启动EA交易
// @Tags MT4
// @Security ApiKeyAuth
// @Param request body StartEARequest true "启动EA请求"
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/orders [post]
func (mc *MT4Controller) StartEA(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)

	var req StartEARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	_, err := mc.service.GetEAByID(req.EAID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "EA不存在",
		})
		return
	}

	isOwner, err := mc.service.CheckMT4AccountOwner(req.MT4AccountID, userID)
	if err != nil || !isOwner {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "无权使用此MT4账户",
		})
		return
	}

	// 获取MT4账户信息并验证
	mt4Account, err := mc.service.GetMT4AccountByID(req.MT4AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "MT4账户不存在",
		})
		return
	}

	// 检查账户状态（0=正常）
	if mt4Account.Status != 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "MT4账户状态异常，无法启动EA",
		})
		return
	}

	// 检查账户余额
	balance := 0.0
	if mt4Account.Amount != nil {
		balance = *mt4Account.Amount
	}

	if balance < 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": fmt.Sprintf("MT4账户余额不足，当前余额: %.2f, 最低要求: 100.00", balance),
		})
		return
	}

	log.Printf("MT4账户验证通过: AccountID=%d, Balance=%.2f", req.MT4AccountID, balance)

	now := time.Now()
	order := &models.Order{
		UserID:       userID,
		EAID:         req.EAID,
		MT4AccountID: req.MT4AccountID,
		Symbol:       req.Symbol,
		Status:       0,
		Params:       &req.Params,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := mc.service.CreateOrder(order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "启动EA失败",
			"error":   err.Error(),
		})
		return
	}

	// 获取EA信息
	ea, err := mc.service.GetEAByID(req.EAID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取EA信息失败",
		})
		return
	}

	// 获取用户信息
	user, err := mc.service.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取用户信息失败",
		})
		return
	}

	// 解析用户自定义参数
	var eaParams map[string]interface{}
	if req.Params != "" {
		if err := json.Unmarshal([]byte(req.Params), &eaParams); err != nil {
			log.Printf("WARN: Failed to parse EA params: %v", err)
			eaParams = make(map[string]interface{})
		}
	} else {
		eaParams = make(map[string]interface{})
	}

	// 从参数中提取配置，如果没有则使用默认值
	timeframe := "M5"
	if tf, ok := eaParams["timeframe"].(string); ok {
		timeframe = tf
	}

	riskPercent := 1.0
	if rp, ok := eaParams["risk_percent"].(float64); ok {
		riskPercent = rp
	}

	maxPositions := 3
	if mp, ok := eaParams["max_positions"].(float64); ok {
		maxPositions = int(mp)
	}

	// 启动EA运行时
	eaConfig := services.EAConfig{
		EAID:         fmt.Sprintf("%d", order.ID),
		UserID:       fmt.Sprintf("%d", userID),
		EAName:       ea.Name,
		Symbol:       req.Symbol,
		Timeframe:    timeframe,
		Strategy:     "trend_following",
		RiskPercent:  riskPercent,
		MaxPositions: maxPositions,
		Enabled:      true,
		MT4AccountID: req.MT4AccountID, // 添加MT4账户ID
		Params:       eaParams,         // 传递原始参数，让EA策略自己解析
	}

	userInfo := services.UserInfo{
		UserID:   userID,
		Username: user.Nickname,
		Balance:  balance, // 使用MT4账户的真实余额
	}

	if err := mc.earuntime.StartEA(order.ID, eaConfig, userInfo); err != nil {
		log.Printf("ERROR: Failed to start EA runtime: %v", err)
		// 不返回错误，因为订单已创建
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "EA启动成功",
		"data": gin.H{
			"order_id": order.ID,
		},
	})
}

// PauseEA 暂停EA
// @Summary 暂停EA
// @Description 暂停指定的EA交易订单
// @Tags MT4
// @Security ApiKeyAuth
// @Param id path int true "订单ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/orders/{id}/pause [put]
func (mc *MT4Controller) PauseEA(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的订单ID",
		})
		return
	}

	isOwner, err := mc.service.CheckOrderOwner(id, userID)
	if err != nil || !isOwner {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "无权操作此订单",
		})
		return
	}

	if err := mc.service.UpdateOrderStatus(id, 1); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "暂停EA失败",
			"error":   err.Error(),
		})
		return
	}

	// 暂停EA运行时
	if err := mc.earuntime.PauseEA(id); err != nil {
		log.Printf("WARN: Failed to pause EA runtime: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "EA已暂停",
	})
}

// ResumeEA 恢复EA
// @Summary 恢复EA
// @Description 恢复已暂停的EA交易订单
// @Tags MT4
// @Security ApiKeyAuth
// @Param id path int true "订单ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/orders/{id}/resume [put]
func (mc *MT4Controller) ResumeEA(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的订单ID",
		})
		return
	}

	isOwner, err := mc.service.CheckOrderOwner(id, userID)
	if err != nil || !isOwner {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "无权操作此订单",
		})
		return
	}

	if err := mc.service.UpdateOrderStatus(id, 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "恢复EA失败",
			"error":   err.Error(),
		})
		return
	}

	// 恢复EA运行时
	if err := mc.earuntime.ResumeEA(id); err != nil {
		log.Printf("WARN: Failed to resume EA runtime: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "EA已恢复",
	})
}

// DeleteEA 删除EA（软删除订单）
// @Summary 删除EA订单
// @Description 删除指定的EA交易订单（软删除）
// @Tags MT4
// @Security ApiKeyAuth
// @Param id path int true "订单ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/orders/{id} [delete]
func (mc *MT4Controller) DeleteEA(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的订单ID",
		})
		return
	}

	isOwner, err := mc.service.CheckOrderOwner(id, userID)
	if err != nil || !isOwner {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "无权操作此订单",
		})
		return
	}

	if err := mc.service.DeleteOrder(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除EA失败",
			"error":   err.Error(),
		})
		return
	}

	// 停止EA运行时
	if err := mc.earuntime.StopEA(id); err != nil {
		log.Printf("WARN: Failed to stop EA runtime: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "EA已删除",
	})
}

// ==================== 订单列表 ====================

// GetOrderList 获取订单的交易列表（支持分页）
// @Summary 获取订单交易列表
// @Description 获取指定订单的交易明细列表，支持分页
// @Tags MT4
// @Security ApiKeyAuth
// @Param id path int true "订单ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/orders/{id}/list [get]
func (mc *MT4Controller) GetOrderList(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的订单ID",
		})
		return
	}

	isOwner, err := mc.service.CheckOrderOwner(id, userID)
	if err != nil || !isOwner {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "无权查看此订单",
		})
		return
	}

	limit, offset := getPagination(c)
	orderList, err := mc.service.GetOrderList(id, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取订单列表失败",
			"error":   err.Error(),
		})
		return
	}

	total, err := mc.service.CountOrderList(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取订单列表总数失败",
			"error":   err.Error(),
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	c.JSON(http.StatusOK, buildPaginationResponse(orderList, total, page, pageSize))
}

// ==================== 货币对 ====================

// GetSymbols 获取货币对列表（支持分页）
// @Summary 获取货币对列表
// @Description 获取可用的货币对列表，支持分页
// @Tags MT4
// @Security ApiKeyAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/symbols [get]
func (mc *MT4Controller) GetSymbols(c *gin.Context) {
	limit, offset := getPagination(c)

	symbols, err := mc.service.GetSymbols(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取货币对列表失败",
			"error":   err.Error(),
		})
		return
	}

	total, err := mc.service.CountSymbols()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取货币对总数失败",
			"error":   err.Error(),
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	c.JSON(http.StatusOK, buildPaginationResponse(symbols, total, page, pageSize))
}
