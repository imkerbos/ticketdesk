import request, { type ApiResponse } from '@/utils/request'

// 用户信息
export interface User {
  id: number
  username: string
  email: string
  display_name: string
  avatar_url: string
  status: number
  roles: string[]
  mfa_enabled: boolean
  auth_source: string
  sso_provider?: string
  locale?: string // 偏好语言（zh-CN / en-US），空 = 跟随站点设置
  last_login_at?: string
  created_at: string
  updated_at: string
}

// 登录请求
export interface LoginRequest {
  username: string
  password: string
}

// 登录响应
//
// 账号启用 MFA 时，第一步只会返回 requires_mfa + mfa_token，
// 令牌与用户信息为空；需再调 verifyMFA 换取正式令牌。
export interface LoginResponse {
  access_token?: string
  refresh_token?: string
  expires_in?: number
  user?: User
  requires_mfa?: boolean
  mfa_token?: string
}

// MFA 登录验证请求（第二步）
export interface MFALoginRequest {
  mfa_token: string
  code: string
}

// 刷新 Token 请求
export interface RefreshTokenRequest {
  refresh_token: string
}

// 登录
export const login = (data: LoginRequest) => {
  return request.post<ApiResponse<LoginResponse>>('/auth/login', data)
}

// 提交 TOTP 码完成登录（第二步）
export const verifyMFALogin = (data: MFALoginRequest) => {
  return request.post<ApiResponse<LoginResponse>>('/auth/mfa/verify', data)
}

// 刷新 Token
export const refreshToken = (data: RefreshTokenRequest) => {
  return request.post<ApiResponse<LoginResponse>>('/auth/refresh', data)
}

// 获取当前用户信息
export const getCurrentUser = () => {
  return request.get<ApiResponse<User>>('/users/me')
}

// 忘记密码请求
export interface ForgotPasswordRequest {
  email: string
}

// 重置密码请求
export interface ResetPasswordWithTokenRequest {
  token: string
  new_password: string
}

// 忘记密码
export const forgotPassword = (data: ForgotPasswordRequest) => {
  return request.post<ApiResponse<{ message: string }>>('/auth/forgot-password', data)
}

// 验证重置密码令牌
export const verifyResetToken = (token: string) => {
  return request.get<ApiResponse<{ message: string }>>('/auth/verify-reset-token', {
    params: { token }
  })
}

// 使用令牌重置密码
export const resetPasswordWithToken = (data: ResetPasswordWithTokenRequest) => {
  return request.post<ApiResponse<{ message: string }>>('/auth/reset-password', data)
}

// ============ SSO 相关 ============

// SSO 配置响应
export interface SSOConfigResponse {
  enabled: boolean
  provider_name?: string
}

// SSO 授权 URL 响应
export interface SSOAuthorizeResponse {
  authorize_url: string
}

// SSO 回调请求
export interface SSOCallbackRequest {
  code: string
  state: string
}

// 获取 SSO 配置
export const getSSOConfig = () => {
  return request.get<ApiResponse<SSOConfigResponse>>('/auth/sso/config')
}

// 获取 SSO 授权 URL
export const getSSOAuthorizeURL = () => {
  return request.get<ApiResponse<SSOAuthorizeResponse>>('/auth/sso/authorize')
}

// SSO 回调
export const ssoCallback = (data: SSOCallbackRequest) => {
  return request.post<ApiResponse<LoginResponse>>('/auth/sso/callback', data)
}
