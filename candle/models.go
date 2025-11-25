package main

import (
	"encoding/json"
	"time"
)

// WS数据结构 - 完整的上游格式
type UpstreamQuote struct {
	Id   int    `json:"Id"`
	Type string `json:"Type"`
	Data struct {
		Args struct {
			Symbol string  `json:"Symbol"`
			Bid    float64 `json:"Bid"`
			Ask    float64 `json:"Ask"`
			Time   string  `json:"Time"` // 格式: "2025-11-24T19:45:19"
			High   float64 `json:"High"`   // 当日最高价
			Low    float64 `json:"Low"`    // 当日最低价
			Spread float64 `json:"Spread"` // 点差
		} `json:"Args"`
		Id         int `json:"Id"`
		Login      int `json:"Login"`
		PlatformId int `json:"PlatformId"`
	} `json:"Data"`
}

// 处理Tick数据
type CleanTick struct {
	Symbol    string
	Price     float64
	Volume    int64
	Timestamp time.Time
}

// Candle K线结构
type Candle struct {
	Symbol    string    `json:"symbol"`
	Timeframe string    `json:"timeframe"`
	StartTime time.Time `json:"start_time"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    int64     `json:"volume"`
}

// 发布到Redis
type PublishEvent struct {
	Status string `json:"status"` // "UPDATE" (K线跳动) 或 "CLOSE" (K线闭合)
	Candle Candle `json:"candle"`
}

// ToJSON 辅助函数
func (e *PublishEvent) ToJSON() []byte {
	b, _ := json.Marshal(e)
	return b
}