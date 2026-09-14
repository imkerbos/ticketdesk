// Package storage: 驱动选择与配置
package storage

import (
	"fmt"
	"strings"
)

// 驱动名
const (
	// DriverLocal 本地文件系统；仅适用于单副本部署
	DriverLocal = "local"
	// DriverS3 S3 兼容对象存储（MinIO / AWS S3 / 阿里云 OSS / 腾讯 COS / GCS 互操作模式）
	DriverS3 = "s3"
)

// Config 存储配置
type Config struct {
	// Driver 取值 local 或 s3；留空按 local 处理，保持老部署行为不变
	Driver string `mapstructure:"driver"`
	// LocalPath local 驱动的根目录
	LocalPath string `mapstructure:"local_path"`
	// S3 s3 驱动的连接参数
	S3 S3Config `mapstructure:"s3"`
}

// S3Config S3 兼容存储配置
type S3Config struct {
	// Endpoint 形如 "s3.amazonaws.com"、"oss-cn-hangzhou.aliyuncs.com"、"minio:9000"（不带协议）
	Endpoint  string `mapstructure:"endpoint"`
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	// PathStyle 路径风格寻址（MinIO 等自建网关必须开启）
	PathStyle bool `mapstructure:"path_style"`
	// Prefix 统一对象键前缀，便于多环境共用一个 bucket
	Prefix string `mapstructure:"prefix"`
}

// New 按配置创建存储实例
func New(cfg Config) (Storage, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver == "" {
		driver = DriverLocal
	}

	switch driver {
	case DriverLocal:
		path := cfg.LocalPath
		if path == "" {
			path = "./uploads"
		}
		return NewLocalStorage(path)
	case DriverS3:
		return NewS3Storage(cfg.S3)
	default:
		return nil, fmt.Errorf("不支持的存储驱动 %q（可选：%s、%s）", cfg.Driver, DriverLocal, DriverS3)
	}
}
