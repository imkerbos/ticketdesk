package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestSendToUserRaceWithReconnect 复现并锁定「向已关闭 channel 发送」导致的进程崩溃。
//
// 旧实现在 SendToUser 里先释放读锁再写 client.send，而 Run() 处理 register 时
// 会在写锁内 close(old.send) 顶掉同一用户的旧连接。同一用户反复重连
// （用户刷新页面即可触发）与推送并发时，会命中 "send on closed channel" panic。
//
// 用 -race 跑本用例可同时检出数据竞争。
func TestSendToUserRaceWithReconnect(t *testing.T) {
	m := NewManager()
	go m.Run()

	const userID uint64 = 7

	upgrader := websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool { return true },
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		m.RegisterClient(userID, conn)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// 持续推送
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				m.SendToUser(userID, &WSMessage{Type: "notification", Data: "ping"})
			}
		}
	}()

	// 同一用户反复重连，不断触发 Run() 里的 close(old.send)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 60; i++ {
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				continue
			}
			time.Sleep(time.Millisecond)
			_ = conn.Close()
		}
	}()

	// 等重连循环结束后停止推送
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	time.Sleep(300 * time.Millisecond)
	close(stop)
	<-done
	// 跑到这里没有 panic 即视为通过
}

// TestRunLoopSurvivesPanicInHandler 事件循环必须扛住单次事件处理的 panic。
//
// register 分支里会对被顶掉的旧连接调用 old.conn.Close()。若旧连接的 conn 为 nil
// （异常路径），该调用会 panic。没有就地 recover 时循环直接退出，
// 后果不只是漏一条消息：m.register 是无缓冲通道，无人接收后
// RegisterClient 会永久阻塞，每个新建 WS 连接的 HTTP 协程都挂死，站内通知全线失效。
func TestRunLoopSurvivesPanicInHandler(t *testing.T) {
	m := NewManager()
	go m.Run()

	newClient := func(uid uint64) *WSClient {
		return &WSClient{userID: uid, conn: nil, send: make(chan []byte, 1), manager: m}
	}

	// 先占位，再用同一 userID 顶掉它 —— 触发 old.conn.Close() 的 nil 解引用
	m.register <- newClient(1)
	m.register <- newClient(1)

	// 循环若已死，这次发送会永久阻塞
	sent := make(chan struct{})
	go func() {
		m.register <- newClient(2)
		close(sent)
	}()

	select {
	case <-sent:
	case <-time.After(2 * time.Second):
		t.Fatal("事件循环已退出：register 通道无人接收，RegisterClient 将永久阻塞")
	}

	// 确认后续事件确实被正常处理
	deadline := time.After(time.Second)
	for {
		m.mu.RLock()
		_, ok := m.clients[2]
		m.mu.RUnlock()
		if ok {
			return
		}
		select {
		case <-deadline:
			t.Fatal("panic 之后的注册事件未被处理")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// TestSendToUnknownUserIsNoop 未连接的用户推送应静默丢弃，不阻塞、不 panic。
func TestSendToUnknownUserIsNoop(t *testing.T) {
	m := NewManager()
	go m.Run()

	done := make(chan struct{})
	go func() {
		m.SendToUser(999, &WSMessage{Type: "notification"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("向未连接用户推送发生了阻塞")
	}
}
