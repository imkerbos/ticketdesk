// Package safego 提供带 panic 恢复的 goroutine 启动器
//
// 背景：Gin 的 Recovery 中间件只能兜住处理请求的那个 goroutine，
// 对 handler / service 里 `go func(){...}` 派生出去的协程完全无效。
// 项目中通知下发、告警处理、工作流推进等都是异步分支，
// 其中任意一处 panic（空指针、越界、向已关闭 channel 发送）都会直接杀掉整个进程。
//
// 因此所有后台协程一律通过本包启动，不要再裸写 `go func()`。
package safego

import (
	"runtime/debug"

	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/pkg/logger"
)

// Go 启动一个带 panic 恢复的 goroutine。
// name 用于在日志中定位是哪个后台任务出的问题。
func Go(name string, fn func()) {
	go Run(name, fn)
}

// Run 在当前 goroutine 内执行 fn 并捕获 panic。
// 供已经处于独立协程、只需要补一层保护的场景使用。
func Run(name string, fn func()) {
	defer Recover(name)
	fn()
}

// Recover 捕获并记录 panic，需配合 defer 使用。
func Recover(name string) {
	if r := recover(); r != nil {
		logger.Error("goroutine panic recovered",
			zap.String("task", name),
			zap.Any("panic", r),
			zap.ByteString("stack", debug.Stack()),
		)
	}
}
