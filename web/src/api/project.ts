import request, { type ApiResponse } from '@/utils/request'
import type {
  Project,
  ProjectListRequest,
  ProjectListResponse,
  CreateProjectRequest,
  UpdateProjectRequest,
  ProjectMember,
  AddProjectMemberRequest,
  ProjectIssueType,
  CreateIssueTypeRequest,
  ProjectRole,
  MyProjectPermissions,
  CreateProjectRoleRequest,
  UpdateProjectRoleRequest,
  ProjectRoleMember,
  AddRoleMemberRequest,
  UserRolesResponse,
  NotificationChannel,
  CreateNotificationChannelRequest,
  UpdateNotificationChannelRequest,
} from '@/types/project'

// 项目列表
export const getProjectList = (params?: ProjectListRequest) => {
  return request.get<ApiResponse<ProjectListResponse>>('/projects', { params })
}

// 项目详情
export const getProjectDetail = (key: string, config?: any) => {
  return request.get<ApiResponse<Project>>(`/projects/${key}`, config)
}

// 创建项目
export const createProject = (data: CreateProjectRequest) => {
  return request.post<ApiResponse<Project>>('/projects', data)
}

// 更新项目
export const updateProject = (key: string, data: UpdateProjectRequest) => {
  return request.put<ApiResponse<Project>>(`/projects/${key}`, data)
}

// 删除项目
export const deleteProject = (key: string) => {
  return request.delete(`/projects/${key}`)
}

// 项目成员列表
export const getProjectMembers = (key: string) => {
  return request.get<ApiResponse<ProjectMember[]>>(`/projects/${key}/members`)
}

// 添加项目成员
export const addProjectMember = (key: string, data: AddProjectMemberRequest) => {
  return request.post<ApiResponse<ProjectMember>>(`/projects/${key}/members`, data)
}

// 移除项目成员
export const removeProjectMember = (key: string, userId: number) => {
  return request.delete(`/projects/${key}/members/${userId}`)
}

// 更新成员角色
export const updateProjectMemberRole = (key: string, userId: number, role: string) => {
  return request.put(`/projects/${key}/members/${userId}`, { role })
}

// 所有工单类型（用于筛选器）
export const getAllIssueTypes = () => {
  return request.get<ApiResponse<ProjectIssueType[]>>('/issue-types')
}

// 项目工单类型列表
export const getProjectIssueTypes = (key: string) => {
  return request.get<ApiResponse<ProjectIssueType[]>>(`/projects/${key}/issue-types`)
}

// 创建工单类型
export const createProjectIssueType = (key: string, data: CreateIssueTypeRequest) => {
  return request.post<ApiResponse<ProjectIssueType>>(`/projects/${key}/issue-types`, data)
}

// 更新工单类型
export const updateProjectIssueType = (key: string, typeId: number, data: Partial<CreateIssueTypeRequest>) => {
  return request.put<ApiResponse<ProjectIssueType>>(`/projects/${key}/issue-types/${typeId}`, data)
}

// 删除工单类型
export const deleteProjectIssueType = (key: string, typeId: number) => {
  return request.delete(`/projects/${key}/issue-types/${typeId}`)
}

// 获取所有项目（用于选择器，不分页）
export const getAllProjects = () => {
  return request.get<ApiResponse<Project[]>>('/projects/all')
}

// ========== 项目角色管理 ==========

// 获取我在项目中的权限。
// 非成员也能调用（返回 is_member: false）—— 前端要能问出「我没有权限」，
// 不能因为没权限就连问都问不了。
export const getMyProjectPermissions = (key: string) => {
  return request.get<ApiResponse<MyProjectPermissions>>(`/projects/${key}/my-permissions`)
}

// 获取项目角色列表
export const getProjectRoles = (key: string, config?: Record<string, unknown>) => {
  return request.get<ApiResponse<ProjectRole[]>>(`/projects/${key}/roles`, config)
}

// 创建项目角色
export const createProjectRole = (key: string, data: CreateProjectRoleRequest) => {
  return request.post<ApiResponse<ProjectRole>>(`/projects/${key}/roles`, data)
}

// 更新项目角色
export const updateProjectRole = (key: string, roleId: number, data: UpdateProjectRoleRequest) => {
  return request.put<ApiResponse<ProjectRole>>(`/projects/${key}/roles/${roleId}`, data)
}

// 删除项目角色
export const deleteProjectRole = (key: string, roleId: number) => {
  return request.delete(`/projects/${key}/roles/${roleId}`)
}

// 获取角色成员列表
export const getRoleMembers = (key: string, roleId: number) => {
  return request.get<ApiResponse<ProjectRoleMember[]>>(`/projects/${key}/roles/${roleId}/members`)
}

// 添加角色成员
export const addRoleMember = (key: string, roleId: number, data: AddRoleMemberRequest) => {
  return request.post<ApiResponse<ProjectRoleMember>>(`/projects/${key}/roles/${roleId}/members`, data)
}

// 移除角色成员
export const removeRoleMember = (key: string, roleId: number, userId: number) => {
  return request.delete(`/projects/${key}/roles/${roleId}/members/${userId}`)
}

// 获取用户在项目中的角色
export const getUserRoles = (key: string, userId: number) => {
  return request.get<ApiResponse<UserRolesResponse>>(`/projects/${key}/users/${userId}/roles`)
}

// 获取角色权限
export const getRolePermissions = (key: string, roleId: number) => {
  return request.get<ApiResponse<string[]>>(`/projects/${key}/roles/${roleId}/permissions`)
}

// 设置角色权限
export const setRolePermissions = (key: string, roleId: number, data: { permissions: string[] }) => {
  return request.put<ApiResponse<any>>(`/projects/${key}/roles/${roleId}/permissions`, data)
}

// ========== 项目通知渠道管理 ==========

// 获取项目通知渠道列表
export const getNotificationChannels = (key: string) => {
  return request.get<ApiResponse<NotificationChannel[]>>(`/projects/${key}/notification-channels`)
}

// 创建通知渠道
export const createNotificationChannel = (key: string, data: CreateNotificationChannelRequest) => {
  return request.post<ApiResponse<NotificationChannel>>(`/projects/${key}/notification-channels`, data)
}

// 更新通知渠道
export const updateNotificationChannel = (key: string, channelId: number, data: UpdateNotificationChannelRequest) => {
  return request.put<ApiResponse<NotificationChannel>>(`/projects/${key}/notification-channels/${channelId}`, data)
}

// 删除通知渠道
export const deleteNotificationChannel = (key: string, channelId: number) => {
  return request.delete(`/projects/${key}/notification-channels/${channelId}`)
}

// 测试通知渠道
export const testNotificationChannel = (key: string, channelId: number) => {
  return request.post(`/projects/${key}/notification-channels/${channelId}/test`)
}

// 立即触发项目每日日报
export const runDailyDigest = (key: string) => {
  return request.post(`/projects/${key}/daily-digest/run`)
}

