package ws

import (
	"api/ws/indicators"
	"sync"
	"time"
	"log"
	"strings"
	"github.com/jmoiron/sqlx"
)

// CandleData K线数据结构
type CandleData struct {
	Time   time.Time `json:"time"`
	Open   float64   `json:"open"`
	High   float64   `json:"high"`
	Low    float64   `json:"low"`
	Close  float64   `json:"close"`
	Volume int64     `json:"volume"`
}

// CandleBuffer K线缓冲区 (固定大小的滑动窗口)
type CandleBuffer struct {
	mu      sync.RWMutex
	candles []CandleData
	maxSize int
}

// NewCandleBuffer 创建K线缓冲区
func NewCandleBuffer(maxSize int) *CandleBuffer {
	return &CandleBuffer{
		candles: make([]CandleData, 0, maxSize),
		maxSize: maxSize,
	}
}

// Add 添加K线 (保持固定数量)
func (cb *CandleBuffer) Add(candle CandleData) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.candles = append(cb.candles, candle)

	// 如果超过最大数量,删除最旧的
	if len(cb.candles) > cb.maxSize {
		cb.candles = cb.candles[1:]
	}
}

// Update 更新最后一根K线
func (cb *CandleBuffer) Update(candle CandleData) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if len(cb.candles) > 0 {
		cb.candles[len(cb.candles)-1] = candle
	} else {
		cb.candles = append(cb.candles, candle)
	}
}

// GetAll 获取所有K线 (从旧到新)
func (cb *CandleBuffer) GetAll() []CandleData {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	result := make([]CandleData, len(cb.candles))
	copy(result, cb.candles)
	return result
}

// Size 获取当前数量
func (cb *CandleBuffer) Size() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return len(cb.candles)
}

// IndicatorCalculator 指标计算器
type IndicatorCalculator struct {
	mu     sync.RWMutex
	params indicators.GreenArrowParams
}

// NewIndicatorCalculator 创建指标计算器
func NewIndicatorCalculator() *IndicatorCalculator {
	return &IndicatorCalculator{
		params: indicators.GreenArrowParams{
			Length:    8,
			Deviation: 1,
			MoneyRisk: 1.0,
			Signal:    1,
			Line:      1,
		},
	}
}

// UpdateParams 更新参数
func (ic *IndicatorCalculator) UpdateParams(params indicators.GreenArrowParams) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	ic.params = params
}

// GetParams 获取参数
func (ic *IndicatorCalculator) GetParams() indicators.GreenArrowParams {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	return ic.params
}

// Calculate 计算指标
func (ic *IndicatorCalculator) Calculate(candles []CandleData) []indicators.GreenArrowResult {
	if len(candles) == 0 {
		return []indicators.GreenArrowResult{}
	}

	// 转换为indicators包的Candle类型
	indCandles := make([]indicators.Candle, len(candles))
	for i, c := range candles {
		indCandles[i] = indicators.Candle{
			Open:   c.Open,
			High:   c.High,
			Low:    c.Low,
			Close:  c.Close,
			Volume: c.Volume,
		}
	}

	// 获取参数
	ic.mu.RLock()
	params := ic.params
	ic.mu.RUnlock()

	// 计算指标
	return indicators.CalculateGreenArrow(indCandles, params)
}

// MultiPeriodManager 多周期管理器
type MultiPeriodManager struct {
	mu         sync.RWMutex
	buffers    map[string]*CandleBuffer // key: "XAUUSD:M5"
	calculator *IndicatorCalculator
	maxSize    int
	db         *sqlx.DB
}

// NewMultiPeriodManager 创建多周期管理器
func NewMultiPeriodManager(maxSize int, db *sqlx.DB) *MultiPeriodManager {
	return &MultiPeriodManager{
		buffers:    make(map[string]*CandleBuffer),
		calculator: NewIndicatorCalculator(),
		maxSize:    maxSize,
		db:         db,
	}
}

// GetOrCreateBuffer 获取或创建缓冲区
func (m *MultiPeriodManager) GetOrCreateBuffer(key string) *CandleBuffer {
	m.mu.RLock()
	if buffer, exists := m.buffers[key]; exists {
		m.mu.RUnlock()
		log.Printf("📦 Using existing buffer for %s (size: %d)", key, buffer.Size())
		return buffer
	}
	m.mu.RUnlock()

	m.mu.Lock()
	// 双重检查（防止并发创建）
	if buffer, exists := m.buffers[key]; exists {
		m.mu.Unlock()
		log.Printf("📦 Using existing buffer for %s (size: %d)", key, buffer.Size())
		return buffer
	}

	log.Printf("🆕 Creating new buffer for %s", key)
	buffer := NewCandleBuffer(m.maxSize)
	m.buffers[key] = buffer
	m.mu.Unlock() // 释放锁后再加载数据
	
	// 同步加载历史数据（不使用goroutine，确保数据加载完成后再返回）
	if m.db != nil {
		m.loadFromDB(key, buffer)
	}
	
	return buffer
}

// loadFromDB 从数据库加载历史数据
func (m *MultiPeriodManager) loadFromDB(key string, buffer *CandleBuffer) {
	parts := strings.Split(key, ":")
	if len(parts) != 2 {
		log.Printf("❌ Invalid key format: %s", key)
		return
	}
	symbol := parts[0]
	timeframe := parts[1]
	
	if m.db == nil {
		log.Printf("❌ Database connection is nil for %s", key)
		return
	}
	
	log.Printf("🔍 Loading history for %s (symbol=%s, timeframe=%s, limit=%d)", key, symbol, timeframe, m.maxSize)

	query := `
		SELECT 
			start_time as time,
			open,
			high,
			low,
			close,
			volume
		FROM klines
		WHERE symbol = $1 AND timeframe = $2
		ORDER BY start_time DESC
		LIMIT $3
	`

	// 定义临时结构体用于扫描
	type DBCandle struct {
		Time   time.Time `db:"time"`
		Open   float64   `db:"open"`
		High   float64   `db:"high"`
		Low    float64   `db:"low"`
		Close  float64   `db:"close"`
		Volume int64     `db:"volume"`
	}

	var dbCandles []DBCandle
	err := m.db.Select(&dbCandles, query, symbol, timeframe, m.maxSize)
	if err != nil {
		log.Printf("❌ Failed to load history for %s: %v", key, err)
		return
	}
	
	log.Printf("📊 Query returned %d candles for %s", len(dbCandles), key)

	// 反转并添加到缓冲区（数据库查询是DESC，需要反转为ASC）
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	
	// 清空现有数据（如果有的话，通常是空的）
	buffer.candles = make([]CandleData, 0, m.maxSize)
	
	validCount := 0
	skippedCount := 0
	var lastTime time.Time
	
	for i := len(dbCandles) - 1; i >= 0; i-- {
		c := dbCandles[i]
		
		// ✅ 数据验证：检查OHLC合理性
		if c.High < c.Low {
			log.Printf("⚠️  Skipping invalid candle from DB: High (%.2f) < Low (%.2f) at %s", 
				c.High, c.Low, c.Time.Format("2006-01-02 15:04:05"))
			skippedCount++
			continue
		}
		if c.High < c.Open || c.High < c.Close {
			log.Printf("⚠️  Skipping invalid candle from DB: High (%.2f) < Open/Close (%.2f/%.2f) at %s", 
				c.High, c.Open, c.Close, c.Time.Format("2006-01-02 15:04:05"))
			skippedCount++
			continue
		}
		if c.Low > c.Open || c.Low > c.Close {
			log.Printf("⚠️  Skipping invalid candle from DB: Low (%.2f) > Open/Close (%.2f/%.2f) at %s", 
				c.Low, c.Open, c.Close, c.Time.Format("2006-01-02 15:04:05"))
			skippedCount++
			continue
		}
		
		// ✅ 时间戳验证：确保递增
		if !lastTime.IsZero() && !c.Time.After(lastTime) {
			log.Printf("⚠️  Skipping duplicate/out-of-order candle from DB at %s (last: %s)", 
				c.Time.Format("2006-01-02 15:04:05"), lastTime.Format("2006-01-02 15:04:05"))
			skippedCount++
			continue
		}
		
		buffer.candles = append(buffer.candles, CandleData{
			Time:   c.Time,
			Open:   c.Open,
			High:   c.High,
			Low:    c.Low,
			Close:  c.Close,
			Volume: c.Volume,
		})
		lastTime = c.Time
		validCount++
	}
	
	if len(buffer.candles) > 0 {
		firstTime := buffer.candles[0].Time.Format("2006-01-02 15:04:05")
		lastTimeStr := buffer.candles[len(buffer.candles)-1].Time.Format("2006-01-02 15:04:05")
		log.Printf("✅ Loaded %d valid candles from DB for %s (skipped: %d)", validCount, key, skippedCount)
		log.Printf("   Time range: %s to %s", firstTime, lastTimeStr)
	} else {
		log.Printf("⚠️  No valid candles loaded from DB for %s (skipped: %d)", key, skippedCount)
	}
}

// AddCandle 添加K线
func (m *MultiPeriodManager) AddCandle(key string, candle CandleData, isNew bool) {
	buffer := m.GetOrCreateBuffer(key)

	// ✅ 添加数据验证
	if candle.High < candle.Low {
		log.Printf("⚠️  Rejecting invalid candle for %s: High (%.2f) < Low (%.2f)", 
			key, candle.High, candle.Low)
		return
	}
	if candle.High < candle.Open || candle.High < candle.Close {
		log.Printf("⚠️  Rejecting invalid candle for %s: High (%.2f) < Open/Close (%.2f/%.2f)", 
			key, candle.High, candle.Open, candle.Close)
		return
	}
	if candle.Low > candle.Open || candle.Low > candle.Close {
		log.Printf("⚠️  Rejecting invalid candle for %s: Low (%.2f) > Open/Close (%.2f/%.2f)", 
			key, candle.Low, candle.Open, candle.Close)
		return
	}

	if isNew {
		buffer.Add(candle)
		log.Printf("➕ Added NEW candle to %s: time=%s, close=%.2f", 
			key, candle.Time.Format("15:04:05"), candle.Close)
	} else {
		buffer.Update(candle)
		// 只在每10次UPDATE时输出一次日志，避免日志过多
		if buffer.Size() % 10 == 0 {
			log.Printf("🔄 Updated candle in %s: time=%s, close=%.2f", 
				key, candle.Time.Format("15:04:05"), candle.Close)
		}
	}
}

// GetCandles 获取K线
func (m *MultiPeriodManager) GetCandles(key string) []CandleData {
	m.mu.RLock()
	buffer, exists := m.buffers[key]
	m.mu.RUnlock()

	if !exists {
		return []CandleData{}
	}

	return buffer.GetAll()
}

// CalculateIndicators 计算指标
func (m *MultiPeriodManager) CalculateIndicators(key string) []indicators.GreenArrowResult {
	candles := m.GetCandles(key)
	return m.calculator.Calculate(candles)
}

// UpdateParams 更新指标参数
func (m *MultiPeriodManager) UpdateParams(params indicators.GreenArrowParams) {
	m.calculator.UpdateParams(params)
}
