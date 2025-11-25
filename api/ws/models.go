package ws

import "fmt"

// ClientMessage 前端发送给我们的订阅/取消订阅消息
type ClientMessage struct {
	Action    string `json:"action"`    // "subscribe" 或 "unsubscribe"
	Symbol    string `json:"symbol"`    // "XAUUSD"
	Timeframe string `json:"timeframe"` // "M1", "H1"
}

// ToChannelName 将客户端消息转换为Redis的频道名称
func (cm *ClientMessage) ToChannelName() (string, error) {
	if cm.Symbol == "" || cm.Timeframe == "" {
		return "", fmt.Errorf("invalid message: symbol and timeframe are required")
	}
	return fmt.Sprintf("kline:%s:%s", cm.Symbol, cm.Timeframe), nil
}
