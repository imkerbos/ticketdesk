package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestCleanKeyRejectsTraversal 对象键来自用户上传的文件名与后台输入，不可信。
// 未经清理时，local 驱动会把文件写到存储根之外。
func TestCleanKeyRejectsTraversal(t *testing.T) {
	bad := []string{
		"../etc/passwd",
		"../../../../etc/passwd",
		"attachments/../../etc/passwd",
		"/../etc/passwd",
		"..",
		"",
		"   ",
		"a\x00b",
	}
	for _, k := range bad {
		if got, err := CleanKey(k); err == nil {
			t.Errorf("CleanKey(%q) 应被拒绝，实际返回 %q", k, got)
		}
	}
}

func TestCleanKeyNormalizes(t *testing.T) {
	cases := map[string]string{
		"attachments/a.png":       "attachments/a.png",
		"/attachments/a.png":      "attachments/a.png",
		"attachments//a.png":      "attachments/a.png",
		"attachments/./a.png":     "attachments/a.png",
		"attachments\\sub\\a.png": "attachments/sub/a.png",
		"brand/logo.svg":          "brand/logo.svg",
	}
	for in, want := range cases {
		got, err := CleanKey(in)
		if err != nil {
			t.Errorf("CleanKey(%q) 意外失败: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("CleanKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestLocalStorageRoundTrip 本地驱动的存取删闭环
func TestLocalStorageRoundTrip(t *testing.T) {
	root := t.TempDir()
	s, err := NewLocalStorage(root)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	ctx := context.Background()
	const key = "attachments/hello.txt"
	const body = "hello ticketdesk"

	if s.Exists(ctx, key) {
		t.Fatal("尚未写入就报告存在")
	}

	if err := s.Save(ctx, key, bytes.NewBufferString(body), int64(len(body)), "text/plain"); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if !s.Exists(ctx, key) {
		t.Fatal("写入后仍报告不存在")
	}

	obj, err := s.Open(ctx, key)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	got, _ := io.ReadAll(obj.Body)
	obj.Body.Close()
	if string(got) != body {
		t.Fatalf("内容不符: got %q", string(got))
	}
	if obj.Size != int64(len(body)) {
		t.Fatalf("大小不符: got %d", obj.Size)
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if s.Exists(ctx, key) {
		t.Fatal("删除后仍存在")
	}
	// 删除不存在的对象应视为成功
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("重复删除应成功: %v", err)
	}
}

func TestLocalStorageOpenMissingReturnsNotFound(t *testing.T) {
	s, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(context.Background(), "attachments/nope.txt"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("期望 ErrNotFound，实际: %v", err)
	}
}

// TestLocalStorageBlocksEscape 端到端确认路径穿越写不出存储根
func TestLocalStorageBlocksEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "..", "escaped.txt")

	s, err := NewLocalStorage(filepath.Join(root, "uploads"))
	if err != nil {
		t.Fatal(err)
	}

	err = s.Save(context.Background(), "../../escaped.txt", bytes.NewBufferString("x"), 1, "")
	if err == nil {
		t.Fatal("路径穿越写入应被拒绝")
	}
	if _, statErr := os.Stat(outside); statErr == nil {
		t.Fatal("文件被写到了存储根之外")
	}
}

// TestFactoryDefaultsToLocal 未配置 driver 时保持老部署行为
func TestFactoryDefaultsToLocal(t *testing.T) {
	s, err := New(Config{LocalPath: t.TempDir()})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if s.Driver() != DriverLocal {
		t.Fatalf("默认驱动应为 local，实际 %q", s.Driver())
	}
}

func TestFactoryRejectsUnknownDriver(t *testing.T) {
	if _, err := New(Config{Driver: "ftp"}); err == nil {
		t.Fatal("未知驱动应报错")
	}
}

// TestFactoryS3RequiresConfig s3 驱动缺少必填项时必须在启动阶段就失败，
// 而不是等到用户上传文件时才暴露
func TestFactoryS3RequiresConfig(t *testing.T) {
	if _, err := New(Config{Driver: DriverS3}); err == nil {
		t.Fatal("缺少 endpoint 应报错")
	}
	if _, err := New(Config{Driver: DriverS3, S3: S3Config{Endpoint: "minio:9000"}}); err == nil {
		t.Fatal("缺少 bucket 应报错")
	}
}

// TestJoinKeyEnforcesPrefix 前缀边界必须是结构性保证，而不是调用方的约定
func TestJoinKeyEnforcesPrefix(t *testing.T) {
	bad := []string{
		"../attachments/evil",
		"sub/evil.png",
		"a\\b.png",
		"..",
		".",
		"",
		"   ",
	}
	for _, name := range bad {
		if got, err := JoinKey(PrefixBrand, name); err == nil {
			t.Errorf("JoinKey(brand, %q) 应被拒绝，实际得到 %q", name, got)
		}
	}

	got, err := JoinKey(PrefixBrand, "logo_123.svg")
	if err != nil {
		t.Fatalf("正常文件名不应失败: %v", err)
	}
	if got != "brand/logo_123.svg" {
		t.Fatalf("got %q", got)
	}
}
