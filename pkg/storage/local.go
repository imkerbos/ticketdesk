// Package storage: 本地文件系统驱动
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// LocalStorage 本地文件存储
//
// 保留此驱动是为了兼容既有部署（单副本 + PVC）。
// 多副本场景请改用 s3 驱动，本驱动的数据只存在于单个 Pod 的卷上。
type LocalStorage struct {
	basePath string
}

// 确保实现了接口
var _ Storage = (*LocalStorage)(nil)

// NewLocalStorage 创建本地存储实例
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	if err := os.MkdirAll(basePath, 0o750); err != nil {
		return nil, fmt.Errorf("创建存储根目录失败: %w", err)
	}
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("解析存储根目录失败: %w", err)
	}
	return &LocalStorage{basePath: abs}, nil
}

// Driver 返回驱动名
func (s *LocalStorage) Driver() string { return DriverLocal }

// resolve 把对象键解析为绝对路径，并确保没有逃出存储根
func (s *LocalStorage) resolve(key string) (string, error) {
	clean, err := CleanKey(key)
	if err != nil {
		return "", err
	}
	full := filepath.Join(s.basePath, filepath.FromSlash(clean))

	// 二次兜底：即便 CleanKey 有疏漏，也不允许最终路径落在根目录之外
	if full != s.basePath && !strings.HasPrefix(full, s.basePath+string(os.PathSeparator)) {
		return "", ErrInvalidKey
	}
	return full, nil
}

// Save 写入对象
func (s *LocalStorage) Save(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	full, err := s.resolve(key)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	dst, err := os.Create(full)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, r); err != nil {
		// 写了一半失败，清掉残留，避免留下损坏的文件
		_ = os.Remove(full)
		return fmt.Errorf("写入文件失败: %w", err)
	}
	return nil
}

// Open 读取对象
func (s *LocalStorage) Open(_ context.Context, key string) (*Object, error) {
	full, err := s.resolve(key)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("读取文件信息失败: %w", err)
	}

	return &Object{
		Body:        f,
		Size:        info.Size(),
		ContentType: mime.TypeByExtension(filepath.Ext(full)),
	}, nil
}

// Delete 删除对象；不存在视为成功
func (s *LocalStorage) Delete(_ context.Context, key string) error {
	full, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("删除文件失败: %w", err)
	}
	return nil
}

// Exists 判断对象是否存在
func (s *LocalStorage) Exists(_ context.Context, key string) bool {
	full, err := s.resolve(key)
	if err != nil {
		return false
	}
	_, err = os.Stat(full)
	return err == nil
}
