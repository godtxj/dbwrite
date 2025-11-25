package main

import (
	"encoding/json"
	"time"
)

// Candle K线结构
type Candle struct {
	StartTime time.Time `json:"start_time" db:"start_time"`
	Symbol    string    `json:"symbol" db:"symbol"`
	Timeframe string    `json:"timeframe" db:"timeframe"`
	Open      float64   `json:"open" db:"open"`
	High      float64   `json:"high" db:"high"`
	Low       float64   `json:"low" db:"low"`
	Close     float64   `json:"close" db:"close"`
	Volume    int64     `json:"volume" db:"volume"`
}

// Redis收到的结构
type PublishEvent struct {
	Status string `json:"status"` // "UPDATE" 或 "CLOSE"
	Candle Candle `json:"candle"`
}

// ParseEvent 辅助函数
func ParseEvent(payload []byte) (*PublishEvent, error) {
	var event PublishEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return &event, nil
}