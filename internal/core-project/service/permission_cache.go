// Package service: 项目权限的缓存层
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kerbos/ticketdesk/pkg/cache"
)

// permCacheTTL 用户项目权限集的缓存时长
//
// 每一次非管理员的受保护请求都要过 CheckUserPermission，而它原本要打 4 次库
// （项目、成员、项目角色、角色权限），叠加中间件里的角色查询共 5 次 ——
// 业务逻辑还没开始跑就已经 5 个往返。这里把结果集缓存起来。
//
// 权限变更（改成员、改角色权限、删项目角色）会主动失效，
// 该 TTL 只是兜底收敛时间。
const permCacheTTL = 5 * time.Minute

// permCacheKey 用户在某项目下的权限集缓存键
func permCacheKey(projectKey string, userID uint64) string {
	return fmt.Sprintf("perm:%s:%d", strings.ToUpper(projectKey), userID)
}

// permCacheProjectPrefix 项目维度的失效标记键
//
// Redis 上按前缀批量删除需要 SCAN，代价高且非原子。这里改用「版本号」方案：
// 每个项目维护一个单调递增的版本号，权限相关变更时递增，
// 缓存键里带上版本号，旧版本的键自然失效并随 TTL 过期。
func permCacheVersionKey(projectKey string) string {
	return fmt.Sprintf("perm:ver:%s", strings.ToUpper(projectKey))
}

// cachedPermissions 缓存中保存的权限集
type cachedPermissions struct {
	// IsMember 是否为项目成员；非成员直接判定无权限，避免缓存穿透
	IsMember bool `json:"is_member"`
	// IsOwner owner 拥有全部权限
	IsOwner bool `json:"is_owner"`
	// Permissions 合并后的权限键集合
	Permissions []string `json:"permissions"`
}

func (c *cachedPermissions) has(permission string) bool {
	if !c.IsMember {
		return false
	}
	if c.IsOwner {
		return true
	}
	for _, p := range c.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// permVersion 读取项目的权限版本号，读不到则视为 0
func permVersion(ctx context.Context, projectKey string) string {
	if v, ok := cache.Get(ctx, permCacheVersionKey(projectKey)); ok {
		return v
	}
	return "0"
}

// InvalidateProjectPermissions 递增项目权限版本号，使该项目下所有用户的权限缓存失效
//
// 成员增删、角色成员变更、角色权限修改后必须调用，
// 否则被移除的成员在 TTL 内仍能通过权限校验。
func InvalidateProjectPermissions(ctx context.Context, projectKey string) {
	key := permCacheVersionKey(projectKey)
	if _, err := cache.Incr(ctx, key); err != nil {
		// Redis 不可用时缓存本来就读不到，退化为直连数据库，安全侧无影响
		return
	}
}

// loadCachedPermissions 读缓存；未命中返回 nil
func loadCachedPermissions(ctx context.Context, projectKey string, userID uint64) *cachedPermissions {
	key := permCacheKey(projectKey, userID) + ":" + permVersion(ctx, projectKey)
	var c cachedPermissions
	if cache.GetJSON(ctx, key, &c) {
		return &c
	}
	return nil
}

// storeCachedPermissions 写缓存
func storeCachedPermissions(ctx context.Context, projectKey string, userID uint64, c *cachedPermissions) {
	key := permCacheKey(projectKey, userID) + ":" + permVersion(ctx, projectKey)
	cache.SetJSON(ctx, key, *c, permCacheTTL)
}
