package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024
)

// Client WebSocket 连接和 Hub 之间的中间人
type Client struct {
	Hub           *Hub
	Conn          *websocket.Conn
	Send          chan []byte        // 传出消息通道
	Subscriptions map[string]bool    // 此客户端订阅的频道 (用于清理)
}

// readPump 将消息从 WebSocket 连接泵送到 Hub (处理订阅)
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c // 告诉 Hub 自己断开了
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error { _ = c.Conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break // 退出循环, 触发 defer
		}

		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Failed to unmarshal client message: %v", err)
			continue
		}

		channel, err := msg.ToChannelName()
		if err != nil {
			log.Printf("Invalid message from client: %v", err)
			continue
		}

		// 根据消息执行 Hub 操作
		switch msg.Action {
		case "subscribe":
			c.Hub.Subscribe(c, channel)
			c.Subscriptions[channel] = true
		case "unsubscribe":
			c.Hub.Unsubscribe(c, channel)
			delete(c.Subscriptions, channel)
		}
	}
}

// WritePump 将消息从 Hub 泵送到 WebSocket 连接 (推送K线)
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok { // Hub 关闭了通道
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return // 写入失败, 退出循环
			}
		case <-ticker.C: // Ping 消息
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
