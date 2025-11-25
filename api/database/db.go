package database

import (
	"api/config"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"
)

// DB 全局数据库连接
var DB *sqlx.DB

// InitDB 初始化数据库连接
func InitDB(cfg *config.Config) error {
	var err error
	
	// 连接MySQL数据库
	DB, err = sqlx.Connect("mysql", cfg.GetDSN())
	if err != nil {
		return err
	}

	// 设置连接池参数
	DB.SetMaxOpenConns(100)                      // 最大打开连接数
	DB.SetMaxIdleConns(10)                       // 最大空闲连接数
	DB.SetConnMaxLifetime(time.Hour)             // 连接最大生命周期

	// 测试连接
	if err := DB.Ping(); err != nil {
		return err
	}

	log.Println("Database connected successfully")
	return nil
}

// CloseDB 关闭数据库连接
func CloseDB() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}

// GetDB 获取数据库连接
func GetDB() *sqlx.DB {
	return DB
}
