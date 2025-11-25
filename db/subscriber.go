package main

import (
	"context"
	"log"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v4/stdlib"
)

type DBWriterService struct {
	rdb *redis.Client
	db  *sqlx.DB
}

func NewDBWriterService(rdb *redis.Client, db *sqlx.DB) *DBWriterService {
	return &DBWriterService{ rdb: rdb, db: db }
}

func (s *DBWriterService) Run() {
	ctx := context.Background()
	pubsub := s.rdb.PSubscribe(ctx, "kline:*:*")
	_, err := pubsub.Receive(ctx)
	if err != nil {
		log.Fatalf("FATAL: Failed to subscribe to Redis kline channels: %v", err)
	}
	log.Println("DB Writer started. Subscribed to 'kline:*:*'")

	ch := pubsub.Channel()
	for msg := range ch {
		// (可选) 可以在 goroutine 中处理, 或使用 worker pool,
		// 但对于DB写入, 顺序处理更简单
		s.processMessage(msg)
	}
}

func (s *DBWriterService) processMessage(msg *redis.Message) {
	event, err := ParseEvent([]byte(msg.Payload))
	if err != nil {
		log.Printf("ERROR: Failed to parse event payload: %v", err)
		return
	}

	// 关键逻辑: 只处理 "CLOSE" 事件
	if event.Status != "CLOSE" {
		return // 忽略 "UPDATE"
	}

	if err := s.insertKlineToDB(event.Candle); err != nil {
		log.Printf("ERROR: Failed to insert kline to DB: %v", err)
	} else {
		log.Printf("DB Write: %s %s StartTime: %s", 
			event.Candle.Symbol, 
			event.Candle.Timeframe, 
			event.Candle.StartTime.Format("15:04:05"))
	}
}

// insertKlineToDB 执行数据库 INSERT 操作 (幂等)
func (s *DBWriterService) insertKlineToDB(c Candle) error {
	// 关键: ON CONFLICT DO NOTHING 保证幂等性
	// (这需要您在DB中为 (symbol, timeframe, start_time) 设置 UNIQUE 约束)
	query := `
		INSERT INTO klines 
			(start_time, symbol, timeframe, open, high, low, close, volume)
		VALUES 
			(:start_time, :symbol, :timeframe, :open, :high, :low, :close, :volume)
		ON CONFLICT (symbol, timeframe, start_time) DO NOTHING;
	`
	
	_, err := s.db.NamedExec(query, c)
	return err
}