package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

// --- 生产配置 ---
const (
	UPSTREAM_WS_URL = "ws://106.55.179.109:8088/event?id=6" // 您的真实上游数据源
	REDIS_ADDR      = "localhost:6379"
)

var rdb *redis.Client
var ctx = context.Background()

func main() {
	rdb = redis.NewClient(&redis.Options{ Addr: REDIS_ADDR })
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		log.Fatalf("FATAL: Could not connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")

	manager := NewAggregatorManager(rdb)

	go connectAndRead(manager) // 启动上游连接器

	log.Println("Candle Aggregator service is running.")
	select {} // 保持主程序运行
}

// 连接WS并处理自动重连
func connectAndRead(manager *AggregatorManager) {
	for { // 自动重连循环
		log.Printf("Connecting to upstream WebSocket: %s", UPSTREAM_WS_URL)

		c, _, err := websocket.DefaultDialer.Dial(UPSTREAM_WS_URL, nil)
		if err != nil {
			log.Printf("ERROR: Failed to connect to upstream: %v", err)
			time.Sleep(5 * time.Second)
			continue // 重试
		}

		log.Println("Successfully connected to upstream WebSocket.")

		// (可选) 发送订阅消息
		// c.WriteMessage(...)

		// 内部读取循环
		func() {
			defer c.Close()
			for {
				_, message, err := c.ReadMessage()
				if err != nil {
					log.Printf("ERROR: Upstream read error: %v. Connection lost.", err)
					return // 退出内部函数, 触发重连
				}

				var quote UpstreamQuote
				if err := json.Unmarshal(message, &quote); err != nil {
					log.Printf("WARNING: Failed to unmarshal message: %v.", err)
					continue
				}

				// 将解析后的数据交给聚合器
				manager.HandleRawQuote(quote)
			}
		}()
	}
}