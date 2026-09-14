import { defineStore } from 'pinia'
import { clearProjectPermissionCache } from '@/utils/project-permissions'
import { ref, computed } from 'vue'
import type { UserProfile } from '@/types/user'

export const useUserStore = defineStore('user', () => {
  // 状态
  const user = ref<UserProfile | null>(null)
  const token = ref<string>('')
  const refreshToken = ref<string>('')

  // 计算属性
  const isLoggedIn = computed(() => !!token.value && !!user.value)
  const isAdmin = computed(() => user.value?.roles?.includes('admin') || false)
  const isProjectAdmin = computed(() => user.value?.roles?.includes('admin') || user.value?.roles?.includes('project_admin') || false)
  const username = computed(() => user.value?.username || '')
  const displayName = computed(() => user.value?.display_name || user.value?.username || '')
  const avatarUrl = computed(() => user.value?.avatar_url || '')
  const roles = computed(() => user.value?.roles || [])

  // 初始化：从 localStorage 加载
  const initFromStorage = () => {
    const storedToken = localStorage.getItem('token')
    const storedRefreshToken = localStorage.getItem('refresh_token')
    const storedUser = localStorage.getItem('user')

    if (storedToken) token.value = storedToken
    if (storedRefreshToken) refreshToken.value = storedRefreshToken
    if (storedUser) {
      try {
        user.value = JSON.parse(storedUser)
      } catch {
        // ignored
      }
    }
  }

  // 登录
  const login = (accessToken: string, refreshTokenValue: string, userInfo: UserProfile) => {
    token.value = accessToken
    refreshToken.value = refreshTokenValue
    user.value = userInfo

    localStorage.setItem('token', accessToken)
    localStorage.setItem('refresh_token', refreshTokenValue)
    localStorage.setItem('user', JSON.stringify(userInfo))
  }

  // 登出
  const logout = () => {
    token.value = ''
    refreshToken.value = ''
    user.value = null

    localStorage.removeItem('token')
    localStorage.removeItem('refresh_token')
    localStorage.removeItem('user')

    // 项目权限是按人缓存的，不清掉的话换个人登录会继续用上一个人的权限
    clearProjectPermissionCache()
  }

  // 更新用户信息
  const updateUser = (userInfo: Partial<UserProfile>) => {
    if (user.value) {
      user.value = { ...user.value, ...userInfo }
      localStorage.setItem('user', JSON.stringify(user.value))
    }
  }

  // 检查是否有指定角色
  const hasRole = (role: string) => {
    return user.value?.roles?.includes(role) || false
  }

  // 检查是否有任意一个角色
  const hasAnyRole = (roleList: string[]) => {
    return roleList.some((role) => hasRole(role))
  }

  return {
    // 状态
    user,
    token,
    refreshToken,
    // 计算属性
    isLoggedIn,
    isAdmin,
    isProjectAdmin,
    username,
    displayName,
    avatarUrl,
    roles,
    // 方法
    initFromStorage,
    login,
    logout,
    updateUser,
    hasRole,
    hasAnyRole,
  }
})
