package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config 应用配置
type Config struct {
	// 数据库配置
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// 服务器配置
	ServerPort string

	// JWT配置
	JWTSecret         []byte
	JWTExpireDuration time.Duration

	// SMTP配置（QQ邮箱）
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	// Redis配置
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// PostgreSQL/TimescaleDB配置（用于K线数据）
	PGHost     string
	PGPort     string
	PGUser     string
	PGPassword string
	PGDBName   string
}

// LoadConfig 加载配置
func LoadConfig() *Config {
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Println("WARNING: .env file not found, using environment variables or defaults")
	} else {
		log.Println(".env file loaded successfully")
	}

	// 加载JWT过期时间（默认24小时）
	expireHours := getEnvAsInt("JWT_EXPIRE_HOURS", 24)

	// 加载SMTP配置
	smtpPort := getEnvAsInt("SMTP_PORT", 587)

	// 加载Redis配置
	redisDB := getEnvAsInt("REDIS_DB", 0)

	cfg := &Config{
		// 数据库配置
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBUser:     getEnv("DB_USER", "root"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "mt4_trading"),

		// 服务器配置
		ServerPort: getEnv("SERVER_PORT", "8080"),

		// JWT配置
		JWTSecret:         []byte(getEnv("JWT_SECRET", "your-secret-key-change-in-production")),
		JWTExpireDuration: time.Duration(expireHours) * time.Hour,

		// SMTP配置（QQ邮箱）
		SMTPHost:     getEnv("SMTP_HOST", "smtp.qq.com"),
		SMTPPort:     smtpPort,
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", ""),

		// Redis配置
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,

		// PostgreSQL/TimescaleDB配置
		PGHost:     getEnv("PG_HOST", "localhost"),
		PGPort:     getEnv("PG_PORT", "5432"),
		PGUser:     getEnv("PG_USER", "kline"),
		PGPassword: getEnv("PG_PASSWORD", "c75scFhGrbie"),
		PGDBName:   getEnv("PG_DBNAME", "kline"),
	}

	// 生产环境检查
	if string(cfg.JWTSecret) == "your-secret-key-change-in-production" {
		log.Println("WARNING: Using default JWT secret. Set JWT_SECRET environment variable in production!")
	}

	if cfg.SMTPUser == "" || cfg.SMTPPassword == "" {
		log.Println("WARNING: SMTP credentials not configured. Email sending will fail!")
	}

	return cfg
}

// GetDSN 获取MySQL数据库连接字符串
func (c *Config) GetDSN() string {
	// MySQL DSN格式: user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
	return c.DBUser + ":" + c.DBPassword + "@tcp(" + c.DBHost + ":" + c.DBPort + ")/" + c.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
}

// GetPGDSN 获取PostgreSQL数据库连接字符串
func (c *Config) GetPGDSN() string {
	// PostgreSQL DSN格式: postgres://user:password@host:port/dbname?sslmode=disable
	return "postgres://" + c.PGUser + ":" + c.PGPassword + "@" + c.PGHost + ":" + c.PGPort + "/" + c.PGDBName + "?sslmode=disable"
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvAsInt 获取环境变量作为整数
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("WARNING: Invalid integer value for %s, using default %d", key, defaultValue)
		return defaultValue
	}
	return value
}
