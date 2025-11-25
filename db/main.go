package main

import (
	"context"
	"log"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v4/stdlib" 
)

// --- 配置 ---
const (
	DATABASE_URL = "postgres://kline:c75scFhGrbie@localhost:5432/kline?sslmode=disable"
	REDIS_ADDR   = "localhost:6379"
)

func main() {
	rdb := redis.NewClient(&redis.Options{ Addr: REDIS_ADDR })
	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		log.Fatalf("FATAL: Could not connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")

	db, err := sqlx.Connect("pgx", DATABASE_URL)
	if err != nil {
		log.Fatalf("FATAL: Failed to connect to TimescaleDB: %v", err)
	}
	log.Println("Connected to TimescaleDB/PostgreSQL")

	service := NewDBWriterService(rdb, db)
	go service.Run() // 启动服务

	log.Println("DB Writer service is running.")
	select {} // 保持主程序运行
}