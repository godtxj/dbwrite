package ws

import (
	"api/ws/indicators"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

// Hub 维护所有活跃的客户端和订阅关系
type Hub struct {
	Clients          map[*Client]bool
	Subscriptions    map[string]map[*Client]bool // Key: 频道, Value: 客户端Set
	subMutex         sync.RWMutex                // 保护 subscriptions
	RedisMessages    chan *redis.Message         // 从 Redis 传入的消息
	Register         chan *Client                // 注册
	Unregister       chan *Client                // 注销
	indicatorManager *MultiPeriodManager         // 指标管理器
	redisClient      *redis.Client               // Redis客户端 (用于发布指标)
	ctx              context.Context             // Context
}

// NewHub 创建Hub
func NewHub(maxCandles int, redisClient *redis.Client, db *sqlx.DB) *Hub {
	return &Hub{
		Clients:          make(map[*Client]bool),
		Subscriptions:    make(map[string]map[*Client]bool),
		RedisMessages:    make(chan *redis.Message, 1024),
		Register:         make(chan *Client),
		Unregister:       make(chan *Client),
		indicatorManager: NewMultiPeriodManager(maxCandles, db),
		redisClient:      redisClient,
		ctx:              context.Background(),
	}
}

// KlineMessage K线消息结构
type KlineMessage struct {
	Symbol    string     `json:"symbol"`
	Timeframe string     `json:"timeframe"`
	Candle    CandleData `json:"candle"`
	IsNew     bool       `json:"is_new"`
}

// EnhancedKlineMessage 增强的K线消息 (包含指标)
type EnhancedKlineMessage struct {
	Symbol     string                       `json:"symbol"`
	Timeframe  string                       `json:"timeframe"`
	Candle     CandleData                   `json:"candle"`
	IsNew      bool                         `json:"is_new"`
	Indicators *IndicatorData               `json:"indicators,omitempty"`
}

// IndicatorData 指标数据
type IndicatorData struct {
	GreenArrow *indicators.GreenArrowResult `json:"green_arrow,omitempty"`
}

// Run 启动 Hub 的主循环
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client] = true
			log.Printf("Client registered: %s", client.Conn.RemoteAddr())

		case client := <-h.Unregister:
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send) // 关闭发送通道
				h.cleanUpSubscriptions(client) // 关键清理
				log.Printf("Client unregistered: %s", client.Conn.RemoteAddr())
			}

		case msg := <-h.RedisMessages:
			// 从 Redis 收到K线, 计算指标后转发给所有订阅者
			h.handleKlineMessage(msg)
		}
	}
}

// handleKlineMessage 处理K线消息
func (h *Hub) handleKlineMessage(msg *redis.Message) {
	channel := msg.Channel
	log.Printf("🔄 Processing kline message from channel: %s", channel)

	// 尝试解析candle service的格式 (带status字段)
	var candleServiceMsg struct {
		Status string `json:"status"` // "UPDATE" or "CLOSE"
		Candle struct {
			Symbol    string    `json:"symbol"`
			Timeframe string    `json:"timeframe"`
			StartTime time.Time `json:"start_time"`
			Open      float64   `json:"open"`
			High      float64   `json:"high"`
			Low       float64   `json:"low"`
			Close     float64   `json:"close"`
			Volume    int64     `json:"volume"`
		} `json:"candle"`
	}

	if err := json.Unmarshal([]byte(msg.Payload), &candleServiceMsg); err == nil && candleServiceMsg.Status != "" {
		// 成功解析candle service格式
		log.Printf("✅ Parsed candle service format: %s %s (status=%s)", 
			candleServiceMsg.Candle.Symbol, candleServiceMsg.Candle.Timeframe, candleServiceMsg.Status)
		isNew := (candleServiceMsg.Status == "CLOSE")
		
		// 转换为内部格式
		klineMsg := KlineMessage{
			Symbol:    candleServiceMsg.Candle.Symbol,
			Timeframe: candleServiceMsg.Candle.Timeframe,
			Candle: CandleData{
				Time:   candleServiceMsg.Candle.StartTime,
				Open:   candleServiceMsg.Candle.Open,
				High:   candleServiceMsg.Candle.High,
				Low:    candleServiceMsg.Candle.Low,
				Close:  candleServiceMsg.Candle.Close,
				Volume: candleServiceMsg.Candle.Volume,
			},
			IsNew: isNew,
		}

		// 提取symbol和timeframe
		key := klineMsg.Symbol + ":" + klineMsg.Timeframe

		// 添加到缓冲区
		h.indicatorManager.AddCandle(key, klineMsg.Candle, klineMsg.IsNew)

		// ✅ 简化方案：获取完整的K线列表
		allCandles := h.indicatorManager.GetCandles(key)
		
		if len(allCandles) == 0 {
			log.Printf("⚠️  No candles available for %s", key)
			return
		}

		// ✅ 创建snapshot消息（包含完整K线列表）
		snapshot := SnapshotMessage{
			Type:      "snapshot",
			Symbol:    klineMsg.Symbol,
			Timeframe: klineMsg.Timeframe,
			Data:      allCandles,
		}

		// 序列化snapshot消息
		payload, err := json.Marshal(snapshot)
		if err != nil {
			log.Printf("Failed to marshal snapshot message: %v", err)
			return
		}

		// 转发完整K线列表给WebSocket客户端
		h.forwardMessage(channel, payload)
		log.Printf("✅ Forwarded complete kline list: %s %s (%d candles, subscribers=%d)", 
			klineMsg.Symbol, klineMsg.Timeframe, len(allCandles), h.getSubscriberCount(channel))

		// 计算并发布指标结果到Redis (供EA订阅)
		indicatorResults := h.indicatorManager.CalculateIndicators(key)
		if len(indicatorResults) > 0 {
			lastInd := indicatorResults[len(indicatorResults)-1]
			h.publishIndicatorToRedis(klineMsg.Symbol, klineMsg.Timeframe, &lastInd)
		}
		return
	}

	// 如果不是candle service格式，尝试解析旧格式
	var klineMsg KlineMessage
	if err := json.Unmarshal([]byte(msg.Payload), &klineMsg); err != nil {
		log.Printf("Failed to parse kline message: %v", err)
		return
	}

	// 提取symbol和timeframe
	key := klineMsg.Symbol + ":" + klineMsg.Timeframe

	// 添加到缓冲区
	h.indicatorManager.AddCandle(key, klineMsg.Candle, klineMsg.IsNew)

	// ✅ 简化方案：获取完整的K线列表
	allCandles := h.indicatorManager.GetCandles(key)
	
	if len(allCandles) == 0 {
		log.Printf("⚠️  No candles available for %s", key)
		return
	}

	// ✅ 创建snapshot消息（包含完整K线列表）
	snapshot := SnapshotMessage{
		Type:      "snapshot",
		Symbol:    klineMsg.Symbol,
		Timeframe: klineMsg.Timeframe,
		Data:      allCandles,
	}

	// 序列化snapshot消息
	payload, err := json.Marshal(snapshot)
	if err != nil {
		log.Printf("Failed to marshal snapshot message: %v", err)
		return
	}

	// 转发完整K线列表给WebSocket客户端
	h.forwardMessage(channel, payload)
	log.Printf("✅ Forwarded complete kline list: %s %s (%d candles, subscribers=%d)", 
		klineMsg.Symbol, klineMsg.Timeframe, len(allCandles), h.getSubscriberCount(channel))

	// 计算并发布指标结果到Redis (供EA订阅)
	indicatorResults := h.indicatorManager.CalculateIndicators(key)
	if len(indicatorResults) > 0 {
		lastInd := indicatorResults[len(indicatorResults)-1]
		h.publishIndicatorToRedis(klineMsg.Symbol, klineMsg.Timeframe, &lastInd)
	}
}

// publishIndicatorToRedis 发布指标结果到Redis
func (h *Hub) publishIndicatorToRedis(symbol, timeframe string, indicator *indicators.GreenArrowResult) {
	// 频道格式: indicator:{symbol}:{timeframe}:green_arrow
	channel := fmt.Sprintf("indicator:%s:%s:green_arrow", symbol, timeframe)

	// 序列化指标数据
	data, err := json.Marshal(indicator)
	if err != nil {
		log.Printf("Failed to marshal indicator for Redis: %v", err)
		return
	}

	// 发布到Redis
	if err := h.redisClient.Publish(h.ctx, channel, data).Err(); err != nil {
		log.Printf("Failed to publish indicator to Redis: %v", err)
	}
}

// forwardMessage 转发消息给订阅者
func (h *Hub) forwardMessage(channel string, payload []byte) {
	h.subMutex.RLock()
	defer h.subMutex.RUnlock()

	if clients, ok := h.Subscriptions[channel]; ok {
		for client := range clients {
			select {
			case client.Send <- payload: // 发送
			default: // 客户端缓冲满, 丢弃
				log.Printf("Client send buffer full. Dropping message for %s", client.Conn.RemoteAddr())
			}
		}
	}
}

func (h *Hub) Subscribe(client *Client, channel string) {
	h.subMutex.Lock()
	if _, ok := h.Subscriptions[channel]; !ok {
		h.Subscriptions[channel] = make(map[*Client]bool)
	}
	h.Subscriptions[channel][client] = true
	h.subMutex.Unlock()  // ✅ 提前释放锁
	
	log.Printf("Client %s subscribed to %s", client.Conn.RemoteAddr(), channel)
	
	// ✅ 在锁外发送快照消息（避免死锁）
	go h.sendSnapshot(client, channel)  // ✅ 异步发送
}

func (h *Hub) Unsubscribe(client *Client, channel string) {
	h.subMutex.Lock()
	defer h.subMutex.Unlock()
	if clients, ok := h.Subscriptions[channel]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.Subscriptions, channel)
		}
	}
}

// cleanUpSubscriptions 当客户端断开时, 清理其所有订阅
func (h *Hub) cleanUpSubscriptions(client *Client) {
	h.subMutex.Lock()
	defer h.subMutex.Unlock()
	for channel := range client.Subscriptions { // 遍历客户端的订阅列表
		if clients, ok := h.Subscriptions[channel]; ok {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.Subscriptions, channel) // 如果频道空了, 也删除
			}
		}
	}
}

// UpdateIndicatorParams 更新指标参数
func (h *Hub) UpdateIndicatorParams(params indicators.GreenArrowParams) {
	h.indicatorManager.UpdateParams(params)
	log.Printf("Indicator params updated: %+v", params)
}

// getSubscriberCount 获取频道订阅者数量
func (h *Hub) getSubscriberCount(channel string) int {
	h.subMutex.RLock()
	defer h.subMutex.RUnlock()
	if clients, ok := h.Subscriptions[channel]; ok {
		return len(clients)
	}
	return 0
}

// SnapshotMessage 快照消息
type SnapshotMessage struct {
	Type      string       `json:"type"`      // "snapshot"
	Symbol    string       `json:"symbol"`
	Timeframe string       `json:"timeframe"`
	Data      []CandleData `json:"data"`
}

// UpdateMessage 增量更新消息
type UpdateMessage struct {
	Type       string         `json:"type"`       // "update"
	Symbol     string         `json:"symbol"`
	Timeframe  string         `json:"timeframe"`
	Action     string         `json:"action"`     // "UPDATE" or "NEW"
	Candle     CandleData     `json:"candle"`
	Indicators *IndicatorData `json:"indicators,omitempty"`
}

// sendSnapshot 发送快照消息给客户端
func (h *Hub) sendSnapshot(client *Client, channel string) {
	// 解析channel: kline:SYMBOL:TIMEFRAME
	parts := splitChannel(channel)
	if len(parts) != 3 || parts[0] != "kline" {
		log.Printf("Invalid channel format: %s", channel)
		return
	}
	
	symbol := parts[1]
	timeframe := parts[2]
	key := symbol + ":" + timeframe
	
	// 从缓冲区获取所有K线数据
	candles := h.indicatorManager.GetCandles(key)
	
	if len(candles) == 0 {
		log.Printf("No candles available for %s", key)
		return
	}
	
	// 创建快照消息
	snapshot := SnapshotMessage{
		Type:      "snapshot",
		Symbol:    symbol,
		Timeframe: timeframe,
		Data:      candles,
	}
	
	// 序列化并发送
	payload, err := json.Marshal(snapshot)
	if err != nil {
		log.Printf("Failed to marshal snapshot: %v", err)
		return
	}
	
	select {
	case client.Send <- payload:
		if len(candles) > 0 {
			firstTime := candles[0].Time.Format("2006-01-02 15:04:05")
			lastTime := candles[len(candles)-1].Time.Format("2006-01-02 15:04:05")
			firstPrice := candles[0].Close
			lastPrice := candles[len(candles)-1].Close
			log.Printf("✅ Sent snapshot to client %s: %s (%d candles)", 
				client.Conn.RemoteAddr(), key, len(candles))
			log.Printf("   Time range: %s to %s", firstTime, lastTime)
			log.Printf("   Price range: %.2f to %.2f", firstPrice, lastPrice)
		}
	default:
		log.Printf("Client send buffer full, dropping snapshot")
	}
}

// splitChannel 分割频道字符串
func splitChannel(channel string) []string {
	result := []string{}
	start := 0
	for i := 0; i < len(channel); i++ {
		if channel[i] == ':' {
			result = append(result, channel[start:i])
			start = i + 1
		}
	}
	result = append(result, channel[start:])
	return result
}
