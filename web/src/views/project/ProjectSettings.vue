<template>
  <!-- 页头和其它页一致；项目内导航（概览/看板/设置）统一成一排 tab。
       tab 内容仍用 el-tabs：七个 pane 嵌套很深，硬拆风险大于收益。 -->
  <div class="page">
    <div class="page-head">
      <div class="detail-title">
        <h1>{{ t('project.settings.title') }}</h1>
        <div class="detail-meta"><span class="key static">{{ projectKey }}</span></div>
      </div>
      <div class="grow"></div>
      <button class="btn secondary" @click="$router.push(`/projects/${projectKey}`)">{{ t('project.overview') }}</button>
      <button class="btn secondary" @click="$router.push(`/projects/${projectKey}/board`)">{{ t('project.board') }}</button>
    </div>

    <section class="card">
      <el-tabs v-model="activeTab" class="settings-tabs">
        <!-- 基本信息 -->
        <el-tab-pane name="basic">
          <template #label>
            <span class="tab-label">
              {{ t('project.settings.tabBasic') }}
            </span>
          </template>
          <div v-loading="loading" class="tab-content">
            <el-form ref="basicFormRef" :model="basicForm" :rules="basicRules" label-width="120px" class="basic-form">
              <el-form-item :label="t('project.key')">
                <el-input v-model="basicForm.project_key" disabled>
                  <template #prefix>
                    <el-icon><Key /></el-icon>
                  </template>
                </el-input>
                <div class="form-tip">
                  <el-icon><InfoFilled /></el-icon>
                  <span>{{ t('project.settings.keyImmutable') }}</span>
                </div>
              </el-form-item>
              <el-form-item :label="t('project.name')" prop="name">
                <el-input v-model="basicForm.name" :placeholder="t('project.namePlaceholder')" maxlength="50" show-word-limit>
                  <template #prefix>
                    <el-icon><Folder /></el-icon>
                  </template>
                </el-input>
              </el-form-item>
              <el-form-item :label="t('project.settings.projectDesc')">
                <el-input
                  v-model="basicForm.description"
                  type="textarea"
                  :rows="4"
                  :placeholder="t('project.descPlaceholder')"
                  maxlength="500"
                  show-word-limit
                />
              </el-form-item>
              <el-form-item :label="t('project.settings.lead')">
                <el-select v-model="basicForm.lead_user_id" :placeholder="t('project.leadPlaceholder')" filterable style="width: 100%" popper-class="user-select-popper">
                  <template #prefix>
                    <el-icon><User /></el-icon>
                  </template>
                  <el-option v-for="u in users" :key="u.id" :label="u.display_name" :value="u.id">
                    <div class="user-option">
                      <div class="user-option-avatar">{{ u.display_name?.charAt(0) || '?' }}</div>
                      <div class="user-option-info">
                        <span class="user-option-name">{{ u.display_name }}</span>
                        <span class="user-option-username">@{{ u.username }}</span>
                      </div>
                    </div>
                  </el-option>
                </el-select>
              </el-form-item>
              <el-form-item :label="t('project.settings.status')">
                <div class="status-field">
                  <!-- 开关两边各挂一个标签是老后台写法：关的那个字灰、开的那个字变蓝，
                       蓝字看着像可点的链接。开关自己已经说清开还是关，下面一行说明讲后果，
                       和系统设置里的开关保持同一种形态。 -->
                  <el-switch
                    v-model="basicForm.status"
                    :active-value="1"
                    :inactive-value="0"
                  />
                  <div class="form-tip">
                    <el-icon><InfoFilled /></el-icon>
                    <span>{{ t('project.settings.disabledTip') }}</span>
                  </div>
                </div>
              </el-form-item>
              <div class="form-actions">
                <el-button type="primary" :loading="saveLoading" size="large" @click="saveBasicInfo">
                  <el-icon><Check /></el-icon>
                  {{ t('project.settings.saveChanges') }}
                </el-button>
              </div>
            </el-form>
          </div>
        </el-tab-pane>

        <!-- 项目成员 -->
        <el-tab-pane name="members">
          <template #label>
            <span class="tab-label">
              {{ t('project.settings.tabMembers') }}
            </span>
          </template>
          <div v-loading="membersLoading" class="tab-content">
            <div class="section-header">
              <div class="section-title">
                <span>{{ t('project.settings.memberList') }}</span>
                <span class="section-count">{{ t('project.settings.memberCount', { n: members.length }) }}</span>
              </div>
              <el-button type="primary" @click="openMemberDialog">
                <el-icon><Plus /></el-icon>
                {{ t('project.settings.addMember') }}
              </el-button>
            </div>
            <el-table v-if="members.length > 0" :data="members" stripe class="members-table">
              <el-table-column :label="t('project.settings.member')" min-width="200">
                <template #default="{ row }">
                  <div class="member-cell">
                    <div class="member-avatar" :style="{ color: personColor(row.user?.display_name) }">
                      {{ row.user?.display_name?.charAt(0) || '?' }}
                    </div>
                    <div class="member-info">
                      <div class="member-name">{{ row.user?.display_name }}</div>
                      <div class="member-username">@{{ row.user?.username }}</div>
                    </div>
                  </div>
                </template>
              </el-table-column>
              <el-table-column :label="t('project.settings.role')" width="120">
                <template #default="{ row }">
                  <!-- 角色是分类维度，不是健康状态：原来 owner 映到 danger、
                       administrators 映到 warning，红色的「所有者」读起来像出了事。
                       语义色留给状态/优先级/告警级别（CLAUDE.md 3.3）。 -->
                  <el-tag type="info" size="small">
                    {{ row.role_name || row.role }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column :label="t('common.operation')" width="200" align="right">
                <template #default="{ row }">
                  <el-dropdown trigger="click" @command="(cmd: string) => handleEditMemberRole(row, cmd)">
                    <el-button size="small" type="primary" text>
                      {{ t('project.settings.changeRole') }}
                      <el-icon class="el-icon--right"><ArrowLeft style="transform: rotate(-90deg)" /></el-icon>
                    </el-button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="owner" :disabled="row.role === 'owner'">
                          <el-icon><Star /></el-icon>
                          {{ t('project.settings.owner') }}
                        </el-dropdown-item>
                        <el-dropdown-item
                          v-for="r in roles"
                          :key="r.role_key"
                          :command="r.role_key"
                          :disabled="row.role === r.role_key"
                        >
                          {{ r.role_name }}
                        </el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                  <el-button size="small" type="danger" text @click="handleRemoveMember(row)">
                    {{ t('common.remove') }}
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
            <TdEmptyState v-if="!membersLoading && members.length === 0" preset="no-data" :title="t('project.noMembers')" />
          </div>
        </el-tab-pane>

        <!-- 项目角色 -->
        <el-tab-pane name="roles">
          <template #label>
            <span class="tab-label">
              {{ t('project.settings.tabRoles') }}
            </span>
          </template>
          <div v-loading="rolesLoading" class="tab-content">
            <div class="section-header">
              <div class="section-title">
                <span>{{ t('project.settings.roleList') }}</span>
                <span class="section-count">{{ t('project.settings.countUnit', { n: roles.length }) }}</span>
              </div>
              <el-button type="primary" @click="openRoleDialog">
                <el-icon><Plus /></el-icon>
                {{ t('project.settings.createRole') }}
              </el-button>
            </div>
            <div class="roles-grid">
              <div v-for="role in roles" :key="role.id" class="role-card" :class="getRoleIconClass(role.role_key)">
                <!-- 卡片头部 -->
                <div class="role-card-top">
                  <div class="role-card-title">
                    <div class="role-name-row">
                      <span class="role-name">{{ role.role_name }}</span>
                      <el-tag v-if="role.is_system" size="small" effect="plain" round>{{ t('project.settings.roleSystem') }}</el-tag>
                    </div>
                    <div class="role-key">{{ role.role_key }}</div>
                  </div>
                </div>
                <!-- 描述 -->
                <div class="role-desc">{{ role.description || t('project.noDescription') }}</div>
                <!-- 统计标签 -->
                <div class="role-stats">
                  <div class="role-stat-item">
                    <el-icon :size="13"><User /></el-icon>
                    <span>{{ t('project.settings.roleMembers', { n: role.member_count || 0 }) }}</span>
                  </div>
                  <div class="role-stat-item">
                    <el-icon :size="13"><Lock /></el-icon>
                    <span>{{ t('project.settings.rolePerms', { n: role.permissions?.length || 0 }) }}</span>
                  </div>
                </div>
                <!-- 操作按钮 -->
                <div class="role-actions">
                  <el-button size="small" class="role-action-btn" @click="handleManageRoleMembers(role)">
                    <el-icon><User /></el-icon>
                    {{ t('project.settings.memberList') }}
                  </el-button>
                  <el-button size="small" class="role-action-btn" @click="handleConfigPermissions(role)">
                    <el-icon><Lock /></el-icon>
                    {{ t('project.settings.permissions') }}
                  </el-button>
                  <el-button v-if="!role.is_system" size="small" class="role-action-btn" @click="handleEditRole(role)">
                    <el-icon><Edit /></el-icon>
                    {{ t('common.edit') }}
                  </el-button>
                  <el-button v-if="!role.is_system" size="small" type="danger" text class="role-action-btn is-destructive" @click="handleDeleteRole(role)">
                    <el-icon><Delete /></el-icon>
                    {{ t('common.delete') }}
                  </el-button>
                </div>
              </div>
            </div>
            <TdEmptyState v-if="!rolesLoading && roles.length === 0" preset="no-data" :title="t('project.roles.empty')" />
          </div>
        </el-tab-pane>

        <!-- 工单类型 -->
        <el-tab-pane name="issue-types">
          <template #label>
            <span class="tab-label">
              {{ t('project.settings.tabIssueTypes') }}
            </span>
          </template>
          <div v-loading="issueTypesLoading" class="tab-content">
            <div class="section-header">
              <div class="section-title">
                <span>{{ t('project.settings.typeList') }}</span>
                <span class="section-count">{{ t('project.settings.countUnit', { n: issueTypes.length }) }}</span>
              </div>
              <el-button type="primary" @click="openIssueTypeDialog">
                <el-icon><Plus /></el-icon>
                {{ t('project.settings.addType') }}
              </el-button>
            </div>
            <div class="issue-types-grid">
              <div v-for="type in issueTypes" :key="type.id" class="issue-type-card">
                <div class="type-info">
                  <div class="type-name">
                    <el-icon class="type-glyph" :style="{ color: type.color || 'var(--td-color-primary)' }">
                      <component :is="getIssueTypeIcon(type.icon)" />
                    </el-icon>
                    {{ type.display_name }}
                  </div>
                  <div class="type-key">{{ type.name }}</div>
                  <div v-if="type.description" class="type-desc">{{ type.description }}</div>
                </div>
                <div class="type-actions">
                  <el-button size="small" @click="handleEditIssueTypeClick(type)">
                    <el-icon><Edit /></el-icon>
                  </el-button>
                  <el-button size="small" type="danger" text class="is-destructive" @click="handleDeleteIssueType(type)">
                    <el-icon><Delete /></el-icon>
                  </el-button>
                </div>
              </div>
            </div>
            <TdEmptyState v-if="!issueTypesLoading && issueTypes.length === 0" preset="no-data" :title="t('project.settings.noIssueType')" />
          </div>
        </el-tab-pane>

        <!-- 字段配置 -->
        <el-tab-pane name="fields">
          <template #label>
            <span class="tab-label">
              {{ t('project.settings.tabFields') }}
            </span>
          </template>
          <div class="tab-content">
            <FieldConfigTab :project-key="projectKey" />
          </div>
        </el-tab-pane>

        <!-- 通知渠道 -->
        <el-tab-pane name="notifications">
          <template #label>
            <span class="tab-label">
              {{ t('project.settings.tabChannels') }}
            </span>
          </template>
          <div class="tab-content">
            <div class="section-header">
              <div class="section-title">
                <span>{{ t('project.settings.channelTitle') }}</span>
                <span class="section-count">{{ t('project.settings.countUnit', { n: channelCount }) }}</span>
              </div>
              <el-button type="primary" @click="channelsRef?.openDialog()">
                <el-icon><Plus /></el-icon>
                {{ t('project.settings.addChannel') }}
              </el-button>
            </div>

            <!-- 内置通知提示 -->
            <div class="notification-builtin-card">
              <div class="builtin-content">
                <div class="builtin-title">{{ t('project.settings.builtinTitle') }}</div>
                <div class="builtin-desc">{{ t('project.settings.builtinDesc') }}</div>
              </div>
              <el-tag type="success" size="small" effect="plain" class="builtin-tag">{{ t('project.settings.builtinTag') }}</el-tag>
            </div>

            <!-- 每日日报配置区 -->
            <div class="digest-card">
              <div class="digest-card-header">
                <div class="digest-card-title">
                  <el-icon :size="18"><Bell /></el-icon>
                  <span>{{ t('project.settings.digestTitle') }}</span>
                  <el-tag v-if="digestForm.enabled" type="success" size="small" effect="plain" class="builtin-tag">{{ t('project.settings.digestOn') }}</el-tag>
                  <el-tag v-else type="info" size="small" effect="plain" class="builtin-tag">{{ t('project.settings.digestOff') }}</el-tag>
                </div>
                <div class="digest-card-desc">{{ t('project.settings.digestDesc') }}</div>
              </div>
              <div class="digest-card-body">
                <el-form label-position="top" :model="digestForm" class="digest-form">
                  <el-form-item :label="t('project.settings.digestEnable')">
                    <el-switch
                      v-model="digestForm.enabled"
                    />
                  </el-form-item>
                  <el-form-item :label="t('project.settings.digestTime')">
                    <el-time-picker
                      v-model="digestForm.timeValue"
                      format="HH:mm"
                      value-format="HH:mm"
                      :placeholder="t('project.settings.digestTimePlaceholder')"
                      :clearable="false"
                      style="width: 160px"
                    />
                    <span class="digest-form-tip">{{ t('project.settings.digestTimeTip') }}</span>
                  </el-form-item>
                  <el-form-item :label="t('project.settings.digestTimezone')">
                    <el-select v-model="digestForm.tz" style="width: 240px">
                      <el-option label="Asia/Shanghai (UTC+8)" value="Asia/Shanghai" />
                      <el-option label="Asia/Tokyo (UTC+9)" value="Asia/Tokyo" />
                      <el-option label="Asia/Singapore (UTC+8)" value="Asia/Singapore" />
                      <el-option label="UTC" value="UTC" />
                      <el-option label="America/Los_Angeles" value="America/Los_Angeles" />
                      <el-option label="America/New_York" value="America/New_York" />
                      <el-option label="Europe/London" value="Europe/London" />
                    </el-select>
                  </el-form-item>
                  <el-form-item :label="t('project.settings.digestScope')">
                    <el-radio-group v-model="digestForm.scope">
                      <el-radio value="all_open">{{ t('project.settings.digestScopeAll') }}</el-radio>
                      <el-radio value="assigned_only">{{ t('project.settings.digestScopeAssigned') }}</el-radio>
                    </el-radio-group>
                  </el-form-item>
                  <el-form-item :label="t('project.settings.digestTypes')">
                    <div class="digest-types-wrapper">
                      <el-checkbox-group v-model="digestForm.issueTypeIds" class="digest-types-group">
                        <el-checkbox
                          v-for="type in issueTypes"
                          :key="type.id"
                          :value="type.id"
                          class="digest-type-checkbox"
                        >
                          {{ type.display_name }}
                        </el-checkbox>
                      </el-checkbox-group>
                      <div class="digest-form-tip">
                        {{ t('project.settings.digestTypesTip') }}
                      </div>
                    </div>
                  </el-form-item>
                  <el-form-item>
                    <el-button type="primary" :loading="digestSaving" @click="saveDigestConfig">
                      <el-icon><Check /></el-icon>
                      {{ t('project.settings.digestSave') }}
                    </el-button>
                    <el-button :loading="digestRunning" :disabled="!digestForm.enabled" @click="triggerDigestNow">
                      <el-icon><Promotion /></el-icon>
                      {{ t('project.settings.digestTrigger') }}
                    </el-button>
                    <div class="digest-form-tip" style="margin-top: 8px">
                      {{ t('project.settings.digestChannelTip') }}
                    </div>
                  </el-form-item>
                </el-form>
              </div>
            </div>

            <ProjectChannelsTab ref="channelsRef" :project-key="projectKey" @update:count="channelCount = $event" />
          </div>
        </el-tab-pane>

        <!-- 工作流配置 -->
        <el-tab-pane name="workflow">
          <template #label>
            <span class="tab-label">
              {{ t('project.settings.tabWorkflow') }}
            </span>
          </template>
          <div v-loading="schemesLoading" class="tab-content">
            <div class="section-header">
              <div class="section-title">
                <span>{{ t('project.settings.schemeTitle') }}</span>
                <span class="section-count">{{ t('project.settings.countUnit', { n: schemes.length }) }}</span>
              </div>
              <el-button type="primary" @click="openSchemeDialog">
                <el-icon><Plus /></el-icon>
                {{ t('project.settings.addScheme') }}
              </el-button>
            </div>

            <div class="workflow-scheme-tip">
              <el-icon><InfoFilled /></el-icon>
              <span>{{ t('project.settings.schemeTip') }}</span>
            </div>

            <div v-if="schemes.length > 0" class="schemes-list">
              <div v-for="scheme in schemes" :key="scheme.id" class="scheme-row">
                <div class="scheme-row-left">
                  <el-icon class="scheme-type-glyph" :style="{ color: getIssueTypeColor(scheme.issue_type_id) }">
                    <Tickets />
                  </el-icon>
                  <div class="scheme-row-info">
                    <div class="scheme-row-name">
                      <span class="scheme-type-name">{{ scheme.issue_type_name || t('project.settings.typeFallback', { id: scheme.issue_type_id }) }}</span>
                      <el-icon class="scheme-arrow"><ArrowLeft style="transform: rotate(180deg)" /></el-icon>
                      <span class="scheme-workflow-name">{{ scheme.workflow_name || t('project.settings.workflowFallback', { id: scheme.workflow_id }) }}</span>
                    </div>
                    <div class="scheme-row-detail">
                      {{ t('project.settings.createdAt', { time: formatSchemeTime(scheme.created_at) }) }}
                    </div>
                  </div>
                </div>
                <div class="scheme-row-right">
                  <el-button size="small" type="danger" text class="is-destructive" @click="handleDeleteScheme(scheme)">
                    <el-icon><Delete /></el-icon>
                    {{ t('common.delete') }}
                  </el-button>
                </div>
              </div>
            </div>

            <TdEmptyState v-if="!schemesLoading && schemes.length === 0" preset="first-time" :title="t('project.settings.noSchemeTitle')" :description="t('project.settings.noSchemeDesc')">
              <el-button type="primary" @click="openSchemeDialog">
                <el-icon><Plus /></el-icon>
                {{ t('project.settings.addFirstScheme') }}
              </el-button>
            </TdEmptyState>
          </div>
        </el-tab-pane>

        <!-- 危险操作 -->
        <el-tab-pane name="danger">
          <template #label>
            <span class="tab-label danger-tab-label">
              {{ t('project.settings.tabDanger') }}
            </span>
          </template>
          <div class="tab-content danger-content">
            <div class="danger-card">
              <div class="danger-card-title">
                <el-icon><Warning /></el-icon>
                <span>{{ t('project.settings.deleteProject') }}</span>
              </div>
              <p class="danger-card-desc">
                {{ t('project.settings.deleteProjectDesc') }}
              </p>
              <ul class="danger-card-list">
                <li>{{ t('project.settings.deleteItemKey', { key: basicForm.project_key || projectKey }) }}</li>
                <li>{{ t('project.settings.deleteItemRelated') }}</li>
                <li>{{ t('project.settings.deleteItemAlerts') }}</li>
              </ul>
              <el-button type="danger" :loading="dangerDeleting" @click="handleDeleteProject">
                <el-icon><Delete /></el-icon>
                {{ t('project.settings.deleteThisProject') }}
              </el-button>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </section>

    <!-- 批量添加成员对话框 -->
    <el-dialog v-model="memberDialogVisible" :title="t('project.settings.addMember')" width="680px" destroy-on-close class="custom-dialog batch-member-dialog">
      <div class="batch-member-content">
        <!-- 搜索和角色选择 -->
        <div class="batch-member-toolbar">
          <el-input
            v-model="memberSearchKeyword"
            :placeholder="t('project.settings.memberSearchPlaceholder')"
            clearable
            class="member-search-input"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
          <el-select v-model="memberForm.role" style="width: 140px">
            <el-option
              v-for="r in roles"
              :key="r.role_key"
              :label="r.role_name"
              :value="r.role_key"
            />
          </el-select>
        </div>
        <!-- 已选提示 -->
        <div v-if="selectedUserIds.length > 0" class="batch-member-selected-hint">
          {{ t('project.settings.selectedCount') }} <strong>{{ selectedUserIds.length }}</strong> {{ t('project.settings.selectedUnit') }}
          <el-button type="primary" link size="small" @click="selectedUserIds = []">{{ t('project.settings.clear') }}</el-button>
        </div>
        <!-- 用户列表 -->
        <div class="batch-member-list">
          <el-table
            ref="memberTableRef"
            :data="filteredAvailableUsers"
            max-height="400"
            row-key="id"
            size="small"
            @selection-change="handleMemberSelectionChange"
          >
            <el-table-column type="selection" width="40" :reserve-selection="true" />
            <el-table-column :label="t('project.settings.user')" min-width="200">
              <template #default="{ row }">
                <div class="user-option">
                  <div class="user-option-avatar">{{ row.display_name?.charAt(0) || '?' }}</div>
                  <div class="user-option-info">
                    <span class="user-option-name">{{ row.display_name }}</span>
                    <span class="user-option-username">@{{ row.username }}</span>
                  </div>
                </div>
              </template>
            </el-table-column>
          </el-table>
          <TdEmptyState v-if="filteredAvailableUsers.length === 0 && memberSearchKeyword" preset="no-result" :title="t('project.settings.noMatchUser')" />
        </div>
      </div>
      <template #footer>
        <el-button @click="memberDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button
          type="primary"
          :loading="addMemberLoading"
          :disabled="selectedUserIds.length === 0"
          @click="submitAddMember"
        >
          <el-icon><Check /></el-icon>
          {{ t('project.settings.addN') }}{{ selectedUserIds.length > 0 ? ` (${selectedUserIds.length})` : '' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 创建/编辑角色对话框 -->
    <el-dialog
      v-model="roleDialogVisible"
      :title="isEditingRole ? t('project.roles.edit') : t('project.settings.createRole')"
      width="500px"
      destroy-on-close
      class="custom-dialog"
    >
      <el-form ref="roleFormRef" :model="roleForm" :rules="roleRules" label-width="80px">
        <el-form-item :label="t('project.roles.key')" prop="role_key">
          <el-input v-model="roleForm.role_key" :placeholder="t('project.settings.roleKeyPlaceholder')" :disabled="isEditingRole" />
          <div class="form-tip">{{ t('project.settings.roleKeyTip') }}</div>
        </el-form-item>
        <el-form-item :label="t('project.roles.name')" prop="role_name">
          <el-input v-model="roleForm.role_name" :placeholder="t('project.settings.roleNamePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('issue.description')">
          <el-input v-model="roleForm.description" type="textarea" :rows="3" :placeholder="t('project.roles.descPlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="roleDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="roleSubmitLoading" @click="submitRole">
          <el-icon><Check /></el-icon>
          {{ t('common.confirm') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 角色成员管理对话框 -->
    <el-dialog
      v-model="memberMgmtDialogVisible"
      :title="t('project.roles.memberTitle', { name: currentMgmtRole?.role_name })"
      width="600px"
      destroy-on-close
      class="custom-dialog"
      @close="closeMemberMgmtDialog"
    >
      <div class="member-mgmt-content">
        <div class="member-mgmt-add">
          <el-select v-model="selectedRoleMemberUserId" :placeholder="t('project.roles.selectUser')" filterable style="flex: 1" popper-class="user-select-popper">
            <el-option v-for="u in availableRoleUsers" :key="u.id" :label="u.display_name" :value="u.id">
              <div class="user-option">
                <div class="user-option-avatar">{{ u.display_name?.charAt(0) || '?' }}</div>
                <div class="user-option-info">
                  <span class="user-option-name">{{ u.display_name }}</span>
                  <span class="user-option-username">@{{ u.username }}</span>
                </div>
              </div>
            </el-option>
          </el-select>
          <el-button type="primary" :loading="addRoleMemberLoading" :disabled="!selectedRoleMemberUserId" @click="handleAddRoleMember">
            <el-icon><Plus /></el-icon>
            {{ t('project.roles.addMember') }}
          </el-button>
        </div>
        <el-divider />
        <div v-loading="roleMembersLoading" class="member-mgmt-list">
          <div v-for="member in roleMembers" :key="member.id" class="member-mgmt-item">
            <div class="member-mgmt-info">
              <div class="member-mgmt-avatar">{{ member.user?.display_name?.charAt(0) || '?' }}</div>
              <div class="member-mgmt-details">
                <span class="member-mgmt-name">{{ member.user?.display_name }}</span>
                <span class="member-mgmt-username">@{{ member.user?.username }}</span>
              </div>
            </div>
            <el-button size="small" type="danger" text @click="handleRemoveRoleMember(member)">
              <el-icon><Delete /></el-icon>
              {{ t('common.remove') }}
            </el-button>
          </div>
          <TdEmptyState v-if="!roleMembersLoading && roleMembers.length === 0" preset="no-data" :title="t('project.noMembers')" />
        </div>
      </div>
    </el-dialog>

    <!-- 角色权限配置对话框 -->
    <el-dialog
      v-model="permDialogVisible"
      :title="t('project.settings.permTitle', { name: currentPermRole?.role_name })"
      width="640px"
      destroy-on-close
      class="custom-dialog"
    >
      <div v-loading="permLoading" class="perm-config-content">
        <div v-for="group in PERMISSION_GROUPS" :key="group.module" class="perm-group">
          <div class="perm-group-header">
            <el-checkbox
              :model-value="isModuleAllChecked(group)"
              :indeterminate="isModuleIndeterminate(group)"
              @change="handleToggleModuleAll(group)"
            >
              {{ group.module }}
            </el-checkbox>
          </div>
          <div class="perm-group-items">
            <el-checkbox
              v-for="perm in group.permissions"
              :key="perm.key"
              :model-value="rolePermissions.includes(perm.key)"
              @change="(val: string | number | boolean) => handleTogglePermission(perm.key, !!val)"
            >
              {{ perm.label }}
            </el-checkbox>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="permDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="permSaveLoading" @click="handleSavePermissions">
          <el-icon><Check /></el-icon>
          {{ t('common.save') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 创建/编辑工单类型对话框 -->
    <el-dialog
      v-model="issueTypeDialogVisible"
      :title="isEditingIssueType ? t('project.settings.editIssueType') : t('project.settings.createIssueType')"
      width="560px"
      destroy-on-close
      class="custom-dialog"
    >
      <el-form ref="issueTypeFormRef" :model="issueTypeForm" :rules="issueTypeRules" label-width="80px">
        <el-form-item :label="t('project.settings.typeKey')" prop="name">
          <el-input v-model="issueTypeForm.name" :placeholder="t('project.settings.typeKeyPlaceholder')" :disabled="isEditingIssueType" />
          <div class="form-tip-small">
            <el-icon><InfoFilled /></el-icon>
            <span>{{ t('project.settings.typeKeyTip') }}</span>
          </div>
        </el-form-item>
        <el-form-item :label="t('project.settings.displayName')" prop="display_name">
          <el-input v-model="issueTypeForm.display_name" :placeholder="t('project.settings.displayNamePlaceholder')" />
          <div class="form-tip-small">{{ t('project.settings.displayNameTip') }}</div>
        </el-form-item>
        <el-form-item :label="t('project.settings.icon')">
          <div class="icon-selector">
            <div
              v-for="iconItem in availableIcons"
              :key="iconItem.name"
              class="icon-option"
              :class="{ active: issueTypeForm.icon === iconItem.name }"
              @click="issueTypeForm.icon = iconItem.name"
            >
              <el-icon :size="20">
                <component :is="iconItem.icon" />
              </el-icon>
              <span class="icon-label">{{ iconItem.label }}</span>
            </div>
          </div>
        </el-form-item>
        <el-form-item :label="t('project.settings.color')">
          <div class="color-selector">
            <el-color-picker v-model="issueTypeForm.color" />
            <div class="color-presets">
              <div
                v-for="color in ['var(--td-color-primary)', 'var(--td-color-success)', 'var(--td-color-warning)', 'var(--td-color-danger)', 'var(--td-cat-2)', 'var(--td-cat-5)', 'var(--td-text-secondary)', 'var(--td-cat-6)']"
                :key="color"
                class="color-preset"
                :style="{ background: color }"
                :class="{ active: issueTypeForm.color === color }"
                @click="issueTypeForm.color = color"
              ></div>
            </div>
          </div>
        </el-form-item>
        <el-form-item :label="t('issue.description')">
          <el-input v-model="issueTypeForm.description" type="textarea" :rows="3" :placeholder="t('project.settings.typeDescPlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="issueTypeDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="issueTypeSubmitLoading" @click="submitIssueType">
          <el-icon><Check /></el-icon>
          {{ t('common.confirm') }}
        </el-button>
      </template>
    </el-dialog>


    <!-- 添加工作流方案对话框 -->
    <el-dialog v-model="schemeDialogVisible" :title="t('project.settings.addSchemeTitle')" width="520px" destroy-on-close class="custom-dialog">
      <el-form ref="schemeFormRef" :model="schemeForm" :rules="schemeRules" label-width="100px">
        <el-form-item :label="t('alert.rules.issueType')" prop="issue_type_id">
          <el-select v-model="schemeForm.issue_type_id" :placeholder="t('project.fieldConfig.selectIssueType')" style="width: 100%">
            <el-option
              v-for="type in availableIssueTypes"
              :key="type.id"
              :label="type.display_name"
              :value="type.id"
            >
              <div class="scheme-option">
                <el-icon class="scheme-option-glyph" :size="14" :style="{ color: type.color || 'var(--td-color-primary)' }">
                  <component :is="getIssueTypeIcon(type.icon)" />
                </el-icon>
                <span>{{ type.display_name }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item :label="t('nav.workflows')" prop="workflow_id">
          <el-select v-model="schemeForm.workflow_id" :placeholder="t('project.settings.selectWorkflow')" style="width: 100%" filterable>
            <el-option
              v-for="w in allWorkflows"
              :key="w.id"
              :label="w.name"
              :value="w.id"
            >
              <div class="scheme-option">
                <el-icon class="scheme-option-glyph" :size="14"><Connection /></el-icon>
                <div class="scheme-option-info">
                  <span class="scheme-option-name">{{ w.name }}</span>
                  <span v-if="w.description" class="scheme-option-desc">{{ w.description }}</span>
                </div>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="schemeDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="schemeSubmitLoading" @click="submitScheme">
          <el-icon><Check /></el-icon>
          {{ t('common.confirm') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { personColor } from '@/utils/avatar'
import { useI18n } from 'vue-i18n'
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  Plus,
  User,
  ArrowLeft,
  Document,
  InfoFilled,
  Key,
  Folder,
  Check,
  Star,
  Delete,
  Edit,
  Tickets,
  Lightning,
  CircleCheck,
  Warning,
  Flag,
  Promotion,
  Tools,
  Connection,
  Memo,
  Bell,
  Search,
  Lock } from '@element-plus/icons-vue'
import FieldConfigTab from './components/FieldConfigTab.vue'
import {
  getProjectDetail,
  deleteProject,
  updateProject,
  getProjectMembers,
  addProjectMember,
  updateProjectMemberRole,
  removeProjectMember,
  getProjectRoles,
  createProjectRole,
  updateProjectRole,
  deleteProjectRole,
  getProjectIssueTypes,
  createProjectIssueType,
  updateProjectIssueType,
  deleteProjectIssueType,
  runDailyDigest,
  getRoleMembers,
  addRoleMember,
  removeRoleMember,
  getRolePermissions,
  setRolePermissions } from '@/api/project'
import { getWorkflowSchemes, createWorkflowScheme, deleteWorkflowScheme, getWorkflowList } from '@/api/workflow'
import type { WorkflowScheme, Workflow } from '@/types/workflow'
import { getAllUsers } from '@/api/user'
import type {
  ProjectMember,
  ProjectRole,
  ProjectIssueType,
  UpdateProjectRequest,
  CreateProjectRoleRequest,
  CreateIssueTypeRequest,
  ProjectRoleMember } from '@/types/project'
import type { UserOption } from '@/types/user'
import ProjectChannelsTab from './components/ProjectChannelsTab.vue'

// 渠道数据在子组件里，tab 顶部的区块头要用到计数和「新增」入口
const channelsRef = ref<InstanceType<typeof ProjectChannelsTab>>()
const channelCount = ref(0)

const { t } = useI18n()

const route = useRoute()
const router = useRouter()
const projectKey = computed(() => route.params.key as string)

// 从 URL 查询参数中获取 tab，如果没有则默认为 'basic'
// URL 里的 tab 必须校验：写错一个值（旧书签、tab 改过名）时 el-tabs 找不到
// 对应的 pane，整页就是空白，连 tab 都不高亮。认不出来就退回第一个。
const SETTINGS_TABS = ['basic', 'members', 'roles', 'issue-types', 'fields', 'notifications', 'workflow', 'danger']
const initTab = route.query.tab as string
const activeTab = ref(SETTINGS_TABS.includes(initTab) ? initTab : 'basic')
const loading = ref(false)
const saveLoading = ref(false)
const dangerDeleting = ref(false)
const users = ref<UserOption[]>([])

// 监听 tab 变化，更新 URL
watch(activeTab, (newTab) => {
  router.replace({
    query: { ...route.query, tab: newTab } })
})

// 基本信息
const basicFormRef = ref<FormInstance>()
const basicForm = reactive<UpdateProjectRequest & { project_key?: string }>({
  project_key: '',
  name: '',
  description: '',
  lead_user_id: undefined,
  status: 1 })
const basicRules: FormRules = {
  name: [{ required: true, message: t('project.nameRequired'), trigger: ['blur', 'change'] }] }

// 项目成员
const membersLoading = ref(false)
const members = ref<ProjectMember[]>([])
const memberDialogVisible = ref(false)
const addMemberLoading = ref(false)
const memberTableRef = ref()
const memberSearchKeyword = ref('')
const selectedUserIds = ref<number[]>([])
const memberForm = reactive({
  role: 'developers' })

const availableUsers = computed(() => {
  const memberIds = members.value.map((m) => m.user_id)
  return users.value.filter((u) => !memberIds.includes(u.id))
})

const filteredAvailableUsers = computed(() => {
  const keyword = memberSearchKeyword.value.trim().toLowerCase()
  if (!keyword) return availableUsers.value
  return availableUsers.value.filter(
    (u) =>
      u.display_name?.toLowerCase().includes(keyword) ||
      u.username?.toLowerCase().includes(keyword),
  )
})

const handleMemberSelectionChange = (rows: UserOption[]) => {
  selectedUserIds.value = rows.map((r) => r.id)
}

// 项目角色
const rolesLoading = ref(false)
const roles = ref<ProjectRole[]>([])
const roleDialogVisible = ref(false)
const isEditingRole = ref(false)
const editingRoleId = ref<number | null>(null)
const roleSubmitLoading = ref(false)
const roleFormRef = ref<FormInstance>()
const roleForm = reactive<CreateProjectRoleRequest>({
  role_key: '',
  role_name: '',
  description: '' })
const roleRules: FormRules = {
  role_key: [
    { required: true, message: t('project.roles.keyRequired'), trigger: ['blur', 'change'] },
    { pattern: /^[a-z_]+$/, message: t('project.roles.keyPattern'), trigger: 'blur' },
  ],
  role_name: [{ required: true, message: t('project.roles.nameRequired'), trigger: ['blur', 'change'] }] }

// 角色成员管理
const memberMgmtDialogVisible = ref(false)
const currentMgmtRole = ref<ProjectRole | null>(null)
const roleMembers = ref<ProjectRoleMember[]>([])
const roleMembersLoading = ref(false)
const selectedRoleMemberUserId = ref<number | null>(null)
const addRoleMemberLoading = ref(false)

const availableRoleUsers = computed(() => {
  const memberIds = roleMembers.value.map((m) => m.user_id)
  return users.value.filter((u) => !memberIds.includes(u.id))
})

// 角色权限配置
const PERMISSION_GROUPS = [
  { module: t('project.settings.permModuleProject'), permissions: [{ key: 'project:view', label: t('project.settings.permProjectView') }, { key: 'project:manage', label: t('project.settings.permProjectManage') }] },
  { module: t('project.settings.permModuleIssue'), permissions: [{ key: 'issue:view', label: t('project.settings.permIssueView') }, { key: 'issue:create', label: t('project.settings.permIssueCreate') }, { key: 'issue:edit', label: t('project.settings.permIssueEdit') }, { key: 'issue:delete', label: t('project.settings.permIssueDelete') }, { key: 'issue:assign', label: t('project.settings.permIssueAssign') }] },
  { module: t('project.settings.permModuleMember'), permissions: [{ key: 'member:view', label: t('project.settings.permMemberView') }, { key: 'member:manage', label: t('project.settings.permMemberManage') }] },
  { module: t('project.settings.permModuleRole'), permissions: [{ key: 'role:view', label: t('project.settings.permRoleView') }, { key: 'role:manage', label: t('project.settings.permRoleManage') }] },
  { module: t('project.settings.permModuleWorkflow'), permissions: [{ key: 'workflow:view', label: t('project.settings.permWorkflowView') }, { key: 'workflow:manage', label: t('project.settings.permWorkflowManage') }] },
  { module: t('project.settings.permModuleAlert'), permissions: [{ key: 'alert:view', label: t('project.settings.permAlertView') }, { key: 'alert:manage', label: t('project.settings.permAlertManage') }] },
]
const permDialogVisible = ref(false)
const currentPermRole = ref<ProjectRole | null>(null)
const rolePermissions = ref<string[]>([])
const permLoading = ref(false)
const permSaveLoading = ref(false)

const handleConfigPermissions = async (role: ProjectRole) => {
  currentPermRole.value = role
  permDialogVisible.value = true
  permLoading.value = true
  try {
    const { data } = await getRolePermissions(projectKey.value, role.id)
    rolePermissions.value = data.data || []
  } catch {
    rolePermissions.value = role.permissions || []
  } finally {
    permLoading.value = false
  }
}

const handleTogglePermission = (key: string, checked: boolean) => {
  if (checked) {
    if (!rolePermissions.value.includes(key)) {
      rolePermissions.value = [...rolePermissions.value, key]
    }
  } else {
    rolePermissions.value = rolePermissions.value.filter((k) => k !== key)
  }
}

const isModuleAllChecked = (group: typeof PERMISSION_GROUPS[number]) => {
  return group.permissions.every((p) => rolePermissions.value.includes(p.key))
}

const isModuleIndeterminate = (group: typeof PERMISSION_GROUPS[number]) => {
  const checked = group.permissions.filter((p) => rolePermissions.value.includes(p.key)).length
  return checked > 0 && checked < group.permissions.length
}

const handleToggleModuleAll = (group: typeof PERMISSION_GROUPS[number]) => {
  const allChecked = isModuleAllChecked(group)
  const keys = group.permissions.map((p) => p.key)
  if (allChecked) {
    rolePermissions.value = rolePermissions.value.filter((k) => !keys.includes(k))
  } else {
    const newPerms = [...rolePermissions.value]
    for (const key of keys) {
      if (!newPerms.includes(key)) {
        newPerms.push(key)
      }
    }
    rolePermissions.value = newPerms
  }
}

const handleSavePermissions = async () => {
  if (!currentPermRole.value) return
  permSaveLoading.value = true
  try {
    await setRolePermissions(projectKey.value, currentPermRole.value.id, { permissions: rolePermissions.value })
    ElMessage.success(t('project.settings.permSaved'))
    permDialogVisible.value = false
    loadRoles()
  } catch {
    ElMessage.error(t('project.settings.permSaveFailed'))
  } finally {
    permSaveLoading.value = false
  }
}

// 工单类型
const issueTypesLoading = ref(false)
const issueTypes = ref<ProjectIssueType[]>([])
const issueTypeDialogVisible = ref(false)
const isEditingIssueType = ref(false)
const editingIssueTypeId = ref<number | null>(null)
const issueTypeSubmitLoading = ref(false)
const issueTypeFormRef = ref<FormInstance>()
const issueTypeForm = reactive<CreateIssueTypeRequest>({
  name: '',
  display_name: '',
  description: '',
  icon: 'task',
  color: 'var(--td-color-primary)' })
const issueTypeRules: FormRules = {
  name: [
    { required: true, message: t('project.settings.typeKeyRequired'), trigger: ['blur', 'change'] },
    { pattern: /^[a-z]+$/, message: t('project.settings.typeKeyPattern'), trigger: 'blur' },
  ],
  display_name: [{ required: true, message: t('project.settings.displayNameRequired'), trigger: ['blur', 'change'] }] }

// 通知渠道

// 定时日报配置
const digestForm = reactive({
  enabled: false,
  timeValue: '09:00',
  cron: '0 9 * * *',
  tz: 'Asia/Shanghai',
  scope: 'all_open' as 'all_open' | 'assigned_only',
  issueTypeIds: [] as number[] })
const digestSaving = ref(false)
const digestRunning = ref(false)

const cronToTime = (cron: string): string => {
  const parts = cron.trim().split(/\s+/)
  if (parts.length < 2) return '09:00'
  const mm = String(parts[0]).padStart(2, '0')
  const hh = String(parts[1]).padStart(2, '0')
  return `${hh}:${mm}`
}
const timeToCron = (t: string): string => {
  const [hh, mm] = t.split(':').map((s) => parseInt(s, 10))
  return `${isNaN(mm) ? 0 : mm} ${isNaN(hh) ? 9 : hh} * * *`
}

const saveDigestConfig = async () => {
  digestSaving.value = true
  try {
    await updateProject(projectKey.value, {
      daily_digest_enabled: digestForm.enabled,
      daily_digest_cron: timeToCron(digestForm.timeValue),
      daily_digest_tz: digestForm.tz,
      daily_digest_scope: digestForm.scope,
      daily_digest_issue_type_ids: digestForm.issueTypeIds })
    ElMessage.success(t('project.settings.digestSaved'))
    loadProjectDetail()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || t('project.fieldConfig.saveFailed'))
  } finally {
    digestSaving.value = false
  }
}

const triggerDigestNow = async () => {
  digestRunning.value = true
  try {
    await runDailyDigest(projectKey.value)
    ElMessage.success(t('project.settings.digestTriggered'))
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || t('project.settings.digestTriggerFailed'))
  } finally {
    digestRunning.value = false
  }
}

const loadProjectDetail = async () => {
  loading.value = true
  try {
    const { data } = await getProjectDetail(projectKey.value, { _redirectOn404: true, _redirectOn403: true })
    const project = data.data
    Object.assign(basicForm, {
      project_key: project.project_key,
      name: project.name,
      description: project.description,
      lead_user_id: project.lead_user_id,
      status: project.status })
    // 回填日报配置
    digestForm.enabled = project.daily_digest_enabled === true
    digestForm.cron = project.daily_digest_cron || '0 9 * * *'
    digestForm.timeValue = cronToTime(digestForm.cron)
    digestForm.tz = project.daily_digest_tz || 'Asia/Shanghai'
    digestForm.scope = (project.daily_digest_scope as 'all_open' | 'assigned_only') || 'all_open'
    digestForm.issueTypeIds = Array.isArray(project.daily_digest_issue_type_ids)
      ? [...project.daily_digest_issue_type_ids]
      : []
  } catch {
    // ignored
  } finally {
    loading.value = false
  }
}

const loadUsers = async () => {
  try {
    const { data } = await getAllUsers()
    users.value = data.data
  } catch {
    // ignored
  }
}

const loadMembers = async () => {
  membersLoading.value = true
  try {
    const { data } = await getProjectMembers(projectKey.value)
    members.value = data.data
  } catch {
    // ignored
  } finally {
    membersLoading.value = false
  }
}

const loadRoles = async () => {
  rolesLoading.value = true
  try {
    const { data } = await getProjectRoles(projectKey.value)
    roles.value = data.data
  } catch {
    // ignored
  } finally {
    rolesLoading.value = false
  }
}

const loadIssueTypes = async () => {
  issueTypesLoading.value = true
  try {
    const { data } = await getProjectIssueTypes(projectKey.value)
    issueTypes.value = data.data
  } catch {
    // ignored
  } finally {
    issueTypesLoading.value = false
  }
}








// 工作流方案
const schemesLoading = ref(false)
const schemes = ref<WorkflowScheme[]>([])
const allWorkflows = ref<Workflow[]>([])
const schemeDialogVisible = ref(false)
const schemeSubmitLoading = ref(false)
const schemeFormRef = ref<FormInstance>()
const schemeForm = reactive({
  issue_type_id: undefined as number | undefined,
  workflow_id: undefined as number | undefined })
const schemeRules: FormRules = {
  issue_type_id: [{ required: true, message: t('project.settings.issueTypeRequired'), trigger: 'change' }],
  workflow_id: [{ required: true, message: t('project.settings.workflowRequired'), trigger: 'change' }] }

// 已绑定的工单类型不再显示
const availableIssueTypes = computed(() => {
  const boundTypeIds = schemes.value.map((s) => s.issue_type_id)
  return issueTypes.value.filter((t) => !boundTypeIds.includes(t.id))
})

const loadSchemes = async () => {
  schemesLoading.value = true
  try {
    const { data } = await getWorkflowSchemes(projectKey.value)
    schemes.value = (data as any).data || []
  } catch {
    // ignored
  } finally {
    schemesLoading.value = false
  }
}

const loadAllWorkflows = async () => {
  try {
    const { data } = await getWorkflowList()
    allWorkflows.value = (data as any).data?.items || []
  } catch {
    // ignored
  }
}

const openSchemeDialog = () => {
  schemeForm.issue_type_id = undefined
  schemeForm.workflow_id = undefined
  // 确保工作流列表已加载
  if (allWorkflows.value.length === 0) {
    loadAllWorkflows()
  }
  schemeDialogVisible.value = true
}

const submitScheme = async () => {
  if (!schemeFormRef.value) return
  await schemeFormRef.value.validate(async (valid) => {
    if (!valid) return
    schemeSubmitLoading.value = true
    try {
      await createWorkflowScheme(projectKey.value, {
        issue_type_id: schemeForm.issue_type_id!,
        workflow_id: schemeForm.workflow_id! })
      ElMessage.success(t('project.settings.schemeAdded'))
      schemeDialogVisible.value = false
      loadSchemes()
    } catch {
      // ignored
    } finally {
      schemeSubmitLoading.value = false
    }
  })
}

const handleDeleteScheme = async (scheme: WorkflowScheme) => {
  const typeName = scheme.issue_type_name || t('project.settings.typeFallback', { id: scheme.issue_type_id })
  try {
    await ElMessageBox.confirm(
      t('project.settings.confirmDeleteScheme', { name: typeName }),
      t('issue.list.deleteTitle'),
      { type: 'warning' }
    )
    await deleteWorkflowScheme(projectKey.value, scheme.issue_type_id)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    loadSchemes()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('issue.msg.deleteFailed2'))
    }
  }
}

const getIssueTypeColor = (typeId: number) => {
  const t = issueTypes.value.find((it) => it.id === typeId)
  return t?.color || 'var(--td-color-primary)'
}

const formatSchemeTime = (time: string) => {
  if (!time) return '-'
  const d = new Date(time)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

const saveBasicInfo = async () => {
  if (!basicFormRef.value) return
  await basicFormRef.value.validate(async (valid) => {
    if (!valid) return
    saveLoading.value = true
    try {
      await updateProject(projectKey.value, {
        name: basicForm.name,
        description: basicForm.description,
        lead_user_id: basicForm.lead_user_id,
        status: basicForm.status })
      ElMessage.success(t('common.saveSuccess'))
    } catch {
      // ignored
    } finally {
      saveLoading.value = false
    }
  })
}

const handleDeleteProject = async () => {
  const expectedKey = String(basicForm.project_key || projectKey.value || '').trim().toUpperCase()
  try {
    const promptResult = await ElMessageBox.prompt(
      t('project.settings.deleteConfirmMsg', { key: expectedKey }),
      t('project.settings.deleteProject'),
      {
        type: 'error',
        confirmButtonText: t('project.settings.deleteConfirmBtn'),
        cancelButtonText: t('common.cancel'),
        inputPlaceholder: expectedKey,
        inputValidator: (input) => input.trim().toUpperCase() === expectedKey,
        inputErrorMessage: t('project.settings.deleteKeyError', { key: expectedKey }) },
    )
    const inputValue = typeof promptResult === 'string' ? '' : promptResult.value
    if (inputValue.trim().toUpperCase() !== expectedKey) return

    dangerDeleting.value = true
    await deleteProject(projectKey.value)
    ElMessage.success(t('project.settings.deleteSuccess'))
    router.push('/projects')
  } catch (error: any) {
    if (error !== 'cancel' && error !== 'close') {
      ElMessage.error(error?.response?.data?.message || t('project.settings.deleteFailed'))
    }
  } finally {
    dangerDeleting.value = false
  }
}

const submitAddMember = async () => {
  if (selectedUserIds.value.length === 0) return
  addMemberLoading.value = true
  try {
    let successCount = 0
    let failCount = 0
    for (const userId of selectedUserIds.value) {
      try {
        await addProjectMember(projectKey.value, {
          user_id: userId,
          role: memberForm.role })
        successCount++
      } catch {
        failCount++
      }
    }
    if (successCount > 0) {
      ElMessage.success(failCount > 0 ? t('project.settings.membersAddedPartial', { n: successCount, f: failCount }) : t('project.settings.membersAdded', { n: successCount }))
    } else {
      ElMessage.error(t('project.fieldConfig.addFailed'))
    }
    memberDialogVisible.value = false
    loadMembers()
  } finally {
    addMemberLoading.value = false
  }
}

const openMemberDialog = () => {
  memberForm.role = roles.value.length > 0 ? roles.value[roles.value.length - 1].role_key : 'viewers'
  memberSearchKeyword.value = ''
  selectedUserIds.value = []
  memberDialogVisible.value = true
}

const handleEditMemberRole = async (member: ProjectMember, newRole: string) => {
  try {
    await updateProjectMemberRole(projectKey.value, member.user_id, newRole)
    ElMessage.success(t('project.settings.roleUpdated'))
    loadMembers()
  } catch {
    ElMessage.error(t('project.settings.roleUpdateFailed'))
  }
}

const handleRemoveMember = async (member: ProjectMember) => {
  try {
    await ElMessageBox.confirm(t('project.roles.confirmRemoveMember', { name: member.user?.display_name }), t('project.roles.removeTitle'), {
      type: 'warning' })
    await removeProjectMember(projectKey.value, member.user_id)
    ElMessage.success(t('issue.msg.removeSuccess'))
    loadMembers()
  } catch (error) {
    if (error !== 'cancel') {
      // ignored
    }
  }
}

const openRoleDialog = () => {
  isEditingRole.value = false
  editingRoleId.value = null
  Object.assign(roleForm, {
    role_key: '',
    role_name: '',
    description: '' })
  roleDialogVisible.value = true
}

const handleEditRole = (role: ProjectRole) => {
  isEditingRole.value = true
  editingRoleId.value = role.id
  Object.assign(roleForm, {
    role_key: role.role_key,
    role_name: role.role_name,
    description: role.description })
  roleDialogVisible.value = true
}

const handleDeleteRole = async (role: ProjectRole) => {
  try {
    await ElMessageBox.confirm(t('project.roles.confirmDeleteRole', { name: role.role_name }), t('issue.list.deleteTitle'), {
      type: 'warning' })
    await deleteProjectRole(projectKey.value, role.id)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    loadRoles()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('issue.msg.deleteFailed2'))
    }
  }
}

const handleManageRoleMembers = async (role: ProjectRole) => {
  currentMgmtRole.value = role
  selectedRoleMemberUserId.value = null
  memberMgmtDialogVisible.value = true
  router.replace({ query: { ...route.query, roleId: String(role.id) } })
  await loadRoleMembers(role.id)
}

const loadRoleMembers = async (roleId: number) => {
  roleMembersLoading.value = true
  try {
    const { data } = await getRoleMembers(projectKey.value, roleId)
    roleMembers.value = data.data
  } catch {
    // ignored
  } finally {
    roleMembersLoading.value = false
  }
}

const handleAddRoleMember = async () => {
  if (!selectedRoleMemberUserId.value || !currentMgmtRole.value) return
  addRoleMemberLoading.value = true
  try {
    await addRoleMember(projectKey.value, currentMgmtRole.value.id, { user_id: selectedRoleMemberUserId.value })
    ElMessage.success(t('project.roles.addSuccess'))
    selectedRoleMemberUserId.value = null
    await loadRoleMembers(currentMgmtRole.value.id)
  } catch {
    ElMessage.error(t('project.fieldConfig.addFailed'))
  } finally {
    addRoleMemberLoading.value = false
  }
}

const handleRemoveRoleMember = async (member: ProjectRoleMember) => {
  if (!currentMgmtRole.value) return
  try {
    await ElMessageBox.confirm(t('project.roles.confirmRemoveMember', { name: member.user?.display_name }), t('project.roles.removeTitle'), {
      type: 'warning' })
    await removeRoleMember(projectKey.value, currentMgmtRole.value.id, member.user_id)
    ElMessage.success(t('issue.msg.removeSuccess'))
    await loadRoleMembers(currentMgmtRole.value.id)
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('project.fieldConfig.removeFailed'))
    }
  }
}

const closeMemberMgmtDialog = () => {
  memberMgmtDialogVisible.value = false
  currentMgmtRole.value = null
  roleMembers.value = []
  const { roleId: _roleId, ...rest } = route.query
  router.replace({ query: rest })
}

const submitRole = async () => {
  if (!roleFormRef.value) return
  await roleFormRef.value.validate(async (valid) => {
    if (!valid) return
    roleSubmitLoading.value = true
    try {
      if (isEditingRole.value && editingRoleId.value) {
        await updateProjectRole(projectKey.value, editingRoleId.value, {
          role_name: roleForm.role_name,
          description: roleForm.description })
        ElMessage.success(t('issue.msg.updateSuccess'))
      } else {
        await createProjectRole(projectKey.value, roleForm)
        ElMessage.success(t('common.createSuccess'))
      }
      roleDialogVisible.value = false
      loadRoles()
    } catch {
      ElMessage.error(isEditingRole.value ? t('project.settings.updateFailed') : t('project.settings.createFailed'))
    } finally {
      roleSubmitLoading.value = false
    }
  })
}

const openIssueTypeDialog = () => {
  isEditingIssueType.value = false
  editingIssueTypeId.value = null
  Object.assign(issueTypeForm, {
    name: '',
    display_name: '',
    description: '',
    icon: 'task',
    color: 'var(--td-color-primary)' })
  issueTypeDialogVisible.value = true
}

const handleEditIssueTypeClick = (issueType: ProjectIssueType) => {
  isEditingIssueType.value = true
  editingIssueTypeId.value = issueType.id
  Object.assign(issueTypeForm, {
    name: issueType.name,
    display_name: issueType.display_name,
    description: issueType.description,
    icon: issueType.icon,
    color: issueType.color })
  issueTypeDialogVisible.value = true
}

const submitIssueType = async () => {
  if (!issueTypeFormRef.value) return
  await issueTypeFormRef.value.validate(async (valid) => {
    if (!valid) return
    issueTypeSubmitLoading.value = true
    try {
      if (isEditingIssueType.value && editingIssueTypeId.value) {
        await updateProjectIssueType(projectKey.value, editingIssueTypeId.value, {
          display_name: issueTypeForm.display_name,
          description: issueTypeForm.description,
          icon: issueTypeForm.icon,
          color: issueTypeForm.color })
        ElMessage.success(t('issue.msg.updateSuccess'))
      } else {
        await createProjectIssueType(projectKey.value, issueTypeForm)
        ElMessage.success(t('common.createSuccess'))
      }
      issueTypeDialogVisible.value = false
      loadIssueTypes()
    } catch {
      ElMessage.error(isEditingIssueType.value ? t('project.settings.updateFailed') : t('project.settings.createFailed'))
    } finally {
      issueTypeSubmitLoading.value = false
    }
  })
}

const handleDeleteIssueType = async (issueType: ProjectIssueType) => {
  try {
    await ElMessageBox.confirm(t('project.settings.confirmDeleteIssueType', { name: issueType.display_name }), t('issue.list.deleteTitle'), {
      type: 'warning' })
    await deleteProjectIssueType(projectKey.value, issueType.id)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    loadIssueTypes()
  } catch (error) {
    if (error !== 'cancel') {
      // ignored
    }
  }
}

const getRoleIconClass = (roleKey: string) => {
  const classMap: Record<string, string> = {
    administrators: 'admin',
    developers: 'dev',
    testers: 'test',
    viewers: 'view' }
  return classMap[roleKey] || 'default'
}

// 工单类型图标映射
const issueTypeIconMap: Record<string, any> = {
  epic: Lightning,
  story: Document,
  task: CircleCheck,
  bug: Warning,
  subtask: Memo,
  improvement: Tools,
  feature: Star,
  change: Connection,
  fault: Flag,
  alert: Promotion }

const getIssueTypeIcon = (iconName: string) => {
  return issueTypeIconMap[iconName] || Tickets
}

// 可选的图标列表
const availableIcons = [
  { name: 'epic', label: 'Epic', icon: Lightning },
  { name: 'story', label: 'Story', icon: Document },
  { name: 'task', label: 'Task', icon: CircleCheck },
  { name: 'bug', label: 'Bug', icon: Warning },
  { name: 'subtask', label: 'Subtask', icon: Memo },
  { name: 'improvement', label: 'Improvement', icon: Tools },
  { name: 'feature', label: 'Feature', icon: Star },
  { name: 'change', label: 'Change', icon: Connection },
  { name: 'fault', label: 'Fault', icon: Flag },
  { name: 'alert', label: 'Alert', icon: Promotion },
]

const tryRestoreRoleMemberDialog = () => {
  const roleId = route.query.roleId
  if (roleId && activeTab.value === 'roles') {
    const id = Number(roleId)
    const role = roles.value.find((r) => r.id === id)
    if (role) {
      handleManageRoleMembers(role)
    }
  }
}

// 监听 tab 切换到 roles 时，检查是否需要恢复对话框
watch(activeTab, (newTab) => {
  if (newTab === 'roles') {
    tryRestoreRoleMemberDialog()
  }
})

onMounted(async () => {
  loadProjectDetail()
  loadUsers()
  loadMembers()
  await loadRoles()
  loadIssueTypes()
  loadSchemes()
  loadAllWorkflows()
  tryRestoreRoleMemberDialog()
})
</script>

<style scoped lang="scss">
@use './project-settings-shared.scss' as *;

/* 外壳（.page / .page-head / .card）已经是 .ap 体系，这里只处理 tab 内部。
   模板结构不动：八个 pane 嵌套很深，硬拆风险大于收益。 */

.settings-tabs {
  :deep(.el-tabs__header) {
    padding: 0 16px;
    margin: 0;
  }

  :deep(.el-tabs__nav-wrap)::after {
    height: 1px;
    background-color: var(--td-border-color);
  }

  :deep(.el-tabs__item) {
    padding: 0 14px;
    height: 42px;
    line-height: 42px;
    font-size: 13px;
    font-weight: 500;
    letter-spacing: -0.01em;
    color: var(--td-text-secondary);

    &.is-active {
      color: var(--td-text-primary);
      font-weight: 590;
    }
  }

  :deep(.el-tabs__active-bar) {
    background-color: var(--td-text-primary);
    height: 2px;
  }

  :deep(.el-tabs__content) {
    padding: 0 16px 16px;
  }
}

/* 危险 tab 标签不常驻红色：红色留给真正执行删除的那个按钮 */
.danger-tab-label {
  color: inherit;
}

/* ── 区块头 ───────────────────────────────────── */
.section-header {
  margin-bottom: 14px;
}

.section-title {
  font-size: 15px;
  font-weight: 590;
  letter-spacing: -0.015em;
  gap: 8px;
}

/* 计数和已重写页面的 .card-head .n 一套：淡底小方块，不用 el-tag */
.section-count {
  font-size: 11.5px;
  font-weight: 500;
  color: var(--td-text-secondary);
  background: var(--td-bg-section);
  border-radius: 5px;
  padding: 1px 6px;
  font-variant-numeric: tabular-nums;
}

/* ── 基本信息 ─────────────────────────────────── */
.basic-form {
  /* 输入框不跟着容器铺满：一个 1400px 宽的「项目名称」输入框，
     光标落点和内容长度完全对不上 */
  max-width: 560px;

  :deep(.el-form-item) {
    margin-bottom: 18px;
  }

  :deep(.el-form-item__label) {
    font-size: 12.5px;
    font-weight: 500;
    color: var(--td-text-secondary);
  }

  .status-field {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  /* 字段说明就是说明，不是需要被注意到的提示：
     淡蓝底 + 主色文字会和真正的错误提示抢同一套视觉语言 */
  .form-tip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    margin-top: 5px;
    font-size: 12px;
    color: var(--td-text-secondary);
    line-height: 1.4;

    .el-icon {
      flex-shrink: 0;
      font-size: 12px;
      color: var(--td-text-placeholder);
    }
  }

  .form-actions {
    display: flex;
    gap: 10px;
    margin-top: 20px;
    padding-top: 18px;
    border-top: 1px solid var(--td-divider-color);
  }
}

/* ── 成员表格 ─────────────────────────────────── */
.members-table {
  :deep(.el-table__header th) {
    background: var(--td-bg-card);
    font-size: 11.5px;
    font-weight: 500;
    color: var(--td-text-secondary);
  }

  :deep(.el-table__row td) {
    padding: 7px 0;
  }
}

.member-cell,
.user-cell {
  display: flex;
  align-items: center;
  gap: 9px;
}

/* 头像是身份标识，不是色块：小圆点 + 首字母，和工单列表的 .ap .ava 同一套 ——
   中性淡底，身份靠字母颜色区分（personColor）。原来底色按角色分成蓝/橙/绿三档，
   那是把角色当状态在染色，而且一屏五个人只看得出三种，反倒认不出是谁。 */
.member-avatar,
.user-avatar,
.user-option-avatar,
.member-mgmt-avatar {
  width: 22px;
  height: 22px;
  min-width: 22px;
  border-radius: 50%;
  background: var(--td-bg-section);
  color: var(--td-text-secondary);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10.5px;
  font-weight: 600;
  flex-shrink: 0;
}

.member-info,
.user-info {
  min-width: 0;

  .member-name,
  .user-name {
    font-size: 13px;
    font-weight: 500;
    color: var(--td-text-primary);
  }

  .member-username,
  .user-username {
    font-size: 11.5px;
    color: var(--td-text-placeholder);
    font-family: var(--td-font-mono);
  }
}

/* ── 角色 ─────────────────────────────────────── */
.roles-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(268px, 1fr));
  gap: 10px;
}

/* 卡片只留发丝边。原先左侧竖色条 + 灰底图标方块 + 灰底统计盒三层叠加，
   一张角色卡里有三个东西在喊「我是独立对象」。 */
.role-card {
  display: flex;
  flex-direction: column;
  padding: 14px 16px;
  background: var(--td-bg-card);
  border: 1px solid var(--td-border-color);
  border-radius: 10px;
  transition: border-color 150ms ease-out;

  &:hover {
    border-color: var(--td-border-color-dark);

    .role-actions .is-destructive {
      opacity: 1;
    }
  }
}

.role-card-top {
  margin-bottom: 8px;
}

.role-card-title {
  min-width: 0;

  .role-name-row {
    display: flex;
    align-items: center;
    gap: 7px;
  }

  .role-name {
    font-size: 14px;
    font-weight: 590;
    letter-spacing: -0.01em;
    color: var(--td-text-primary);
  }

  .role-key {
    font-size: 11.5px;
    color: var(--td-text-placeholder);
    font-family: var(--td-font-mono);
    margin-top: 2px;
  }
}

.role-desc {
  font-size: 12.5px;
  color: var(--td-text-secondary);
  line-height: 1.55;
  margin-bottom: 10px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

/* 统计从灰底盒改成一行普通文字：它只是两个数字，不需要一个容器 */
.role-stats {
  display: flex;
  gap: 14px;
  margin-bottom: 10px;

  .role-stat-item {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 11.5px;
    color: var(--td-text-secondary);
    font-variant-numeric: tabular-nums;

    .el-icon {
      color: var(--td-text-placeholder);
    }
  }
}

.role-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  margin-top: auto;
  padding-top: 10px;
  border-top: 1px solid var(--td-divider-color);

  .role-action-btn {
    margin-left: 0;
  }
}

/* ── 工单类型 ─────────────────────────────────── */
.issue-types-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(288px, 1fr));
  gap: 10px;
}

.issue-type-card {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 13px 14px;
  background: var(--td-bg-card);
  border: 1px solid var(--td-border-color);
  border-radius: 10px;
  transition: border-color 150ms ease-out;

  &:hover {
    border-color: var(--td-border-color-dark);

    .type-actions .is-destructive {
      opacity: 1;
    }
  }
}

.type-info {
  flex: 1;
  min-width: 0;

  .type-name {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 14px;
    font-weight: 590;
    letter-spacing: -0.01em;
    color: var(--td-text-primary);
  }

  /* 图标是用户自己选的数据，保留；但只上色不再套实心方块 */
  .type-glyph {
    font-size: 15px;
    flex-shrink: 0;
  }

  .type-key {
    font-size: 11.5px;
    color: var(--td-text-placeholder);
    font-family: var(--td-font-mono);
    margin-top: 2px;
  }

  .type-desc {
    font-size: 12.5px;
    color: var(--td-text-secondary);
    line-height: 1.5;
    margin-top: 4px;
  }
}

.type-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

/* ── 行内破坏性操作：悬停才显形，列宽不跳动 ──────── */
.is-destructive {
  opacity: 0;
  transition: opacity 150ms ease-out;
}

@media (hover: none) {
  .is-destructive {
    opacity: 1;
  }
}

/* ── 角色成员管理弹窗 ─────────────────────────── */
.member-mgmt-content {
  .member-mgmt-add {
    display: flex;
    gap: 10px;
    align-items: center;
  }

  .member-mgmt-list {
    min-height: 180px;
    max-height: 380px;
    overflow-y: auto;
    margin-top: 12px;
    border-top: 1px solid var(--td-divider-color);
  }

  .member-mgmt-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 4px;
    border-bottom: 1px solid var(--td-divider-color);

    &:hover {
      background: var(--td-bg-section);
    }
  }

  .member-mgmt-info {
    display: flex;
    align-items: center;
    gap: 9px;
  }

  .member-mgmt-details {
    display: flex;
    flex-direction: column;
  }

  .member-mgmt-name {
    font-size: 13px;
    font-weight: 500;
    color: var(--td-text-primary);
  }

  .member-mgmt-username {
    font-size: 11.5px;
    color: var(--td-text-placeholder);
    font-family: var(--td-font-mono);
  }
}

/* ── 批量添加成员弹窗 ─────────────────────────── */
.batch-member-content {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.batch-member-toolbar {
  display: flex;
  gap: 8px;

  .member-search-input {
    flex: 1;
  }
}

.batch-member-selected-hint {
  font-size: 12.5px;
  color: var(--td-text-secondary);
  display: flex;
  align-items: center;
  gap: 6px;

  strong {
    color: var(--td-text-primary);
    font-variant-numeric: tabular-nums;
  }
}

.batch-member-list {
  border: 1px solid var(--td-border-color);
  border-radius: 8px;
  overflow: hidden;

  :deep(.el-table) {
    --el-table-border-color: var(--td-border-color-light);

    th.el-table__cell {
      background: var(--td-bg-card);
      font-weight: 500;
      font-size: 11.5px;
      color: var(--td-text-secondary);
    }
  }
}

.user-option {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 3px 0;
}

.user-option-info {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;

  .user-option-name {
    font-size: 13px;
    font-weight: 500;
    color: var(--td-text-primary);
    line-height: 1.4;
  }

  .user-option-username {
    font-size: 11.5px;
    color: var(--td-text-placeholder);
    font-family: var(--td-font-mono);
    line-height: 1.4;
  }
}

.role-option {
  display: flex;
  align-items: center;
  gap: 7px;
}

/* ── 图标 / 颜色选择器 ────────────────────────── */
.icon-selector {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 6px;
}

.icon-option {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 5px;
  padding: 9px 6px;
  border: 1px solid var(--td-border-color);
  border-radius: 8px;
  cursor: pointer;
  transition: border-color 150ms ease-out, background-color 150ms ease-out;

  &:hover {
    border-color: var(--td-border-color-dark);
    background: var(--td-bg-section);
  }

  &.active {
    border-color: var(--td-color-primary);
    box-shadow: var(--td-focus-ring);
  }

  .icon-label {
    font-size: 11px;
    color: var(--td-text-secondary);
    text-align: center;
  }
}

.color-selector {
  display: flex;
  align-items: center;
  gap: 14px;
}

.color-presets {
  display: flex;
  gap: 6px;
}

/* 悬停不做 scale：§3.7 只用背景/边框变化 */
.color-preset {
  width: 26px;
  height: 26px;
  border-radius: 6px;
  cursor: pointer;
  border: 1px solid transparent;
  /* 描边压在任意用户色上，不能用 --td-border-color：
     浅色 token 盖在饱和色块上看不见。用黑色低透明度，深浅底都收得住边。 */
  box-shadow: inset 0 0 0 1px rgb(0 0 0 / 8%);
  transition: box-shadow 150ms ease-out;

  &:hover {
    box-shadow: inset 0 0 0 1px rgb(0 0 0 / 20%);
  }

  &.active {
    border-color: var(--td-bg-card);
    box-shadow: 0 0 0 2px var(--td-text-primary);
  }
}

/* ── 站内通知说明 ─────────────────────────────── */
.notification-builtin-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 14px;
  border: 1px solid var(--td-border-color);
  border-radius: 10px;
  margin-bottom: 12px;
  background: var(--td-bg-card);

  .builtin-content {
    flex: 1;
    min-width: 0;
  }

  .builtin-title {
    font-size: 13px;
    font-weight: 500;
    color: var(--td-text-primary);
  }

  .builtin-desc {
    font-size: 12px;
    color: var(--td-text-secondary);
    line-height: 1.5;
    margin-top: 2px;
  }

  .builtin-tag {
    flex-shrink: 0;
  }
}

/* ── 定时日报 ─────────────────────────────────── */
/* 原来是灰底盒嵌在白卡里（盒中盒），且控件全宽堆叠，一屏只装得下六个字段。
   改成白底 + 发丝边，表单两列排布，纵向间距压到 14。 */
.digest-card {
  border: 1px solid var(--td-border-color);
  border-radius: 10px;
  margin-bottom: 16px;
  background: var(--td-bg-card);

  .digest-card-header {
    padding: 12px 16px;
    border-bottom: 1px solid var(--td-divider-color);

    .digest-card-title {
      display: flex;
      align-items: center;
      gap: 7px;
      font-size: 13.5px;
      font-weight: 590;
      letter-spacing: -0.01em;
      color: var(--td-text-primary);

      .builtin-tag {
        margin-left: 2px;
      }
    }

    .digest-card-desc {
      font-size: 12px;
      color: var(--td-text-secondary);
      line-height: 1.5;
      margin-top: 3px;
    }
  }

  .digest-card-body {
    padding: 14px 16px;
  }

  .digest-form {
    max-width: 720px;

    :deep(.el-form-item) {
      margin-bottom: 14px;
    }

    :deep(.el-form-item__label) {
      padding-bottom: 4px;
      font-size: 12.5px;
      font-weight: 500;
      color: var(--td-text-secondary);
      line-height: 1.4;
    }
  }

  .digest-form-tip {
    font-size: 12px;
    color: var(--td-text-placeholder);
    margin-left: 10px;
  }

  .digest-types-wrapper {
    width: 100%;

    .digest-form-tip {
      margin-left: 0;
      margin-top: 5px;
    }
  }

  .digest-types-group {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 14px;
  }

  .digest-type-checkbox {
    margin-right: 0;
  }
}

/* ── 工作流方案 ───────────────────────────────── */
/* 提示原本是蓝底蓝字的色块，和真正需要注意的错误提示抢同一套语言 */
.workflow-scheme-tip {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin-bottom: 12px;
  font-size: 12.5px;
  color: var(--td-text-secondary);
  line-height: 1.5;

  .el-icon {
    flex-shrink: 0;
    margin-top: 2px;
    font-size: 14px;
    color: var(--td-text-placeholder);
  }
}

.schemes-list {
  display: flex;
  flex-direction: column;
  background: var(--td-bg-card);
  border: 1px solid var(--td-border-color);
  border-radius: 10px;
  overflow: hidden;
}

.scheme-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  transition: background-color 150ms ease-out;

  &:not(:last-child) {
    border-bottom: 1px solid var(--td-divider-color);
  }

  &:hover {
    background: var(--td-bg-section);

    .is-destructive {
      opacity: 1;
    }
  }
}

.scheme-row-left {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

.scheme-type-glyph {
  font-size: 15px;
  flex-shrink: 0;
}

.scheme-row-info {
  flex: 1;
  min-width: 0;
}

.scheme-row-name {
  display: flex;
  align-items: center;
  gap: 6px;

  .scheme-type-name {
    font-size: 13px;
    font-weight: 590;
    color: var(--td-text-primary);
  }

  .scheme-arrow {
    color: var(--td-text-placeholder);
    font-size: 13px;
    flex-shrink: 0;
  }

  .scheme-workflow-name {
    font-size: 13px;
    font-weight: 500;
    color: var(--td-text-regular);
  }
}

.scheme-row-detail {
  font-size: 11.5px;
  color: var(--td-text-placeholder);
  margin-top: 2px;
}

.scheme-row-right {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  margin-left: 14px;
}

.scheme-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 2px 0;
}

.scheme-option-glyph {
  flex-shrink: 0;
}

.scheme-option-info {
  display: flex;
  flex-direction: column;

  .scheme-option-name {
    font-size: 13px;
    font-weight: 500;
    color: var(--td-text-primary);
  }

  .scheme-option-desc {
    font-size: 11.5px;
    color: var(--td-text-placeholder);
    line-height: 1.3;
  }
}

/* ── 权限配置弹窗 ─────────────────────────────── */
.perm-config-content {
  min-height: 180px;
}

.perm-group {
  margin-bottom: 16px;

  &:last-child {
    margin-bottom: 0;
  }
}

/* 分组头原来是灰底圆角盒，和下面的选项盒构成盒中盒；改成一条发丝线 */
.perm-group-header {
  padding: 0 0 6px;
  border-bottom: 1px solid var(--td-divider-color);
  margin-bottom: 8px;

  :deep(.el-checkbox__label) {
    font-weight: 590;
    color: var(--td-text-primary);
    font-size: 13px;
  }
}

.perm-group-items {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 20px;
  padding: 2px 0 2px 24px;

  :deep(.el-checkbox__label) {
    font-size: 12.5px;
    color: var(--td-text-regular);
  }
}

/* ── 危险操作 ─────────────────────────────────── */
/* 原来整块粉红底、标题正文列表全红 —— 全红等于没红，真正要警觉的按钮反而没重量。
   说明文字回正常色，红色只留给按钮和标题那个警告图标。 */
.danger-content {
  display: flex;
  align-items: flex-start;
}

.danger-card {
  width: 100%;
  max-width: 620px;
  background: var(--td-bg-card);
  border: 1px solid var(--td-border-color);
  border-radius: 10px;
  padding: 18px 20px;

  .danger-card-title {
    display: flex;
    align-items: center;
    gap: 7px;
    font-size: 15px;
    font-weight: 590;
    letter-spacing: -0.015em;
    color: var(--td-text-primary);
    margin-bottom: 8px;

    .el-icon {
      color: var(--td-color-danger);
    }
  }

  .danger-card-desc {
    margin: 0 0 10px;
    font-size: 13px;
    color: var(--td-text-secondary);
    line-height: 1.6;
  }

  .danger-card-list {
    margin: 0 0 16px;
    padding-left: 17px;
    font-size: 12.5px;
    color: var(--td-text-secondary);
    line-height: 1.7;
  }
}
</style>

<style lang="scss">
// 用户选择下拉框样式（不能用 scoped，因为是 popper）
.user-select-popper {
  .el-select-dropdown__item {
    height: auto !important;
    padding: 8px 12px !important;
    line-height: normal !important;
  }
}

// 通知渠道对话框样式（dialog teleport 到 body，需要非 scoped）
</style>
