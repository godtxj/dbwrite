package controllers

import (
	"api/ws"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true }, // 允许跨域
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WSController WebSocket控制器
type WSController struct {
	hub *ws.Hub
}

// NewWSController 创建WebSocket控制器
func NewWSController(hub *ws.Hub) *WSController {
	return &WSController{
		hub: hub,
	}
}

// HandleWebSocket 处理WebSocket连接
// @Summary WebSocket实时推送
// @Description 建立WebSocket连接,订阅K线和指标数据
// @Tags WebSocket
// @Accept json
// @Produce json
// @Router /ws [get]
func (wsc *WSController) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}

	client := &ws.Client{
		Hub:           wsc.hub,
		Conn:          conn,
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}

	client.Hub.Register <- client

	// 启动该客户端的读/写 goroutine
	go client.WritePump()
	go client.ReadPump()
}

// RegisterRoutes 注册路由
func (wsc *WSController) RegisterRoutes(router *gin.Engine) {
	router.GET("/ws", wsc.HandleWebSocket)
}
