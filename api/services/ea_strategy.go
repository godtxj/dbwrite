package services

import (
	"api/ws/indicators"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// EAStrategy EA策略接口 - 每个EA实现这个接口
type EAStrategy interface {
	// GetName 获取EA名称
	GetName() string
	
	// GetIndicatorChannel 获取需要订阅的Redis指标频道
	GetIndicatorChannel(symbol, timeframe string) string
	
	// ProcessIndicator 处理指标数据，返回交易信号
	ProcessIndicator(payload string) (*Signal, error)
	
	// CalculateLots 计算手数
	CalculateLots(balance, riskPercent, entryPrice, stopLoss float64) float64
	
	// ValidateParams 验证参数
	ValidateParams(params map[string]interface{}) error
}

// BaseEAStrategy 基础EA策略（提供通用方法）
type BaseEAStrategy struct {
	Name   string
	Params map[string]interface{}
}

func (b *BaseEAStrategy) GetName() string {
	return b.Name
}

func (b *BaseEAStrategy) CalculateLots(balance, riskPercent, entryPrice, stopLoss float64) float64 {
	riskAmount := balance * riskPercent / 100
	pointsRisk := abs(entryPrice - stopLoss)
	if pointsRisk == 0 {
		return 0.01
	}
	
	pointValue := 10.0 // XAUUSD: 1手 = $10/点
	lots := riskAmount / (pointsRisk * pointValue)
	
	if lots < 0.01 {
		lots = 0.01
	}
	if lots > 10.0 {
		lots = 10.0
	}
	
	return roundLots(lots)
}

func (b *BaseEAStrategy) ValidateParams(params map[string]interface{}) error {
	// 默认不验证，子类可以覆盖
	return nil
}

// ============================================
// GreenArrow EA 策略实现
// ============================================

type GreenArrowEA struct {
	BaseEAStrategy
	IndicatorParams indicators.GreenArrowParams
}

func NewGreenArrowEA(params map[string]interface{}) *GreenArrowEA {
	// 解析指标参数
	indicatorParams := indicators.GreenArrowParams{
		Length:    8,
		Deviation: 1,
		MoneyRisk: 1.0,
		Signal:    1,
		Line:      1,
	}
	
	if p, ok := params["indicator_params"].(map[string]interface{}); ok {
		if length, ok := p["length"].(float64); ok {
			indicatorParams.Length = int(length)
		}
		if deviation, ok := p["deviation"].(float64); ok {
			indicatorParams.Deviation = int(deviation)
		}
		if moneyRisk, ok := p["money_risk"].(float64); ok {
			indicatorParams.MoneyRisk = moneyRisk
		}
		if signal, ok := p["signal"].(float64); ok {
			indicatorParams.Signal = int(signal)
		}
		if line, ok := p["line"].(float64); ok {
			indicatorParams.Line = int(line)
		}
	}
	
	return &GreenArrowEA{
		BaseEAStrategy: BaseEAStrategy{
			Name:   "GreenArrow",
			Params: params,
		},
		IndicatorParams: indicatorParams,
	}
}

func (ga *GreenArrowEA) GetIndicatorChannel(symbol, timeframe string) string {
	return fmt.Sprintf("indicator:%s:%s:green_arrow", symbol, timeframe)
}

func (ga *GreenArrowEA) ProcessIndicator(payload string) (*Signal, error) {
	var indicator indicators.GreenArrowResult
	if err := json.Unmarshal([]byte(payload), &indicator); err != nil {
		return nil, fmt.Errorf("解析指标失败: %v", err)
	}
	
	// 只在新信号时才交易
	if !indicator.IsSignal {
		return nil, nil
	}
	
	var signal Signal
	signal.Timestamp = time.Now()
	signal.Trend = indicator.Trend
	
	// 趋势跟踪策略
	if indicator.Trend == 1 && indicator.UpSignal > 0 {
		signal.Type = "BUY"
		signal.Price = indicator.UpSignal
		signal.StopLoss = indicator.UpStop
		return &signal, nil
	} else if indicator.Trend == -1 && indicator.DownSignal > 0 {
		signal.Type = "SELL"
		signal.Price = indicator.DownSignal
		signal.StopLoss = indicator.DownStop
		return &signal, nil
	}
	
	return nil, nil
}

// ============================================
// MACD EA 策略实现（示例）
// ============================================

type MACDEA struct {
	BaseEAStrategy
	FastPeriod   int
	SlowPeriod   int
	SignalPeriod int
}

func NewMACDEA(params map[string]interface{}) *MACDEA {
	fastPeriod := 12
	slowPeriod := 26
	signalPeriod := 9
	
	if p, ok := params["indicator_params"].(map[string]interface{}); ok {
		if fast, ok := p["fast_period"].(float64); ok {
			fastPeriod = int(fast)
		}
		if slow, ok := p["slow_period"].(float64); ok {
			slowPeriod = int(slow)
		}
		if signal, ok := p["signal_period"].(float64); ok {
			signalPeriod = int(signal)
		}
	}
	
	return &MACDEA{
		BaseEAStrategy: BaseEAStrategy{
			Name:   "MACD",
			Params: params,
		},
		FastPeriod:   fastPeriod,
		SlowPeriod:   slowPeriod,
		SignalPeriod: signalPeriod,
	}
}

func (m *MACDEA) GetIndicatorChannel(symbol, timeframe string) string {
	return fmt.Sprintf("indicator:%s:%s:macd", symbol, timeframe)
}

func (m *MACDEA) ProcessIndicator(payload string) (*Signal, error) {
	// TODO: 实现MACD指标解析和信号生成
	// 这里只是示例框架
	log.Printf("[MACD EA] 收到指标数据: %s", payload)
	return nil, nil
}

// ============================================
// EA工厂 - 根据EA类型创建对应的策略
// ============================================

type EAFactory struct{}

func NewEAFactory() *EAFactory {
	return &EAFactory{}
}

// CreateStrategy 根据EA名称创建策略
func (f *EAFactory) CreateStrategy(eaName string, params map[string]interface{}) (EAStrategy, error) {
	switch eaName {
	case "GreenArrow", "绿箭侠", "Green Arrow EA":
		return NewGreenArrowEA(params), nil
	case "MACD", "MACD EA":
		return NewMACDEA(params), nil
	// 未来添加更多EA类型：
	// case "MA":
	//     return NewMAEA(params), nil
	// case "Bollinger":
	//     return NewBollingerEA(params), nil
	default:
		return nil, fmt.Errorf("不支持的EA类型: %s", eaName)
	}
}

// ============================================
// 策略化的EA实例
// ============================================

type StrategyEAInstance struct {
	orderID      int64
	config       EAConfig
	user         *User
	strategy     EAStrategy // 使用策略接口
	rdb          *redis.Client
	ctx          context.Context
	cancel       context.CancelFunc
	positions    map[string]*Position
	positionsMu  sync.RWMutex
	signalChan   chan Signal
	stopChan     chan struct{}
	paused       bool
	pausedMu     sync.RWMutex
	stopped      bool
	stoppedMu    sync.RWMutex
	tradeManager *TradeManager
}

func NewStrategyEAInstance(orderID int64, config EAConfig, user *User, strategy EAStrategy, rdb *redis.Client, tm *TradeManager) *StrategyEAInstance {
	ctx, cancel := context.WithCancel(context.Background())
	return &StrategyEAInstance{
		orderID:      orderID,
		config:       config,
		user:         user,
		strategy:     strategy,
		rdb:          rdb,
		ctx:          ctx,
		cancel:       cancel,
		positions:    make(map[string]*Position),
		signalChan:   make(chan Signal, 100),
		stopChan:     make(chan struct{}),
		paused:       false,
		stopped:      false,
		tradeManager: tm,
	}
}

func (ea *StrategyEAInstance) Start() {
	if !ea.config.Enabled {
		log.Printf("[Order %d] EA未启用,跳过启动", ea.orderID)
		return
	}

	log.Printf("[Order %d] EA启动: Strategy=%s, User=%s, Symbol=%s, Timeframe=%s",
		ea.orderID, ea.strategy.GetName(), ea.user.Username, ea.config.Symbol, ea.config.Timeframe)

	// 启动指标订阅
	go ea.subscribeIndicator()

	// 启动信号处理
	go ea.processSignals()
}

func (ea *StrategyEAInstance) subscribeIndicator() {
	// 使用策略提供的频道
	channel := ea.strategy.GetIndicatorChannel(ea.config.Symbol, ea.config.Timeframe)
	pubsub := ea.rdb.Subscribe(ea.ctx, channel)
	defer pubsub.Close()

	log.Printf("[Order %d] 订阅指标: %s", ea.orderID, channel)

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

func (ea *StrategyEAInstance) handleIndicator(payload string) {
	// 检查是否暂停
	ea.pausedMu.RLock()
	paused := ea.paused
	ea.pausedMu.RUnlock()

	if paused {
		return
	}

	// 使用策略处理指标
	signal, err := ea.strategy.ProcessIndicator(payload)
	if err != nil {
		log.Printf("[Order %d] 处理指标失败: %v", ea.orderID, err)
		return
	}

	if signal != nil {
		signal.Symbol = ea.config.Symbol
		signal.Timeframe = ea.config.Timeframe
		ea.signalChan <- *signal
	}
}

func (ea *StrategyEAInstance) processSignals() {
	for {
		select {
		case signal := <-ea.signalChan:
			ea.executeSignal(signal)
		case <-ea.stopChan:
			return
		}
	}
}

func (ea *StrategyEAInstance) executeSignal(signal Signal) {
	log.Printf("[Order %d] 收到信号: %s %s @ %.2f, SL=%.2f",
		ea.orderID, signal.Type, signal.Symbol, signal.Price, signal.StopLoss)

	// 检查是否达到最大持仓数
	ea.positionsMu.RLock()
	openCount := len(ea.positions)
	ea.positionsMu.RUnlock()

	if openCount >= ea.config.MaxPositions {
		log.Printf("[Order %d] 已达到最大持仓数 %d,跳过信号", ea.orderID, ea.config.MaxPositions)
		return
	}

	// 使用策略计算手数
	lots := ea.strategy.CalculateLots(ea.user.Balance, ea.config.RiskPercent, signal.Price, signal.StopLoss)

	// 创建交易请求
	req := TradeRequest{
		UserID:       ea.user.UserID,
		EAID:         ea.config.EAID,
		MT4AccountID: ea.config.MT4AccountID, // 添加MT4账户ID
		Symbol:       signal.Symbol,
		Type:         signal.Type,
		Lots:         lots,
		StopLoss:     signal.StopLoss,
		TakeProfit:   0,
	}

	// 提交交易
	resp := ea.tradeManager.ExecuteTrade(req)
	if resp.Success {
		log.Printf("[Order %d] ✅ 开仓成功: %s", ea.orderID, resp.PositionID)

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
		log.Printf("[Order %d] ❌ 开仓失败: %s", ea.orderID, resp.Message)
	}
}

func (ea *StrategyEAInstance) Pause() {
	ea.pausedMu.Lock()
	ea.paused = true
	ea.pausedMu.Unlock()
	log.Printf("[Order %d] EA已暂停", ea.orderID)
}

func (ea *StrategyEAInstance) Resume() {
	ea.pausedMu.Lock()
	ea.paused = false
	ea.pausedMu.Unlock()
	log.Printf("[Order %d] EA已恢复", ea.orderID)
}

func (ea *StrategyEAInstance) Stop() {
	ea.stoppedMu.Lock()
	defer ea.stoppedMu.Unlock()

	if ea.stopped {
		log.Printf("[Order %d] EA已经停止", ea.orderID)
		return
	}

	log.Printf("[Order %d] EA停止", ea.orderID)
	ea.stopped = true
	ea.cancel()
	close(ea.stopChan)
}

func (ea *StrategyEAInstance) GetStatus() map[string]interface{} {
	ea.positionsMu.RLock()
	defer ea.positionsMu.RUnlock()

	ea.pausedMu.RLock()
	paused := ea.paused
	ea.pausedMu.RUnlock()

	return map[string]interface{}{
		"order_id":       ea.orderID,
		"ea_name":        ea.strategy.GetName(),
		"user_id":        ea.user.UserID,
		"username":       ea.user.Username,
		"symbol":         ea.config.Symbol,
		"timeframe":      ea.config.Timeframe,
		"enabled":        ea.config.Enabled,
		"paused":         paused,
		"risk_percent":   ea.config.RiskPercent,
		"max_positions":  ea.config.MaxPositions,
		"open_positions": len(ea.positions),
	}
}

