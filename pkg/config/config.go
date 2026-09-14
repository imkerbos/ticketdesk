// Package config 提供配置管理功能
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"

	"github.com/kerbos/ticketdesk/pkg/storage"
)

// Config 应用配置结构
type Config struct {
	App         AppConfig         `mapstructure:"app"`
	Database    DatabaseConfig    `mapstructure:"database"`
	Redis       RedisConfig       `mapstructure:"redis"`
	JWT         JWTConfig         `mapstructure:"jwt"`
	Log         LogConfig         `mapstructure:"log"`
	Nightingale NightingaleConfig `mapstructure:"nightingale"`
	SSO         SSOConfig         `mapstructure:"sso"`
	Storage     storage.Config    `mapstructure:"storage"`
}

// SSOConfig SSO 单点登录配置
type SSOConfig struct {
	Enabled        bool            `mapstructure:"enabled"`
	ProviderName   string          `mapstructure:"provider_name"` // 前端展示的名称，如 "企业统一认证"
	ClientID       string          `mapstructure:"client_id"`
	ClientSecret   string          `mapstructure:"client_secret"`
	IssuerURL      string          `mapstructure:"issuer_url"`
	RedirectURI    string          `mapstructure:"redirect_uri"`
	Scopes         []string        `mapstructure:"scopes"`
	AutoCreateUser bool            `mapstructure:"auto_create_user"` // 是否自动创建用户
	DefaultRole    string          `mapstructure:"default_role"`     // 自动创建用户的默认角色
	ClaimMapping   SSOClaimMapping `mapstructure:"claim_mapping"`
}

// SSOClaimMapping OIDC claims 映射配置
type SSOClaimMapping struct {
	Username    string `mapstructure:"username"`     // 映射到 username 的 claim 名称
	Email       string `mapstructure:"email"`        // 映射到 email 的 claim 名称
	DisplayName string `mapstructure:"display_name"` // 映射到 display_name 的 claim 名称
	Avatar      string `mapstructure:"avatar"`       // 映射到 avatar_url 的 claim 名称
}

// NightingaleConfig 夜莺告警系统配置
type NightingaleConfig struct {
	Enable       bool   `mapstructure:"enable"`
	BaseURL      string `mapstructure:"base_url"`
	Token        string `mapstructure:"token"`
	PollInterval int    `mapstructure:"poll_interval"` // 轮询间隔（秒），默认 30
}

// AppConfig 应用配置
type AppConfig struct {
	Name  string `mapstructure:"name"`
	Env   string `mapstructure:"env"`
	Port  int    `mapstructure:"port"`
	Debug bool   `mapstructure:"debug"`
	// TrustedProxies 可信反向代理网段（CIDR 或 IP）。
	// 只有当直连对端落在此列表内时，Gin 才会采信 X-Forwarded-For 推导客户端 IP。
	// 留空则使用 DefaultTrustedProxies（内网网段），覆盖 Docker / K8s Ingress 场景。
	// 绝不可配成 0.0.0.0/0：那等于任何人都能伪造 IP，限流与审计日志同时失效。
	TrustedProxies []string `mapstructure:"trusted_proxies"`
}

// DefaultTrustedProxies 默认可信代理网段：回环 + RFC1918 内网。
// 反向代理（nginx / Istio / K8s Ingress）通常位于这些网段内，
// 且会以追加方式写 X-Forwarded-For，因此外部伪造的 XFF 条目会被 Gin 判为不可信而跳过。
var DefaultTrustedProxies = []string{
	"127.0.0.1/32",
	"::1/128",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"fc00::/7",
}

// EffectiveTrustedProxies 返回实际生效的可信代理列表
func (c *AppConfig) EffectiveTrustedProxies() []string {
	if len(c.TrustedProxies) == 0 {
		return DefaultTrustedProxies
	}
	return c.TrustedProxies
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	Name            string `mapstructure:"name"`
	Charset         string `mapstructure:"charset"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

// DSN 返回数据库连接字符串
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.Name, c.Charset)
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// Addr 返回 Redis 地址
func (c *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret        string `mapstructure:"secret"`
	AccessExpire  int    `mapstructure:"access_expire"`
	RefreshExpire int    `mapstructure:"refresh_expire"`
	Issuer        string `mapstructure:"issuer"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	FilePath   string `mapstructure:"file_path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// 支持环境变量覆盖
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// IsDevelopment 判断是否为开发环境
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// IsProduction 判断是否为生产环境
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}
