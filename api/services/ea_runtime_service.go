package services

import (
	"api/ws/indicators"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// EARuntimeService EA运行时服务
type EARuntimeService struct {
	mu           sync.RWMutex
	instances    map[int64]*StrategyEAInstance // 使用策略化的EA实例
	tradeManager *TradeManager
	rdb          *redis.Client
	factory      *EAFactory // EA工厂
}

// NewEARuntimeService 创建EA运行时服务
func NewEARuntimeService(rdb *redis.Client) *EARuntimeService {
	return &EARuntimeService{
		instances:    make(map[int64]*StrategyEAInstance),
		tradeManager: NewTradeManager(),
		rdb:          rdb,
		factory:      NewEAFactory(),
	}
}

// StartEA 启动EA
func (s *EARuntimeService) StartEA(orderID int64, config EAConfig, user UserInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已存在
	if _, exists := s.instances[orderID]; exists {
		return fmt.Errorf("EA已在运行")
	}

	userID := fmt.Sprintf("%d", user.UserID)
	
	// 检查用户是否已注册，避免重复注册
	if !s.tradeManager.UserExists(userID) {
		// 注册用户到交易管理器
		s.tradeManager.RegisterUser(&User{
			UserID:     userID,
			Username:   user.Username,
			Balance:    user.Balance,
			FreeMargin: user.Balance * 0.8,
		})
	}

	// 使用工厂创建对应的EA策略
	strategy, err := s.factory.CreateStrategy(config.EAName, config.Params)
	if err != nil {
		return fmt.Errorf("创建EA策略失败: %v", err)
	}

	// 创建策略化的EA实例
	ea := NewStrategyEAInstance(orderID, config, &User{
		UserID:     userID,
		Username:   user.Username,
		Balance:    user.Balance,
		FreeMargin: user.Balance * 0.8,
	}, strategy, s.rdb, s.tradeManager)

	s.instances[orderID] = ea

	// 启动EA
	ea.Start()

	log.Printf("EA启动成功: OrderID=%d, Strategy=%s, UserID=%d, Symbol=%s", 
		orderID, strategy.GetName(), user.UserID, config.Symbol)
	return nil
}

// PauseEA 暂停EA
func (s *EARuntimeService) PauseEA(orderID int64) error {
	s.mu.RLock()
	ea, exists := s.instances[orderID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("EA不存在")
	}

	ea.Pause()
	log.Printf("EA已暂停: OrderID=%d", orderID)
	return nil
}

// ResumeEA 恢复EA
func (s *EARuntimeService) ResumeEA(orderID int64) error {
	s.mu.RLock()
	ea, exists := s.instances[orderID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("EA不存在")
	}

	ea.Resume()
	log.Printf("EA已恢复: OrderID=%d", orderID)
	return nil
}

// StopEA 停止EA
func (s *EARuntimeService) StopEA(orderID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ea, exists := s.instances[orderID]
	if !exists {
		return fmt.Errorf("EA不存在")
	}

	ea.Stop()
	delete(s.instances, orderID)

	log.Printf("EA已停止: OrderID=%d", orderID)
	return nil
}

// GetEAStatus 获取EA状态
func (s *EARuntimeService) GetEAStatus(orderID int64) (map[string]interface{}, error) {
	s.mu.RLock()
	ea, exists := s.instances[orderID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("EA不存在")
	}

	return ea.GetStatus(), nil
}

// GetAllEAStatus 获取所有EA状态
func (s *EARuntimeService) GetAllEAStatus() []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var statuses []map[string]interface{}
	for orderID, ea := range s.instances {
		status := ea.GetStatus()
		status["order_id"] = orderID
		statuses = append(statuses, status)
	}
	return statuses
}

// EAConfig EA配置
type EAConfig struct {
	EAID         string                 `json:"ea_id"`
	UserID       string                 `json:"user_id"`
	EAName       string                 `json:"ea_name"`
	Symbol       string                 `json:"symbol"`
	Timeframe    string                 `json:"timeframe"`
	Strategy     string                 `json:"strategy"`
	RiskPercent  float64                `json:"risk_percent"`
	MaxPositions int                    `json:"max_positions"`
	Enabled      bool                   `json:"enabled"`
	MT4AccountID int64                  `json:"mt4_account_id"` // MT4账户ID
	Params       map[string]interface{} `json:"params"` // 通用参数，每个EA自己解析
}

// UserInfo 用户信息
type UserInfo struct {
	UserID   int64
	Username string
	Balance  float64
}

// User 交易用户
type User struct {
	UserID     string
	Username   string
	Balance    float64
	Equity     float64
	Margin     float64
	FreeMargin float64
}

// EAInstance EA实例
type EAInstance struct {
	config       EAConfig
	user         *User
	rdb          *redis.Client
	ctx          context.Context
	cancel       context.CancelFunc // 添加取消函数
	positions    map[string]*Position
	positionsMu  sync.RWMutex
	signalChan   chan Signal
	stopChan     chan struct{}
	pauseChan    chan bool
	paused       bool
	pausedMu     sync.RWMutex
	stopped      bool           // 添加停止标志
	stoppedMu    sync.RWMutex   // 保护stopped
	tradeManager *TradeManager
}

// NewEAInstance 创建EA实例
func NewEAInstance(config EAConfig, user *User, rdb *redis.Client, tm *TradeManager) *EAInstance {
	ctx, cancel := context.WithCancel(context.Background())
	return &EAInstance{
		config:       config,
		user:         user,
		rdb:          rdb,
		ctx:          ctx,
		cancel:       cancel,
		positions:    make(map[string]*Position),
		signalChan:   make(chan Signal, 100),
		stopChan:     make(chan struct{}),
		pauseChan:    make(chan bool, 1),
		paused:       false,
		stopped:      false,
		tradeManager: tm,
	}
}

// Start 启动EA
func (ea *EAInstance) Start() {
	if !ea.config.Enabled {
		log.Printf("[%s] EA未启用,跳过启动", ea.config.EAID)
		return
	}

	log.Printf("[%s] EA启动: User=%s, Symbol=%s, Timeframe=%s, Strategy=%s",
		ea.config.EAID, ea.user.Username, ea.config.Symbol, ea.config.Timeframe, ea.config.Strategy)

	// 启动指标订阅
	go ea.subscribeIndicator()

	// 启动信号处理
	go ea.processSignals()
}

// Pause 暂停EA
func (ea *EAInstance) Pause() {
	ea.pausedMu.Lock()
	ea.paused = true
	ea.pausedMu.Unlock()
}

// Resume 恢复EA
func (ea *EAInstance) Resume() {
	ea.pausedMu.Lock()
	ea.paused = false
	ea.pausedMu.Unlock()
}

// Stop 停止EA
func (ea *EAInstance) Stop() {
	ea.stoppedMu.Lock()
	defer ea.stoppedMu.Unlock()

	if ea.stopped {
		log.Printf("[%s] EA已经停止", ea.config.EAID)
		return
	}

	log.Printf("[%s] EA停止", ea.config.EAID)
	ea.stopped = true
	
	// 取消context
	ea.cancel()
	
	// 关闭stopChan
	close(ea.stopChan)
}

// subscribeIndicator 订阅指标数据
func (ea *EAInstance) subscribeIndicator() {
	channel := fmt.Sprintf("indicator:%s:%s:green_arrow", ea.config.Symbol, ea.config.Timeframe)
	pubsub := ea.rdb.Subscribe(ea.ctx, channel)
	defer pubsub.Close()

	log.Printf("[%s] 订阅指标: %s", ea.config.EAID, channel)

	ch := pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			ea.handleIndicator(msg.Payload)
		case <-ea.stopChan:
			return
		}
	}
}

// handleIndicator 处理指标数据
func (ea *EAInstance) handleIndicator(payload string) {
	// 检查是否暂停
	ea.pausedMu.RLock()
	paused := ea.paused
	ea.pausedMu.RUnlock()

	if paused {
		return
	}

	var indicator indicators.GreenArrowResult
	if err := json.Unmarshal([]byte(payload), &indicator); err != nil {
		log.Printf("[%s] 解析指标失败: %v", ea.config.EAID, err)
		return
	}

	// 根据策略生成信号
	signal := ea.generateSignal(indicator)
	if signal != nil {
		ea.signalChan <- *signal
	}
}

// generateSignal 根据指标生成交易信号
func (ea *EAInstance) generateSignal(indicator indicators.GreenArrowResult) *Signal {
	// 只在新信号时才交易
	if !indicator.IsSignal {
		return nil
	}

	var signal Signal
	signal.Symbol = ea.config.Symbol
	signal.Timeframe = ea.config.Timeframe
	signal.Timestamp = time.Now()
	signal.Trend = indicator.Trend

	switch ea.config.Strategy {
	case "trend_following":
		// 趋势跟踪策略
		if indicator.Trend == 1 && indicator.UpSignal > 0 {
			signal.Type = "BUY"
			signal.Price = indicator.UpSignal
			signal.StopLoss = indicator.UpStop
			return &signal
		} else if indicator.Trend == -1 && indicator.DownSignal > 0 {
			signal.Type = "SELL"
			signal.Price = indicator.DownSignal
			signal.StopLoss = indicator.DownStop
			return &signal
		}
	}

	return nil
}

// processSignals 处理交易信号
func (ea *EAInstance) processSignals() {
	for {
		select {
		case signal := <-ea.signalChan:
			ea.executeSignal(signal)
		case <-ea.stopChan:
			return
		}
	}
}

// executeSignal 执行交易信号
func (ea *EAInstance) executeSignal(signal Signal) {
	log.Printf("[%s] 收到信号: %s %s @ %.2f, SL=%.2f",
		ea.config.EAID, signal.Type, signal.Symbol, signal.Price, signal.StopLoss)

	// 检查是否达到最大持仓数
	ea.positionsMu.RLock()
	openCount := len(ea.positions)
	ea.positionsMu.RUnlock()

	if openCount >= ea.config.MaxPositions {
		log.Printf("[%s] 已达到最大持仓数 %d,跳过信号", ea.config.EAID, ea.config.MaxPositions)
		return
	}

	// 计算手数
	lots := ea.calculateLots(signal.Price, signal.StopLoss)

	// 创建交易请求
	req := TradeRequest{
		UserID:     ea.user.UserID,
		EAID:       ea.config.EAID,
		Symbol:     signal.Symbol,
		Type:       signal.Type,
		Lots:       lots,
		StopLoss:   signal.StopLoss,
		TakeProfit: 0,
	}

	// 提交交易
	resp := ea.tradeManager.ExecuteTrade(req)
	if resp.Success {
		log.Printf("[%s] ✅ 开仓成功: %s", ea.config.EAID, resp.PositionID)

		// 添加到本地持仓列表
		position := &Position{
			PositionID: resp.PositionID,
			UserID:     req.UserID,
			EAID:       req.EAID,
			Symbol:     req.Symbol,
			Type:       req.Type,
			Lots:       req.Lots,
			OpenPrice:  signal.Price,
			StopLoss:   req.StopLoss,
			TakeProfit: req.TakeProfit,
			OpenTime:   time.Now(),
			Status:     "OPEN",
		}

		ea.positionsMu.Lock()
		ea.positions[resp.PositionID] = position
		ea.positionsMu.Unlock()
	} else {
		log.Printf("[%s] ❌ 开仓失败: %s", ea.config.EAID, resp.Message)
	}
}

// calculateLots 计算手数
func (ea *EAInstance) calculateLots(entryPrice, stopLoss float64) float64 {
	// 风险金额 = 账户余额 × 风险百分比
	riskAmount := ea.user.Balance * ea.config.RiskPercent / 100

	// 风险点数
	pointsRisk := abs(entryPrice - stopLoss)
	if pointsRisk == 0 {
		return 0.01 // 最小手数
	}

	// 每点价值 (简化计算,实际需要根据品种)
	pointValue := 10.0 // XAUUSD: 1手 = $10/点

	// 计算手数
	lots := riskAmount / (pointsRisk * pointValue)

	// 限制手数范围
	if lots < 0.01 {
		lots = 0.01
	}
	if lots > 10.0 {
		lots = 10.0
	}

	return roundLots(lots)
}

// GetStatus 获取EA状态
func (ea *EAInstance) GetStatus() map[string]interface{} {
	ea.positionsMu.RLock()
	defer ea.positionsMu.RUnlock()

	ea.pausedMu.RLock()
	paused := ea.paused
	ea.pausedMu.RUnlock()

	return map[string]interface{}{
		"ea_id":         ea.config.EAID,
		"user_id":       ea.user.UserID,
		"username":      ea.user.Username,
		"symbol":        ea.config.Symbol,
		"timeframe":     ea.config.Timeframe,
		"strategy":      ea.config.Strategy,
		"enabled":       ea.config.Enabled,
		"paused":        paused,
		"risk_percent":  ea.config.RiskPercent,
		"max_positions": ea.config.MaxPositions,
		"open_positions": len(ea.positions),
	}
}

// Signal 交易信号
type Signal struct {
	Symbol    string
	Timeframe string
	Type      string
	Price     float64
	StopLoss  float64
	Trend     int
	Timestamp time.Time
}

// Position 持仓信息
type Position struct {
	PositionID string
	UserID     string
	EAID       string
	Symbol     string
	Type       string
	Lots       float64
	OpenPrice  float64
	StopLoss   float64
	TakeProfit float64
	OpenTime   time.Time
	CloseTime  time.Time
	ClosePrice float64
	Profit     float64
	Status     string // OPEN, CLOSED
}

// TradeRequest 交易请求
type TradeRequest struct {
	UserID       string
	EAID         string
	MT4AccountID int64   // 添加MT4账户ID
	Symbol       string
	Type         string
	Lots         float64
	StopLoss     float64
	TakeProfit   float64
}

// TradeResponse 交易响应
type TradeResponse struct {
	Success    bool
	PositionID string
	Message    string
}

// TradeManager 交易管理器
type TradeManager struct {
	mu        sync.RWMutex
	positions map[string]*Position
	users     map[string]*User
}

// NewTradeManager 创建交易管理器
func NewTradeManager() *TradeManager {
	return &TradeManager{
		positions: make(map[string]*Position),
		users:     make(map[string]*User),
	}
}

// RegisterUser 注册用户
func (tm *TradeManager) RegisterUser(user *User) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.users[user.UserID] = user
	log.Printf("用户注册到交易管理器: %s (余额: $%.2f)", user.Username, user.Balance)
}

// ExecuteTrade 执行交易
func (tm *TradeManager) ExecuteTrade(req TradeRequest) TradeResponse {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 验证用户
	user, exists := tm.users[req.UserID]
	if !exists {
		return TradeResponse{
			Success: false,
			Message: "用户不存在",
		}
	}

	// 计算所需保证金 (简化: 1手 = $1000保证金)
	requiredMargin := req.Lots * 1000

	// 检查可用保证金
	if user.FreeMargin < requiredMargin {
		return TradeResponse{
			Success: false,
			Message: "保证金不足",
		}
	}

	// 创建持仓
	position := &Position{
		PositionID: uuid.New().String(),
		UserID:     req.UserID,
		EAID:       req.EAID,
		Symbol:     req.Symbol,
		Type:       req.Type,
		Lots:       req.Lots,
		OpenPrice:  0,
		StopLoss:   req.StopLoss,
		TakeProfit: req.TakeProfit,
		OpenTime:   time.Now(),
		Status:     "OPEN",
	}

	// 更新用户保证金
	user.Margin += requiredMargin
	user.FreeMargin -= requiredMargin

	// 保存持仓
	tm.positions[position.PositionID] = position

	log.Printf("[交易] 开仓成功: User=%s, EA=%s, %s %s %.2f手, SL=%.2f",
		req.UserID, req.EAID, req.Type, req.Symbol, req.Lots, req.StopLoss)

	return TradeResponse{
		Success:    true,
		PositionID: position.PositionID,
		Message:    "开仓成功",
	}
}

// UserExists 检查用户是否已注册
func (tm *TradeManager) UserExists(userID string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	_, exists := tm.users[userID]
	return exists
}

// 辅助函数
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func roundLots(lots float64) float64 {
	// 保留2位小数
	return float64(int(lots*100+0.5)) / 100
}
