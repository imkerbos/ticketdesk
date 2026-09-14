// 当前用户在各项目里的权限缓存。
//
// 单独成一个模块而不是放在 router 里：router 要用 stores/user，
// stores/user 登出时又要清这份缓存，放一起就成了循环引用。
import { getMyProjectPermissions } from '@/api/project'
import type { MyProjectPermissions } from '@/types/project'

// 同一个项目连着开几个页面不该每次都往后端打一发，按项目 key 缓存住在飞的请求
const cache = new Map<string, Promise<MyProjectPermissions | null>>()

/** 换人登录后必须清掉：否则上一个人的权限会被下一个人用上 */
export const clearProjectPermissionCache = () => cache.clear()

export const hasProjectPermission = async (key: string, permission: string): Promise<boolean> => {
  let pending = cache.get(key)
  if (!pending) {
    pending = getMyProjectPermissions(key)
      .then((res) => res.data.data)
      .catch(() => null)
    cache.set(key, pending)
  }
  const perms = await pending

  // 问不出来时不拦 —— 后端还有一道 requirePerm，宁可让人进去吃一个明确的 403，
  // 也不要因为一次网络抖动把有权限的人挡在外面
  if (!perms) return true
  if (perms.is_admin || perms.is_owner) return true
  if (!perms.is_member) return false
  return perms.permissions?.includes(permission) ?? false
}
