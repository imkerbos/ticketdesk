package safego

import (
	"sync"
	"testing"
	"time"

	"github.com/kerbos/ticketdesk/pkg/config"
	"github.com/kerbos/ticketdesk/pkg/logger"
)

func TestMain(m *testing.M) {
	// Recover 内部会写日志，未初始化时 logger 为 nil 会二次 panic
	_ = logger.Init(&config.LogConfig{Level: "error", Format: "json", Output: "stdout"})
	m.Run()
}

// TestGoRecoversPanic 后台协程 panic 必须被吞掉，而不是打崩整个进程。
// Gin 的 Recovery 中间件覆盖不到派生协程，这是本包存在的唯一理由。
func TestGoRecoversPanic(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	Go("test.panicking", func() {
		defer wg.Done()
		var m map[string]string
		// 故意向 nil map 写入以触发 panic，这正是本用例要验证的场景
		//nolint:staticcheck // SA5000: 有意为之
		m["boom"] = "nil map write"
	})

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
		// 进程还活着 = 通过
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine 未在预期时间内结束")
	}
}

// TestRecoverAsDefer 覆盖直接 defer safego.Recover 的用法
// （现有 15 处 `go func(){...}` 就是这么改的）。
func TestRecoverAsDefer(t *testing.T) {
	survived := false

	func() {
		defer func() { survived = true }()
		defer Recover("test.deferred")
		panic("nested panic")
	}()

	if !survived {
		t.Fatal("Recover 未拦住 panic")
	}
}

func TestGoRunsNormalFunc(t *testing.T) {
	ch := make(chan int, 1)
	Go("test.normal", func() { ch <- 42 })

	select {
	case got := <-ch:
		if got != 42 {
			t.Fatalf("got %d, want 42", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("正常函数未被执行")
	}
}
