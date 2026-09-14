package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// TestS3StorageRoundTrip 针对真实 S3 兼容服务的集成测试。
//
// 默认跳过；设置 TD_TEST_S3_ENDPOINT 后运行，例如本地起一个 MinIO：
//
//	docker run -d -p 9000:9000 -e MINIO_ROOT_USER=testkey \
//	  -e MINIO_ROOT_PASSWORD=testsecret123 minio/minio server /data
//	TD_TEST_S3_ENDPOINT=127.0.0.1:9000 TD_TEST_S3_ACCESS_KEY=testkey \
//	  TD_TEST_S3_SECRET_KEY=testsecret123 go test ./pkg/storage/ -run TestS3
func TestS3StorageRoundTrip(t *testing.T) {
	endpoint := os.Getenv("TD_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("未设置 TD_TEST_S3_ENDPOINT，跳过 S3 集成测试")
	}

	cfg := S3Config{
		Endpoint:  endpoint,
		Bucket:    "ticketdesk-test",
		AccessKey: os.Getenv("TD_TEST_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("TD_TEST_S3_SECRET_KEY"),
		UseSSL:    false,
		PathStyle: true,
		Prefix:    "it",
	}

	// 先确保 bucket 存在
	admin, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       cfg.UseSSL,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		t.Fatalf("连接 S3 失败: %v", err)
	}
	ctx := context.Background()
	exists, err := admin.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		t.Fatalf("检查 bucket 失败: %v", err)
	}
	if !exists {
		if err := admin.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			t.Fatalf("创建 bucket 失败: %v", err)
		}
	}

	s, err := NewS3Storage(cfg)
	if err != nil {
		t.Fatalf("创建 S3 驱动失败: %v", err)
	}
	if s.Driver() != DriverS3 {
		t.Fatalf("driver = %q", s.Driver())
	}

	const key = "attachments/整合测试 file.txt"
	const body = "hello object storage"

	if s.Exists(ctx, key) {
		_ = s.Delete(ctx, key)
	}

	if err := s.Save(ctx, key, bytes.NewBufferString(body), int64(len(body)), "text/plain"); err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	if !s.Exists(ctx, key) {
		t.Fatal("上传后 Exists 为 false")
	}

	obj, err := s.Open(ctx, key)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	got, _ := io.ReadAll(obj.Body)
	obj.Body.Close()
	if string(got) != body {
		t.Fatalf("内容不符: %q", string(got))
	}
	if obj.Size != int64(len(body)) {
		t.Fatalf("大小不符: %d", obj.Size)
	}
	if obj.ContentType != "text/plain" {
		t.Fatalf("Content-Type 不符: %q", obj.ContentType)
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if s.Exists(ctx, key) {
		t.Fatal("删除后仍存在")
	}
	// 删除不存在的对象应视为成功，与 local 驱动语义一致
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("重复删除应成功: %v", err)
	}
	if _, err := s.Open(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("读取已删除对象应返回 ErrNotFound，实际: %v", err)
	}
}

// TestS3StorageRejectsTraversal 对象键的穿越防护对 s3 驱动同样生效
func TestS3StorageRejectsTraversal(t *testing.T) {
	s := &S3Storage{bucket: "b", prefix: "p"}
	if _, err := s.objectKey("../../etc/passwd"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("期望 ErrInvalidKey，实际: %v", err)
	}
	got, err := s.objectKey("/attachments//a.png")
	if err != nil {
		t.Fatalf("意外失败: %v", err)
	}
	if got != "p/attachments/a.png" {
		t.Fatalf("键拼接不符: %q", got)
	}
}
