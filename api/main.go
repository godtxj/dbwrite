package main

import (
	"api/config"
	"api/controllers"
	"api/database"
	"api/middleware"
	"api/services"
	"api/ws"
	"log"
	"os"
	"time"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/gin-gonic/gin"

	_ "api/docs" // Swagger docs
)

// @title           MT4 Trading API
// @version         1.0
// @description     MT4 Trading System API with WebSocket real-time push and EA automation
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@example.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// 1. 加载配置
	cfg := config.LoadConfig()
	log.Println("Configuration loaded")

	// 2. 初始化MySQL数据库连接（全局单例）
	if err := database.InitDB(cfg); err != nil {
		log.Fatalf("FATAL: Failed to connect to MySQL: %v", err)
	}
	defer database.CloseDB()

	// 3. 初始化Redis连接
	if err := database.InitRedis(cfg.RedisHost, cfg.RedisPort, cfg.RedisPassword, cfg.RedisDB); err != nil {
		log.Fatalf("FATAL: Failed to connect to Redis: %v", err)
	}
	defer database.CloseRedis()

	// 4. 初始化PostgreSQL/TimescaleDB连接（用于K线数据）
	pgDB, err := sqlx.Connect("postgres", cfg.GetPGDSN())
	if err != nil {
		log.Fatalf("FATAL: Failed to connect to PostgreSQL/TimescaleDB: %v", err)
	}
	defer pgDB.Close()
	log.Println("Connected to PostgreSQL/TimescaleDB for kline data")

	// 5. 创建JWT中间件实例（全局单例）
	jwtMiddleware := middleware.NewJWTMiddleware(cfg.JWTSecret, cfg.JWTExpireDuration)
	log.Println("JWT middleware initialized")

	// 6. 创建限流中间件
	rateLimiter := middleware.NewRateLimiter(database.GetRedis())
	log.Println("Rate limiter initialized")

	// 7. 创建Gin路由
	router := gin.Default()

	// 8. 应用全局中间件
	router.Use(middleware.CORS())
	router.Use(rateLimiter.GlobalLimit()) // 全局限流

	// 9. 创建服务层
	userService := services.NewUserService(database.GetDB())
	mt4Service := services.NewMT4Service(database.GetDB())
	emailService := services.NewEmailService(
		cfg.SMTPHost,
		cfg.SMTPPort,
		cfg.SMTPUser,
		cfg.SMTPPassword,
		cfg.SMTPFrom,
	)
	verificationService := services.NewRedisVerificationService(database.GetRedis(), emailService)
	captchaService := services.NewCaptchaService(database.GetRedis())
	log.Println("Services initialized")

	// 10. 创建WebSocket Hub
	wsHub := ws.NewHub(500, database.GetRedis(), pgDB) // 500根K线缓冲, 传入PostgreSQL连接
	go wsHub.Run()                                     // 启动Hub
	pubSubManager := ws.NewPubSubManager(database.GetRedis(), wsHub)
	go pubSubManager.Run() // 启动Redis订阅
	log.Println("WebSocket Hub initialized")

	// 11. 创建EA运行时服务
	earuntimeService := services.NewEARuntimeService(database.GetRedis())
	log.Println("EA Runtime Service initialized")

	// 12. 创建控制器
	userController := controllers.NewUserController(userService, jwtMiddleware, verificationService, captchaService)
	mt4Controller := controllers.NewMT4Controller(mt4Service, jwtMiddleware, earuntimeService)
	captchaController := controllers.NewCaptchaController(captchaService)
	wsController := controllers.NewWSController(wsHub)
	klineController := controllers.NewKlineController(pgDB) // 传入PostgreSQL连接
	log.Println("Controllers initialized")

	// 13. 注册路由（包含限流）
	userController.RegisterRoutesWithRateLimit(router, rateLimiter)
	mt4Controller.RegisterRoutes(router)
	captchaController.RegisterRoutes(router)
	wsController.RegisterRoutes(router)
	klineController.RegisterRoutes(router)

	// 14. Swagger文档（仅开发环境）
	if os.Getenv("GIN_MODE") != "release" {
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		log.Println("Swagger documentation available at: http://localhost:8080/swagger/index.html")
	}

	// 15. 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// 16. 启动服务器
	addr := ":" + cfg.ServerPort
	log.Printf("API Server starting on %s", addr)
	log.Printf("WebSocket endpoint: ws://localhost%s/ws", addr)
	log.Printf("Kline API: GET /api/mt4/kline?symbol=XAUUSD&timeframe=M1&limit=300")
	log.Printf("EA Runtime Service: READY")
	log.Println("⚠️  NOTE: Make sure 'Candle Service' and 'DB Service' are running!")
	if err := router.Run(addr); err != nil {
		log.Fatalf("FATAL: Failed to start API server: %v", err)
	}
}