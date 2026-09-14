// Package storage: S3 兼容对象存储驱动
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Storage S3 兼容对象存储
//
// 走 S3 协议而不是各家 SDK：MinIO、AWS S3、阿里云 OSS、腾讯 COS
// 以及 GCS 的互操作模式都提供 S3 兼容端点，一个驱动即可覆盖，
// 换云只需要改 endpoint 与凭证，不用改代码。
type S3Storage struct {
	client *minio.Client
	bucket string
	// prefix 所有对象键的统一前缀，便于多环境共用一个 bucket
	prefix string
}

var _ Storage = (*S3Storage)(nil)

// NewS3Storage 创建 S3 兼容存储实例
func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("storage.no_endpoint")
	}
	if cfg.Bucket == "" {
		return nil, errors.New("storage.no_bucket")
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
		// 路径风格寻址：MinIO 与多数自建网关只支持这种形式；
		// 公有云通常两种都支持，因此默认打开更通用
		BucketLookup: bucketLookup(cfg.PathStyle),
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 S3 客户端失败: %w", err)
	}

	return &S3Storage{
		client: client,
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
	}, nil
}

func bucketLookup(pathStyle bool) minio.BucketLookupType {
	if pathStyle {
		return minio.BucketLookupPath
	}
	return minio.BucketLookupDNS
}

// Driver 返回驱动名
func (s *S3Storage) Driver() string { return DriverS3 }

// objectKey 拼上统一前缀
func (s *S3Storage) objectKey(key string) (string, error) {
	clean, err := CleanKey(key)
	if err != nil {
		return "", err
	}
	if s.prefix == "" {
		return clean, nil
	}
	return s.prefix + "/" + clean, nil
}

// Save 写入对象
func (s *S3Storage) Save(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	objKey, err := s.objectKey(key)
	if err != nil {
		return err
	}

	opts := minio.PutObjectOptions{}
	if contentType != "" {
		opts.ContentType = contentType
	}
	// size 未知时交给 SDK 走分块上传
	if size < 0 {
		size = -1
	}

	if _, err := s.client.PutObject(ctx, s.bucket, objKey, r, size, opts); err != nil {
		return fmt.Errorf("上传对象失败: %w", err)
	}
	return nil
}

// Open 读取对象
func (s *S3Storage) Open(ctx context.Context, key string) (*Object, error) {
	objKey, err := s.objectKey(key)
	if err != nil {
		return nil, err
	}

	obj, err := s.client.GetObject(ctx, s.bucket, objKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取对象失败: %w", err)
	}

	// GetObject 是惰性的，真正的错误要到 Stat 才暴露
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("读取对象信息失败: %w", err)
	}

	return &Object{
		Body:        obj,
		Size:        info.Size,
		ContentType: info.ContentType,
	}, nil
}

// Delete 删除对象；不存在视为成功
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	objKey, err := s.objectKey(key)
	if err != nil {
		return err
	}
	if err := s.client.RemoveObject(ctx, s.bucket, objKey, minio.RemoveObjectOptions{}); err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("删除对象失败: %w", err)
	}
	return nil
}

// Exists 判断对象是否存在
func (s *S3Storage) Exists(ctx context.Context, key string) bool {
	objKey, err := s.objectKey(key)
	if err != nil {
		return false
	}
	_, err = s.client.StatObject(ctx, s.bucket, objKey, minio.StatObjectOptions{})
	return err == nil
}

// isNotFound 判断是否为「对象不存在」
func isNotFound(err error) bool {
	resp := minio.ToErrorResponse(err)
	return resp.StatusCode == http.StatusNotFound ||
		resp.Code == "NoSuchKey" ||
		resp.Code == "NoSuchBucket"
}
