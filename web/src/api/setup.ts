import request, { type ApiResponse } from '@/utils/request'

/** 初始化状态 */
export interface SetupStatus {
  initialized: boolean
}

/** 初始化请求 */
export interface SetupRequest {
  token: string
  username: string
  password: string
  display_name: string
  email: string
  system_name: string
  site_url: string
  language: string
}

/** 查询是否已完成初始化；未初始化时前端强制跳向导 */
export const getSetupStatus = () => request.get<ApiResponse<SetupStatus>>('/setup/status')

/** 提交初始化 */
export const submitSetup = (data: SetupRequest) => request.post<ApiResponse<null>>('/setup', data)
