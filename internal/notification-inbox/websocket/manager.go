// Package websocket 提供 WebSocket 连接管理
package websocket

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/pkg/logger"
	"github.com/kerbos/ticketdesk/pkg/safego"
)

const (
	// 写入超时
	writeWait = 10 * time.Second
	// Pong 响应超时
	pongWait = 60 * time.Second
	// Ping 发送间隔（必须小于 pongWait）
	pingPeriod = 30 * time.Second
	// 最大消息大小
	maxMessageSize = 512
)

// WSMessage WebSocket 消息
type WSMessage struct {
	Type string      `json:"type"` // notification, ping, pong
	Data interface{} `json:"data,omitempty"`
}

// WSClient WebSocket 客户端连接
type WSClient struct {
	userID  uint64
	conn    *websocket.Conn
	send    chan []byte
	manager *Manager
}

// Manager WebSocket 连接管理器
type Manager struct {
	clients    map[uint64]*WSClient // userID -> client
	register   chan *WSClient
	unregister chan *WSClient
	mu         sync.RWMutex
	// instanceID 本副本的随机标识，用于在跨副本广播中过滤掉自己发出的消息
	instanceID string
}

// NewManager 创建 WebSocket 管理器
func NewManager() *Manager {
	return &Manager{
		clients:    make(map[uint64]*WSClient),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		instanceID: newInstanceID(),
	}
}

// newInstanceID 生成副本标识
func newInstanceID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// 退化为时间戳，仅用于区分副本，不参与安全判定
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}

// Run 启动管理器事件循环
//
// 单次事件处理的 panic 必须就地兜住：这个循环一旦退出，
// RegisterClient 里的 `m.register <- client` 会永久阻塞（无缓冲通道无人接收），
// 每个新建 WebSocket 连接的 HTTP 协程都会挂死，且站内通知全线静默失效。
// 因此 recover 放在单次迭代内，而不是包住整个 for。
func (m *Manager) Run() {
	for {
		select {
		case client := <-m.register:
			m.handleRegister(client)
		case client := <-m.unregister:
			m.handleUnregister(client)
		}
	}
}

// handleRegister 注册连接，同一用户的旧连接会被顶掉
//
// 清理动作放在锁外：一是 conn.Close() 涉及网络 IO，不该占着全局写锁；
// 二是持锁期间一旦 panic，recover 只能恢复执行流，却无法释放已加的锁 ——
// 那会让整个 Manager 永久死锁，比进程直接崩溃更难排查。
func (m *Manager) handleRegister(client *WSClient) {
	defer safego.Recover("websocket.handleRegister")

	old := m.swapClient(client)

	// 旧连接已从映射中摘除，SendToUser 再也拿不到它，此处关闭不会与推送竞争
	if old != nil {
		close(old.send)
		if old.conn != nil {
			old.conn.Close()
		}
	}

	logger.Info("websocket client registered",
		zap.Uint64("user_id", client.userID),
	)
}

// swapClient 在写锁内换上新连接，返回被顶掉的旧连接（没有则为 nil）
func (m *Manager) swapClient(client *WSClient) *WSClient {
	m.mu.Lock()
	defer m.mu.Unlock()

	old := m.clients[client.userID]
	m.clients[client.userID] = client
	return old
}

// handleUnregister 摘除连接（仅当映射里仍是同一个 client 实例时）
func (m *Manager) handleUnregister(client *WSClient) {
	defer safego.Recover("websocket.handleUnregister")

	// 仅在确实由本次调用摘除时才关闭通道，避免重复 close
	if m.removeClient(client) {
		close(client.send)
	}

	logger.Info("websocket client unregistered",
		zap.Uint64("user_id", client.userID),
	)
}

// removeClient 在写锁内摘除连接，返回是否确实由本次调用摘除
func (m *Manager) removeClient(client *WSClient) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.clients[client.userID]; ok && existing == client {
		delete(m.clients, client.userID)
		return true
	}
	return false
}

// SendToUser 向指定用户推送消息
//
// 写 client.send 必须全程持有读锁: Run() 里关闭 send 通道 (register 顶掉旧连接 /
// unregister 摘除连接) 是在写锁内做的, 一旦这里提前释放读锁再发送,
// 就会撞上 "send on closed channel" panic —— 用户刷新页面重连即可触发, 且会打崩整个进程。
// 读锁与写锁互斥, 因此把 marshal 之后的发送动作留在临界区内即可根除。
func (m *Manager) SendToUser(userID uint64, msg *WSMessage) {
	// 先本地投递；用户不在本副本时再通过 Redis 广播给其它副本
	if !m.deliverLocal(userID, msg) {
		m.publishBroadcast(userID, msg)
	}
}

// deliverLocal 尝试投递给本副本上的连接，返回是否命中
func (m *Manager) deliverLocal(userID uint64, msg *WSMessage) bool {
	data, err := json.Marshal(msg)
	if err != nil {
		logger.Error("failed to marshal ws message", zap.Error(err))
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clients[userID]
	if !ok {
		return false
	}

	// 非阻塞发送, 不会在持锁期间挂住
	select {
	case client.send <- data:
	default:
		// 通道已满，放弃发送
		logger.Warn("ws send channel full, dropping message",
			zap.Uint64("user_id", userID),
		)
	}
	return true
}

// RegisterClient 注册新客户端
func (m *Manager) RegisterClient(userID uint64, conn *websocket.Conn) {
	client := &WSClient{
		userID:  userID,
		conn:    conn,
		send:    make(chan []byte, 256),
		manager: m,
	}

	m.register <- client

	// 启动读写协程（带 panic 恢复：单个连接出问题不应打崩整个进程）
	safego.Go("websocket.readPump", client.readPump)
	safego.Go("websocket.writePump", client.writePump)
}

// readPump 读取客户端消息（处理心跳 pong）
func (c *WSClient) readPump() {
	defer func() {
		c.manager.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		logger.Warn("failed to set ws read deadline", zap.Uint64("user_id", c.userID), zap.Error(err))
		return
	}
	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			logger.Warn("failed to refresh ws read deadline", zap.Uint64("user_id", c.userID), zap.Error(err))
			return err
		}
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logger.Warn("ws unexpected close",
					zap.Uint64("user_id", c.userID),
					zap.Error(err),
				)
			}
			break
		}
	}
}

// writePump 向客户端写入消息（处理心跳 ping）
func (c *WSClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				logger.Warn("failed to set ws write deadline", zap.Uint64("user_id", c.userID), zap.Error(err))
				return
			}
			if !ok {
				// 通道已关闭
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					logger.Warn("failed to write ws close message", zap.Uint64("user_id", c.userID), zap.Error(err))
				}
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				logger.Warn("failed to set ws ping deadline", zap.Uint64("user_id", c.userID), zap.Error(err))
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
