import request from '@/utils/request'

// 获取仪表盘统计
export function getDashboardStats(params?: { project_key?: string }) {
  return request({
    url: '/reports/dashboard',
    method: 'get',
    params,
  })
}

// 获取工单统计
export function getIssueStats(params?: {
  project_key?: string
  start_date?: string
  end_date?: string
  group_by?: 'day' | 'week' | 'month'
}) {
  return request({
    url: '/reports/issues',
    method: 'get',
    params,
  })
}

// 获取 SLA 报表
export function getSLAReport(params: {
  project_key?: string
  start_date: string
  end_date: string
}) {
  return request({
    url: '/reports/sla',
    method: 'get',
    params,
  })
}

// 获取告警统计
export function getAlertStats(params?: {
  project_key?: string
  start_date?: string
  end_date?: string
  group_by?: 'day' | 'week' | 'month'
}) {
  return request({
    url: '/reports/alerts',
    method: 'get',
    params,
  })
}

// 获取用户绩效
export function getUserPerformance(params?: {
  project_key?: string
  start_date?: string
  end_date?: string
}) {
  return request({
    url: '/reports/user-performance',
    method: 'get',
    params,
  })
}

// 获取工时统计
export function getWorklogStats(params?: {
  project_key?: string
  start_date?: string
  end_date?: string
  grid_month?: string
}) {
  return request({
    url: '/reports/worklogs',
    method: 'get',
    params,
  })
}

/** 交付报表（周报 / 月报）。date 传周期内任意一天，留空取今天 */
export function getDeliveryReport(params: {
  period: 'week' | 'month'
  date?: string
  project_key?: string
}) {
  return request({
    url: '/reports/delivery',
    method: 'get',
    params,
  })
}
