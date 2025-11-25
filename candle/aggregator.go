package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// --- 管理单个周期 (例如 1M) ---
type TimeframeAggregator struct {
	Symbol       string
	Timeframe    time.Duration
	TfName       string // "M1", "M5" ...
	currentCandle *Candle
	lock         sync.Mutex // 保护此周期的 currentCandle
	redisClient  *redis.Client
	redisChannel string
}

func NewTimeframeAggregator(symbol, tfName string, timeframe time.Duration, rdb *redis.Client) *TimeframeAggregator {
	return &TimeframeAggregator{
		Symbol:       symbol,
		Timeframe:    timeframe,
		TfName:       tfName,
		redisClient:  rdb,
		redisChannel: fmt.Sprintf("kline:%s:%s", symbol, tfName),
	}
}

func (t *TimeframeAggregator) ProcessTick(tick CleanTick) {
	t.lock.Lock()
	defer t.lock.Unlock()

	tickWindowStart := tick.Timestamp.Truncate(t.Timeframe)

	// 情况一：第一根K线
	if t.currentCandle == nil {
		t.currentCandle = &Candle{
			Symbol:    t.Symbol, Timeframe: t.TfName, StartTime: tickWindowStart,
			Open: tick.Price, High: tick.Price, Low: tick.Price, Close: tick.Price, Volume: tick.Volume,
		}
		t.publishCandle("UPDATE")
		return
	}

	// 情况二：Tick 属于一根新K线
	if tickWindowStart.After(t.currentCandle.StartTime) {
		// 检测时间跳跃（可能丢失了中间的K线）
		missedBars := int(tickWindowStart.Sub(t.currentCandle.StartTime) / t.Timeframe)
		if missedBars > 1 {
			log.Printf("⚠️  Time gap detected for %s:%s - missed %d bars (from %s to %s)", 
				t.Symbol, t.TfName, missedBars-1,
				t.currentCandle.StartTime.Format("15:04:05"),
				tickWindowStart.Format("15:04:05"))
			
			// 填充缺失的K线（使用上一根的收盘价作为OHLC）
			t.fillMissingBars(missedBars - 1)
		}
		
		t.publishCandle("CLOSE") // 关闭旧K线
		t.currentCandle = &Candle{
			Symbol:    t.Symbol, Timeframe: t.TfName, StartTime: tickWindowStart,
			Open: tick.Price, High: tick.Price, Low: tick.Price, Close: tick.Price, Volume: tick.Volume,
		}
		t.publishCandle("UPDATE") // 开启新K线
		return
	}

	// 情况三：Tick 属于当前K线
	if tickWindowStart.Equal(t.currentCandle.StartTime) {
		t.currentCandle.High = max(t.currentCandle.High, tick.Price)
		t.currentCandle.Low = min(t.currentCandle.Low, tick.Price)
		t.currentCandle.Close = tick.Price
		t.currentCandle.Volume += tick.Volume
		t.publishCandle("UPDATE") // 实时跳动
		return
	}
	
	// 情况四：乱序tick（时间戳早于当前K线）
	if tickWindowStart.Before(t.currentCandle.StartTime) {
		log.Printf("⚠️  Out-of-order tick for %s:%s (tick: %s, current: %s) - ignoring",
			t.Symbol, t.TfName, 
			tickWindowStart.Format("15:04:05"),
			t.currentCandle.StartTime.Format("15:04:05"))
		return
	}
}

// 填充缺失的K线
func (t *TimeframeAggregator) fillMissingBars(count int) {
	if t.currentCandle == nil || count <= 0 {
		return
	}
	
	lastClose := t.currentCandle.Close
	currentTime := t.currentCandle.StartTime
	
	for i := 0; i < count; i++ {
		currentTime = currentTime.Add(t.Timeframe)
		
		// 创建一个平坦的K线（OHLC都相同）
		missingCandle := &Candle{
			Symbol:    t.Symbol,
			Timeframe: t.TfName,
			StartTime: currentTime,
			Open:      lastClose,
			High:      lastClose,
			Low:       lastClose,
			Close:     lastClose,
			Volume:    0, // 无成交量
		}
		
		// 发布缺失的K线
		event := PublishEvent{Status: "CLOSE", Candle: *missingCandle}
		go func(e PublishEvent) {
			err := t.redisClient.Publish(context.Background(), t.redisChannel, e.ToJSON()).Err()
			if err != nil {
				log.Printf("ERROR: Failed to publish missing bar: %v", err)
			}
		}(event)
		
		log.Printf("📝 Filled missing bar for %s:%s at %s", 
			t.Symbol, t.TfName, currentTime.Format("15:04:05"))
	}
}

func (t *TimeframeAggregator) publishCandle(status string) {
	if t.currentCandle == nil { return }
	event := PublishEvent{ Status: status, Candle: *t.currentCandle }
	go func() { // 异步发布, 不阻塞K线聚合
		err := t.redisClient.Publish(context.Background(), t.redisChannel, event.ToJSON()).Err()
		if err != nil {
			log.Printf("ERROR: Redis Publish to %s failed: %v", t.redisChannel, err)
		}
	}()
}


// --- 管理单个品种 (例如 XAUUSD) 的所有周期 ---
type SymbolAggregator struct {
	Symbol     string
	Timeframes map[string]*TimeframeAggregator
}

func NewSymbolAggregator(symbol string, rdb *redis.Client) *SymbolAggregator {
	sa := &SymbolAggregator{
		Symbol:     symbol,
		Timeframes: make(map[string]*TimeframeAggregator),
	}
	sa.Timeframes["M1"] = NewTimeframeAggregator(symbol, "M1", time.Minute, rdb)
	sa.Timeframes["M5"] = NewTimeframeAggregator(symbol, "M5", 5*time.Minute, rdb)
	sa.Timeframes["M15"] = NewTimeframeAggregator(symbol, "M15", 15*time.Minute, rdb)
	sa.Timeframes["M30"] = NewTimeframeAggregator(symbol, "M30", 30*time.Minute, rdb)
	sa.Timeframes["H1"] = NewTimeframeAggregator(symbol, "H1", time.Hour, rdb)
	sa.Timeframes["H4"] = NewTimeframeAggregator(symbol, "H4", 4*time.Hour, rdb)
	sa.Timeframes["D1"] = NewTimeframeAggregator(symbol, "D1", 24*time.Hour, rdb)
	return sa
}

func (s *SymbolAggregator) ProcessTick(tick CleanTick) {
	for _, tfAgg := range s.Timeframes {
		tfAgg.ProcessTick(tick)
	}
}



type AggregatorManager struct {
	Aggregators  map[string]*SymbolAggregator // 存储聚合器实例
	Channels     map[string]chan CleanTick    // 存储每个Symbol的专属通道
	lock         sync.RWMutex                 // 保护上面两个 map
	redisClient  *redis.Client
	droppedTicks map[string]int64             // 统计每个品种丢弃的tick数量
	statsLock    sync.Mutex                   // 保护统计数据
}

func NewAggregatorManager(rdb *redis.Client) *AggregatorManager {
	am := &AggregatorManager{
		Aggregators:  make(map[string]*SymbolAggregator),
		Channels:     make(map[string]chan CleanTick),
		redisClient:  rdb,
		droppedTicks: make(map[string]int64),
	}
	
	// 启动监控goroutine，每30秒输出统计信息
	go am.monitorStats()
	
	return am
}

func (m *AggregatorManager) HandleRawQuote(quote UpstreamQuote) {
	cleanTick, err := m.parseQuote(quote)
	if err != nil {
		log.Printf("Failed to parse quote: %v", err)
		return
	}

	m.lock.RLock()
	tickChannel, exists := m.Channels[cleanTick.Symbol]
	m.lock.RUnlock()

	if !exists {
		m.lock.Lock()
		if tickChannel, exists = m.Channels[cleanTick.Symbol]; !exists {
			log.Printf("🔧 Creating new Worker/Channel for %s", cleanTick.Symbol)
			sa := NewSymbolAggregator(cleanTick.Symbol, m.redisClient)
			tickChannel = make(chan CleanTick, 5000) // 增加缓冲容量到5000
			m.Aggregators[cleanTick.Symbol] = sa
			m.Channels[cleanTick.Symbol] = tickChannel
			
			go m.startSymbolWorker(sa, tickChannel) // 启动专属的 Goroutine (工人)
		}
		m.lock.Unlock()
	}

	// 改进的背压机制：阻塞发送 + 超时
	// 优先尝试立即发送
	select {
	case tickChannel <- cleanTick:
		// 成功发送
		return
	default:
		// Channel满，记录警告并尝试等待
		queueLen := len(tickChannel)
		log.Printf("⚠️  Channel busy for %s (queue: %d/5000), waiting...", cleanTick.Symbol, queueLen)
	}
	
	// 带超时的阻塞发送
	select {
	case tickChannel <- cleanTick:
		// 成功发送
		log.Printf("✅ Tick sent after waiting for %s", cleanTick.Symbol)
	case <-time.After(500 * time.Millisecond):
		// 超时，记录丢弃
		m.statsLock.Lock()
		m.droppedTicks[cleanTick.Symbol]++
		dropped := m.droppedTicks[cleanTick.Symbol]
		m.statsLock.Unlock()
		
		log.Printf("🔴 DROPPED tick for %s (total dropped: %d) - Worker may be stuck!", 
			cleanTick.Symbol, dropped)
	}
}

// 每个品种专属的“工人”
func (m *AggregatorManager) startSymbolWorker(agg *SymbolAggregator, ch chan CleanTick) {
	log.Printf("🚀 Worker started for %s", agg.Symbol)
	tickCount := 0
	lastLog := time.Now()
	
	for tick := range ch {
		agg.ProcessTick(tick)
		tickCount++
		
		// 每10秒输出一次处理速度
		if time.Since(lastLog) > 10*time.Second {
			queueLen := len(ch)
			log.Printf("📊 %s: processed %d ticks in 10s (queue: %d/5000)", 
				agg.Symbol, tickCount, queueLen)
			tickCount = 0
			lastLog = time.Now()
			
			// 如果队列积压严重，发出警告
			if queueLen > 4000 {
				log.Printf("⚠️  WARNING: %s queue is %d%% full!", 
					agg.Symbol, queueLen*100/5000)
			}
		}
	}
}

func (m *AggregatorManager) parseQuote(quote UpstreamQuote) (CleanTick, error) {
	if quote.Type != "Quote" {
		return CleanTick{}, fmt.Errorf("not a quote message")
	}
	args := quote.Data.Args
	
	const layout = "2006-01-02T15:04:05" 
	ts, err := time.ParseInLocation(layout, args.Time, time.UTC) 
	if err != nil {
		return CleanTick{}, fmt.Errorf("invalid time format: %s", args.Time)
	}

	cleanSymbol := args.Symbol
	if strings.Contains(cleanSymbol, ".") {
		parts := strings.SplitN(cleanSymbol, ".", 2) // 按第一个 "." 分割
		cleanSymbol = parts[0] // 只取第一部分
	}

	return CleanTick{
		Symbol:    cleanSymbol,
		Price:     args.Bid,   // 使用 Bid
		Volume:    1,          // 使用 Tick Volume
		Timestamp: ts,
	}, nil
}

// 监控统计信息
func (m *AggregatorManager) monitorStats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		m.statsLock.Lock()
		if len(m.droppedTicks) > 0 {
			log.Println("📈 === Dropped Ticks Statistics ===")
			for symbol, count := range m.droppedTicks {
				if count > 0 {
					log.Printf("   %s: %d ticks dropped", symbol, count)
				}
			}
		}
		m.statsLock.Unlock()
		
		m.lock.RLock()
		log.Printf("📊 Active workers: %d symbols", len(m.Aggregators))
		for symbol, ch := range m.Channels {
			queueLen := len(ch)
			if queueLen > 1000 {
				log.Printf("   %s: queue %d/5000 (%d%%)", 
					symbol, queueLen, queueLen*100/5000)
			}
		}
		m.lock.RUnlock()
	}
}

// 辅助函数
func max(a, b float64) float64 { if a > b { return a }; return b }
func min(a, b float64) float64 { if a < b { return a }; return b }