package ws

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

// PubSubManager Redis订阅管理器
type PubSubManager struct {
	rdb *redis.Client
	hub *Hub
	ctx context.Context
}

// NewPubSubManager 创建订阅管理器
func NewPubSubManager(rdb *redis.Client, hub *Hub) *PubSubManager {
	return &PubSubManager{
		rdb: rdb,
		hub: hub,
		ctx: context.Background(),
	}
}

// Run 启动订阅
func (pm *PubSubManager) Run() {
	// 订阅所有K线频道
	pubsub := pm.rdb.PSubscribe(pm.ctx, "kline:*:*")
	defer pubsub.Close()

	log.Println("✅ Subscribed to Redis channel: kline:*:*")
	log.Println("⏳ Waiting for kline data from candle service...")

	// 接收消息
	ch := pubsub.Channel()
	for msg := range ch {
		log.Printf("📨 Received Redis message on channel: %s (payload size: %d bytes)", msg.Channel, len(msg.Payload))
		pm.hub.RedisMessages <- msg
	}
	
	log.Println("⚠️  Redis subscription channel closed")
}
