// Package storage 提供文件存储抽象
//
// 存在的原因：早期实现直接把文件写在容器本地目录（uploads/），
// K8s 上靠一块 ReadWriteOnce 的 PVC 承载。Deployment + RWO PVC + 多副本天生不兼容 ——
// 滚动升级时新 Pod 抢不到卷，触发 Multi-Attach error 卡在 Init；
// 强行多副本则会出现「A 副本传的附件在 B 副本上 404」。
//
// 抽象出本接口后，后端不再持有本地状态，可以随意扩副本。
// 老部署继续用 local driver，行为不变。
package storage

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"
)

// ErrNotFound 对象不存在
var ErrNotFound = errors.New("storage.object_not_found")

// ErrInvalidKey 非法的对象键
var ErrInvalidKey = errors.New("storage.invalid_key")

// Object 读取对象时返回的内容与元信息
type Object struct {
	Body        io.ReadCloser
	Size        int64
	ContentType string
}

// Storage 文件存储接口
//
// key 是相对于存储根的路径，形如 "attachments/xxx.png"、"brand/logo.svg"。
// 各实现必须把 key 当作不可信输入处理（见 CleanKey）。
type Storage interface {
	// Save 写入对象；size 为 -1 表示未知大小
	Save(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	// Open 读取对象，调用方负责关闭 Body
	Open(ctx context.Context, key string) (*Object, error)
	// Delete 删除对象；对象不存在时返回 nil
	Delete(ctx context.Context, key string) error
	// Exists 判断对象是否存在
	Exists(ctx context.Context, key string) bool
	// Driver 返回驱动名，用于日志与健康检查
	Driver() string
}

// CleanKey 规范化并校验对象键
//
// 附件名来自用户上传，品牌资源名来自后台输入，都不可信。
// 这里统一剥离路径穿越成分：本地驱动下 "../../etc/passwd" 会写穿容器文件系统，
// 对象存储下也会造出难以清理的畸形键。
func CleanKey(key string) (string, error) {
	k := strings.TrimSpace(key)
	k = strings.ReplaceAll(k, "\\", "/")
	k = strings.TrimPrefix(k, "/")
	if k == "" {
		return "", ErrInvalidKey
	}

	// path.Clean 会消解 ".." 与 "."；清理后仍以 ".." 开头说明试图越出根目录
	k = path.Clean(k)
	if k == "." || k == ".." || strings.HasPrefix(k, "../") {
		return "", ErrInvalidKey
	}
	if strings.ContainsRune(k, 0) {
		return "", ErrInvalidKey
	}
	return k, nil
}

// JoinKey 在指定前缀下拼出对象键，并保证结果不会越出该前缀。
//
// 目前两个调用方（附件上传、品牌资源上传）各自用固定前缀 + 已清洗的文件名，
// 前缀边界是靠约定维持的。用本函数把它变成结构性约束：
// name 里一旦出现路径分隔符或 ".."，直接拒绝，
// 避免将来新增调用方时不小心让附件写进品牌资源命名空间（反之亦然）。
func JoinKey(prefix, name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" || n == "." || n == ".." {
		return "", ErrInvalidKey
	}
	if strings.ContainsAny(n, `/\`) || strings.ContainsRune(n, 0) {
		return "", ErrInvalidKey
	}

	key, err := CleanKey(prefix + "/" + n)
	if err != nil {
		return "", err
	}
	// 兜底：清理后必须仍在该前缀之下
	if !strings.HasPrefix(key, prefix+"/") {
		return "", ErrInvalidKey
	}
	return key, nil
}

// 常用的键前缀
const (
	// PrefixAttachments 工单附件
	PrefixAttachments = "attachments"
	// PrefixBrand 品牌资源（Logo / Favicon）
	PrefixBrand = "brand"
)
