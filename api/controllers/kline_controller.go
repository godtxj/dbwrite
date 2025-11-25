package controllers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// KlineController K线控制器
type KlineController struct {
	pgDB *sqlx.DB
}

// NewKlineController 创建K线控制器
func NewKlineController(pgDB *sqlx.DB) *KlineController {
	return &KlineController{
		pgDB: pgDB,
	}
}

// RegisterRoutes 注册路由
func (kc *KlineController) RegisterRoutes(router *gin.Engine) {
	router.GET("/api/mt4/kline", kc.GetHistoricalKline)
}

// Kline K线数据结构
type Kline struct {
	Time   int64   `json:"time" db:"time"`
	Open   float64 `json:"open" db:"open"`
	High   float64 `json:"high" db:"high"`
	Low    float64 `json:"low" db:"low"`
	Close  float64 `json:"close" db:"close"`
	Volume int64   `json:"volume" db:"volume"`
}

// GetHistoricalKline 获取历史K线数据
// @Summary 获取历史K线
// @Description 从TimescaleDB获取历史K线数据
// @Tags Kline
// @Param symbol query string true "交易品种" default(XAUUSD)
// @Param timeframe query string true "时间周期" default(M1)
// @Param limit query int false "数量限制" default(300)
// @Success 200 {object} map[string]interface{}
// @Router /api/mt4/kline [get]
func (kc *KlineController) GetHistoricalKline(c *gin.Context) {
	symbol := c.DefaultQuery("symbol", "XAUUSD")
	timeframe := c.DefaultQuery("timeframe", "M1")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "300"))

	if limit > 1000 {
		limit = 1000
	}
	if limit < 1 {
		limit = 300
	}

	if kc.pgDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "TimescaleDB连接失败",
		})
		return
	}

	// 查询历史K线，使用PostgreSQL语法
	query := `
		SELECT 
			EXTRACT(EPOCH FROM start_time)::bigint * 1000 as time,
			open,
			high,
			low,
			close,
			volume
		FROM candles
		WHERE symbol = $1 AND timeframe = $2
		ORDER BY start_time DESC
		LIMIT $3
	`

	var klines []Kline
	err := kc.pgDB.Select(&klines, query, symbol, timeframe, limit)
	if err != nil {
		log.Printf("Failed to query klines: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询K线数据失败",
			"error":   err.Error(),
		})
		return
	}

	// 反转数组，使其按时间正序排列（从旧到新）
	for i, j := 0, len(klines)-1; i < j; i, j = i+1, j-1 {
		klines[i], klines[j] = klines[j], klines[i]
	}

	log.Printf("Successfully queried %d klines for %s %s", len(klines), symbol, timeframe)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": klines,
	})
}
