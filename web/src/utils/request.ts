import axios, { type AxiosInstance, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import { reportLoadFailure } from './load-failure'
import router from '@/router'
import { getLocale } from '@/i18n'
import { i18n } from '@/i18n'

// 拦截器在组件外执行，拿不到 useI18n 的 t，只能走全局实例
const t = i18n.global.t

// API 响应结构
export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
}

// 分页响应结构
export interface PageResponse<T = unknown> {
  items: T[]
  total: number
  page: number
  page_size: number
}

// 创建 axios 实例
const request: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Token 刷新状态管理
let isRefreshing = false
let pendingRequests: Array<(token: string) => void> = []

const processQueue = (token: string) => {
  pendingRequests.forEach(cb => cb(token))
  pendingRequests = []
}

const clearAuthAndRedirect = () => {
  localStorage.removeItem('token')
  localStorage.removeItem('refresh_token')
  localStorage.removeItem('user')
  if (window.location.pathname !== '/login') {
    ElMessage.error(t('common.sessionExpired'))
    window.location.href = '/login'
  }
}

// 请求拦截器
request.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    // 带上当前界面语言，让后端的错误信息、通知内容跟着一起切。
    // 不带的话会出现"界面英文、报错中文"的割裂。
    config.headers['Accept-Language'] = getLocale()
    // FormData 时删除默认 Content-Type，让浏览器自动设置 multipart/form-data 及 boundary
    if (config.data instanceof FormData) {
      delete config.headers['Content-Type']
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器
request.interceptors.response.use(
  (response: AxiosResponse<ApiResponse>) => {
    const { data } = response
    if (data.code !== 0) {
      ElMessage.error(data.message || t('common.requestFailed'))
      return Promise.reject(new Error(data.message))
    }
    return response
  },
  async (error) => {
    const originalRequest = error.config as InternalAxiosRequestConfig & { _retry?: boolean }
    // 支持静默错误：请求配置中设置 _silent 则不弹出错误提示
    const silent = (error.config as any)?._silent

    if (error.response) {
      const { status, data } = error.response

      // 401 且不是刷新请求本身 → 尝试用 Refresh Token 刷新
      if (status === 401 && !originalRequest._retry && window.location.pathname !== '/login') {
        const refreshTokenStr = localStorage.getItem('refresh_token')
        if (!refreshTokenStr) {
          clearAuthAndRedirect()
          return Promise.reject(error)
        }

        if (isRefreshing) {
          // 已经在刷新中，排队等待
          return new Promise((resolve) => {
            pendingRequests.push((newToken: string) => {
              originalRequest.headers.Authorization = `Bearer ${newToken}`
              resolve(request(originalRequest))
            })
          })
        }

        originalRequest._retry = true
        isRefreshing = true

        try {
          const res = await axios.post('/api/v1/auth/refresh', { refresh_token: refreshTokenStr })
          const { access_token, refresh_token: newRefreshToken } = res.data.data

          localStorage.setItem('token', access_token)
          if (newRefreshToken) {
            localStorage.setItem('refresh_token', newRefreshToken)
          }

          // 处理排队的请求
          processQueue(access_token)

          // 重试原请求
          originalRequest.headers.Authorization = `Bearer ${access_token}`
          return request(originalRequest)
        } catch {
          // Refresh Token 也过期了，跳登录
          processQueue('')
          clearAuthAndRedirect()
          return Promise.reject(error)
        } finally {
          isRefreshing = false
        }
      }

      if (!silent) {
        switch (status) {
          case 401:
            // 登录页面的 401 只显示错误信息，不跳转
            if (window.location.pathname === '/login') {
              ElMessage.error(data?.message || t('common.badCredentials'))
            }
            break
          case 403:
            // 标记了 _redirectOn403 的请求（详情页主资源加载）直接送去 403 页。
            // 不这样做的话，无权访问的项目会渲染成一个「空项目」——
            // 0 工单 0 成员，看着像这个项目本来就是空的，而不是「你没有权限」。
            if ((error.config as any)?._redirectOn403) {
              router.replace('/403')
            } else {
              // grouping：一个页面并发几个请求同时 403 时，只叠一条而不是三条一样的
              ElMessage.error({ message: data?.message || t('common.noPermission'), grouping: true })
            }
            break
          case 404:
            // 标记了 _redirectOn404 的请求（详情页主资源加载）自动跳转 404 页面
            if ((error.config as any)?._redirectOn404) {
              router.replace('/404')
            }
            break
          case 500:
            ElMessage.error({ message: data?.message || t('common.serverError'), grouping: true })
            reportLoadFailure('server')
            break
          case 503:
            // 系统尚未初始化：路由守卫已经把人送去向导页了，
            // 再弹一句"请先完成初始化向导"是站在向导页上说废话
            if (data?.code !== 'SETUP_REQUIRED') {
              ElMessage.error({ message: data?.message || t('common.requestFailed'), grouping: true })
            }
            break
          default:
            ElMessage.error({ message: data?.message || t('common.requestFailed'), grouping: true })
        }
      }
    } else if (!silent) {
      ElMessage.error({ message: t('common.networkError'), grouping: true })
      reportLoadFailure('network')
    }
    return Promise.reject(error)
  }
)

export default request
