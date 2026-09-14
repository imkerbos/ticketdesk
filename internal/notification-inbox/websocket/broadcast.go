// Package websocket: 跨副本的通知广播
package websocket

import (
	"context"
	"encoding/json"

	redisv9 "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/pkg/logger"
	pkgRedis "github.com/kerbos/ticketdesk/pkg/redis"
	"github.com/kerbos/ticketdesk/pkg/safego"
)

// broadcastChannel 跨副本广播使用的 Redis 频道
const broadcastChannel = "ticketdesk:ws:broadcast"

// broadcastEnvelope 广播消息信封
type broadcastEnvelope struct {
	UserID uint64    `json:"user_id"`
	Msg    WSMessage `json:"msg"`
	// Origin 发出广播的副本标识，用于避免自己处理自己发的消息
	Origin string `json:"origin"`
}

// StartBroadcastSubscriber 订阅跨副本广播
//
// 存在的原因：连接表是进程内的 map。多副本部署（helm 默认 replicas: 2）下，
// 用户的 WebSocket 只连在其中一个 Pod 上，而产生通知的请求可能落在另一个 Pod，
// 那条通知就永远推不出去 —— 副本数为 2 时约一半的实时通知直接丢失。
//
// Redis 不可用时静默降级为单副本行为（仅本地推送），不阻断启动。
func (m *Manager) StartBroadcastSubscriber(ctx context.Context) {
	client := pkgRedis.GetClient()
	if client == nil {
		logger.Warn("websocket broadcast disabled: redis unavailable (多副本部署下通知只会送达本副本)")
		return
	}

	sub := client.Subscribe(ctx, broadcastChannel)
	safego.Go("websocket.broadcastSubscriber", func() {
		defer sub.Close()
		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				m.handleBroadcast(msg)
			}
		}
	})

	logger.Info("websocket broadcast subscriber started", zap.String("channel", broadcastChannel))
}

// handleBroadcast 处理收到的广播消息
func (m *Manager) handleBroadcast(msg *redisv9.Message) {
	defer safego.Recover("websocket.handleBroadcast")

	var env broadcastEnvelope
	if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
		logger.Warn("failed to decode ws broadcast", zap.Error(err))
		return
	}
	// 自己发出的广播已在本地投递过，跳过
	if env.Origin == m.instanceID {
		return
	}
	m.deliverLocal(env.UserID, &env.Msg)
}

// publishBroadcast 把消息发到 Redis，让其它副本也尝试投递
func (m *Manager) publishBroadcast(userID uint64, msg *WSMessage) {
	client := pkgRedis.GetClient()
	if client == nil {
		return
	}

	payload, err := json.Marshal(broadcastEnvelope{UserID: userID, Msg: *msg, Origin: m.instanceID})
	if err != nil {
		logger.Warn("failed to encode ws broadcast", zap.Error(err))
		return
	}

	if err := client.Publish(context.Background(), broadcastChannel, payload).Err(); err != nil {
		logger.Warn("failed to publish ws broadcast", zap.Error(err))
	}
}
