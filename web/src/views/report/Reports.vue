<template>
  <div class="page">
    <div class="page-head">
      <h1>{{ t('report.title') }}</h1>
      <div class="grow"></div>
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        :range-separator="t('report.dateSeparator')"
        :start-placeholder="t('report.startDate')"
        :end-placeholder="t('report.endDate')"
        format="YYYY-MM-DD"
        value-format="YYYY-MM-DD"
        :shortcuts="dateShortcuts"
        class="date-range"
        @change="handleDateChange"
      />
      <el-select v-model="selectedProject" :placeholder="t('report.allProjects')" clearable class="filter-select" filterable @change="handleProjectChange">
        <el-option v-for="p in projects" :key="p.project_key" :label="`${p.project_key} - ${p.name}`" :value="p.project_key" />
      </el-select>
    </div>

    <div class="tabs" role="tablist">
      <button
        v-for="tab in reportTabs"
        :key="tab"
        class="tab"
        role="tab"
        :aria-selected="activeTab === tab"
        @click="selectTab(tab)"
      >
        {{ t(`report.tab${tab.charAt(0).toUpperCase()}${tab.slice(1)}`) }}
      </button>
    </div>

    <template v-if="activeTab === 'issues'">
      <div v-loading="loading.issues" class="tab-content">
        <!-- 汇总卡片 -->
        <section class="card">
          <div class="kpis">
            <div class="kpi"><div class="k">{{ t('report.issueTotal') }}</div><div class="v">{{ issueStats.summary?.total || 0 }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.resolved') }}</div><div class="v">{{ issueStats.summary?.resolved || 0 }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.inProgress') }}</div><div class="v">{{ issueStats.summary?.in_progress || 0 }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.avgResolveTime') }}</div><div class="v">{{ formatHours(issueStats.summary?.avg_resolve_time || 0) }}</div></div>
          </div>
        </section>

        <!-- 分布图表：第一行 状态 + 优先级 -->
        <div class="stretch-row">
          <section class="card">
            <div class="card-head">
              <h2>{{ t('report.statusDist') }}</h2>
            </div>
            <TdBarChart :items="chartItems(issueStats.status_distribution, getStatusLabel, getStatusColor, STATUS_ORDER)" :height="chartHeight(issueStats.status_distribution)" />
          </section>
          <section class="card">
            <div class="card-head">
              <h2>{{ t('report.priorityDist') }}</h2>
            </div>
            <TdBarChart :items="chartItems(issueStats.priority_distribution, undefined, getPriorityColor, PRIORITY_ORDER)" :height="chartHeight(issueStats.priority_distribution)" />
          </section>
        </div>

        <!-- 分布图表：第二行 工单类型 + 指派人 -->
        <div class="stretch-row">
          <section class="card">
            <div class="card-head">
              <h2>{{ t('report.typeDist') }}</h2>
            </div>
            <TdBarChart :items="chartItems(issueStats.type_distribution)" :height="chartHeight(issueStats.type_distribution)" />
          </section>
          <section class="card">
            <div class="card-head">
              <h2>{{ t('report.assigneeDist') }}</h2>
            </div>
            <TdBarChart :items="chartItems(issueStats.assignee_distribution)" :height="chartHeight(issueStats.assignee_distribution)" />
          </section>
        </div>

        <!-- 分布图表：第三行 Epic -->
        <section class="card">
          <div class="card-head">
            <h2>{{ t('report.epicDist') }}</h2>
          </div>
          <TdBarChart :items="chartItems(issueStats.epic_distribution)" :height="chartHeight(issueStats.epic_distribution)" />
        </section>
      </div>

      <!-- 时间趋势 -->
      <section class="card">
        <div class="card-head">
          <h2>{{ t('report.issueTrend') }}</h2>
        </div>
        <div class="timeline-list">
          <el-table v-if="issueStats.timeline?.length" :data="issueStats.timeline || []" stripe size="small" :header-cell-style="{ background: 'var(--td-table-header-bg)', color: 'var(--td-text-regular)', fontWeight: 600 }">
            <el-table-column :label="t('report.date')" min-width="120">
              <template #default="{ row }">
                <span class="timeline-date">{{ formatDate(row.date) }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('report.created')" min-width="100">
              <template #default="{ row }">
                <span class="timeline-badge created">{{ row.created }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('report.inProgress')" min-width="100">
              <template #default="{ row }">
                <span class="timeline-badge in-progress">{{ row.in_progress }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('report.resolvedCol')" min-width="100">
              <template #default="{ row }">
                <span class="timeline-badge resolved">{{ row.resolved }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('report.closed')" min-width="100">
              <template #default="{ row }">
                <span class="timeline-badge closed">{{ row.closed }}</span>
              </template>
            </el-table-column>
          </el-table>
          <TdEmptyState v-if="!issueStats.timeline?.length" preset="no-data" :title="t('report.noTrend')" />
        </div>
      </section>
    </template>

    <template v-if="activeTab === 'sla'">
      <div v-loading="loading.sla" class="tab-content">
        <!-- SLA 汇总卡片 -->
        <section class="card">
          <div class="kpis">
            <div class="kpi"><div class="k">{{ t('report.slaTotalLabel', { n: slaReport.summary?.resolved_issues || 0 }) }}</div><div class="v">{{ slaReport.summary?.total_issues || 0 }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.slaMetRate', { met: slaReport.summary?.sla_met || 0, violated: slaReport.summary?.sla_violated || 0 }) }}</div><div class="v">{{ formatPercent(slaReport.summary?.sla_rate || 0) }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.mtta') }}</div><div class="v">{{ formatMinutes(slaReport.summary?.mtta || 0) }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.mttr') }}</div><div class="v">{{ formatMinutes(slaReport.summary?.mttr || 0) }}</div></div>
          </div>
        </section>

        <!-- 按优先级 + 按项目 SLA（等高两列） -->
        <div class="stretch-row">
          <section class="card">
            <div class="card-head">
              <h2>{{ t('report.slaByPriority') }}</h2>
            </div>
            <el-table :data="slaReport.by_priority || []" stripe size="small" :header-cell-style="{ background: 'var(--td-table-header-bg)', color: 'var(--td-text-regular)', fontWeight: 600 }">
              <el-table-column prop="priority" :label="t('issue.priority')" min-width="70">
                <template #default="{ row }">
                  <el-tag :type="getPriorityType(row.priority)" effect="dark" size="small">{{ row.priority }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="total" :label="t('report.total')" min-width="55" />
              <el-table-column prop="resolved" :label="t('report.resolvedCol')" min-width="55" />
              <el-table-column :label="t('report.slaTarget')" min-width="80">
                <template #default="{ row }">{{ formatMinutes(row.sla_target) }}</template>
              </el-table-column>
              <el-table-column label="MTTR" min-width="80">
                <template #default="{ row }">{{ formatMinutes(row.mttr) }}</template>
              </el-table-column>
              <el-table-column :label="t('report.metRate')" min-width="75">
                <template #default="{ row }">
                  <span :class="getSLARateClass(row.sla_rate)">{{ formatPercent(row.sla_rate) }}</span>
                </template>
              </el-table-column>
            </el-table>
          </section>
          <section class="card">
            <div class="card-head">
              <h2>{{ t('report.slaByProject') }}</h2>
            </div>
            <el-table v-if="slaReport.by_project?.length" :data="slaReport.by_project || []" stripe size="small" :header-cell-style="{ background: 'var(--td-table-header-bg)', color: 'var(--td-text-regular)', fontWeight: 600 }">
              <el-table-column :label="t('issue.project')" min-width="140">
                <template #default="{ row }">
                  <span class="project-key-badge">{{ row.project_key }}</span>
                  <span class="project-name-text">{{ row.project_name }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="total" :label="t('report.total')" width="55" />
              <el-table-column prop="resolved" :label="t('report.resolvedCol')" width="55" />
              <el-table-column label="MTTR" width="80">
                <template #default="{ row }">{{ formatMinutes(row.mttr) }}</template>
              </el-table-column>
              <el-table-column :label="t('report.metRate')" width="75">
                <template #default="{ row }">
                  <span :class="getSLARateClass(row.sla_rate)">{{ formatPercent(row.sla_rate) }}</span>
                </template>
              </el-table-column>
            </el-table>
            <TdEmptyState v-if="!slaReport.by_project?.length" preset="no-data" :title="t('report.noData')" />
          </section>
        </div>

        <!-- SLA 超时工单列表 -->
        <section class="card">
          <div class="card-head">
            <h2>{{ t('report.slaViolations') }}</h2>
            <span v-if="slaReport.violations?.length" class="n">{{ t('report.count', { n: slaReport.violations.length }) }}</span>
          </div>
          <el-table v-if="slaReport.violations?.length" :data="slaReport.violations || []" stripe size="small" :header-cell-style="{ background: 'var(--td-table-header-bg)', color: 'var(--td-text-regular)', fontWeight: 600 }">
            <el-table-column prop="issue_key" :label="t('issue.key')" width="120">
              <template #default="{ row }">
                <router-link :to="`/issues/${row.issue_key}`" class="issue-link">{{ row.issue_key }}</router-link>
              </template>
            </el-table-column>
            <el-table-column prop="title" :label="t('issue.title')" min-width="200" show-overflow-tooltip />
            <el-table-column prop="priority" :label="t('issue.priority')" width="80">
              <template #default="{ row }">
                <el-tag :type="getPriorityType(row.priority)" effect="dark" size="small">{{ row.priority }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="assignee_name" :label="t('report.handler')" width="100" />
            <el-table-column :label="t('report.slaTarget')" width="100">
              <template #default="{ row }">{{ formatMinutes(row.sla_target) }}</template>
            </el-table-column>
            <el-table-column :label="t('report.actualTime')" width="100">
              <template #default="{ row }">
                <span class="text-danger">{{ formatMinutes(row.actual_time) }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('report.overdue')" width="100">
              <template #default="{ row }">
                <span class="text-danger">{{ formatOverdue(row.overdue_by) }}</span>
              </template>
            </el-table-column>
          </el-table>
          <TdEmptyState v-if="!slaReport.violations?.length" preset="no-data" :title="t('report.noViolations')" />
        </section>
      </div>
    </template>

    <template v-if="activeTab === 'alerts'">
      <div v-loading="loading.alerts" class="tab-content">
        <!-- 告警汇总卡片 -->
        <section class="card">
          <div class="kpis">
            <div class="kpi"><div class="k">{{ t('report.alertTotal') }}</div><div class="v">{{ alertStats.summary?.total || 0 }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.firing') }}</div><div class="v">{{ alertStats.summary?.firing || 0 }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.acked') }}</div><div class="v">{{ alertStats.summary?.acked || 0 }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.recovered') }}</div><div class="v">{{ alertStats.summary?.resolved || 0 }}</div></div>
          </div>
        </section>

        <!-- 严重程度分布 + 告警趋势（等高两列） -->
        <div class="stretch-row">
          <section class="card">
            <div class="card-head">
              <h2>{{ t('report.severityDist') }}</h2>
            </div>
            <TdBarChart :items="chartItems(alertStats.severity_distribution, getSeverityLabel, getSeverityColor)" :height="chartHeight(alertStats.severity_distribution)" />
          </section>
          <section class="card">
            <div class="card-head">
              <h2>{{ t('report.alertTrend') }}</h2>
            </div>
            <div class="timeline-list">
              <el-table v-if="alertStats.timeline?.length" :data="alertStats.timeline || []" stripe size="small" :header-cell-style="{ background: 'var(--td-table-header-bg)', color: 'var(--td-text-regular)', fontWeight: 600 }">
                <el-table-column :label="t('report.date')" min-width="120">
                  <template #default="{ row }">
                    <span class="timeline-date">{{ formatDate(row.date) }}</span>
                  </template>
                </el-table-column>
                <el-table-column :label="t('report.triggered')" min-width="100">
                  <template #default="{ row }">
                    <span class="timeline-badge created">{{ row.firing }}</span>
                  </template>
                </el-table-column>
                <el-table-column :label="t('report.recoveredCol')" min-width="100">
                  <template #default="{ row }">
                    <span class="timeline-badge resolved">{{ row.resolved }}</span>
                  </template>
                </el-table-column>
              </el-table>
              <TdEmptyState v-if="!alertStats.timeline?.length" preset="no-data" :title="t('report.noTrend')" />
            </div>
          </section>
        </div>

        <!-- Top 告警（全宽，含严重程度列） -->
        <section class="card">
          <div class="card-head">
            <h2>{{ t('report.topAlerts') }}</h2>
          </div>
          <el-table v-if="alertStats.top_alerts?.length" :data="alertStats.top_alerts || []" stripe size="small" :header-cell-style="{ background: 'var(--td-table-header-bg)', color: 'var(--td-text-regular)', fontWeight: 600 }">
            <el-table-column :label="t('report.rank')" width="60" align="center">
              <template #default="{ $index }">
                <span :class="['rank-badge', { 'top-3': $index < 3 }]">{{ $index + 1 }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="alert_name" :label="t('alert.name')" min-width="200" show-overflow-tooltip />
            <el-table-column prop="severity" :label="t('alert.severity')" width="100">
              <template #default="{ row }">
                <el-tag :type="getSeverityTagType(row.severity)" size="small" effect="plain">{{ getSeverityLabel(row.severity) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="count" :label="t('report.times')" width="100" align="center">
              <template #default="{ row }">
                <span class="alert-count-value">{{ row.count }}</span>
              </template>
            </el-table-column>
          </el-table>
          <TdEmptyState v-if="!alertStats.top_alerts?.length" preset="no-data" :title="t('report.noData')" />
        </section>
      </div>
    </template>

    <template v-if="activeTab === 'performance'">
      <div v-loading="loading.performance" class="tab-content">
        <!-- 绩效汇总卡片 -->
        <section class="card">
          <div class="kpis">
            <div class="kpi"><div class="k">{{ t('report.participants') }}</div><div class="v">{{ performanceSummary.userCount }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.totalAssigned') }}</div><div class="v">{{ performanceSummary.totalAssigned }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.totalResolved') }}</div><div class="v">{{ performanceSummary.totalResolved }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.avgResolveRate') }}</div><div class="v">{{ formatPercent(performanceSummary.avgResolveRate) }}</div></div>
          </div>
        </section>

        <!-- 绩效表格 -->
        <section class="card">
          <div class="card-head">
            <h2>{{ t('report.performanceTitle') }}</h2>
          </div>
          <el-table v-if="userPerformance.length > 0" :data="userPerformance" stripe size="small" :header-cell-style="{ background: 'var(--td-table-header-bg)', color: 'var(--td-text-regular)', fontWeight: 600 }">
            <el-table-column :label="t('report.rank')" width="60" align="center">
              <template #default="{ $index }">
                <span :class="['rank-badge', { 'top-3': $index < 3 }]">{{ $index + 1 }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="display_name" :label="t('report.userCol')" min-width="150">
              <template #default="{ row }">
                <div class="report-user">
                  <el-avatar :size="24" class="dist-avatar" :style="{ color: personColor(row.display_name || row.username) }">{{ (row.display_name || row.username)?.charAt(0) }}</el-avatar>
                  <span>{{ row.display_name || row.username }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="assigned" :label="t('report.assignedCol')" width="100" align="center" />
            <el-table-column prop="resolved" :label="t('report.resolvedIssues')" width="100" align="center" />
            <el-table-column :label="t('report.resolveRate')" width="120">
              <template #default="{ row }">
                <el-progress
                  :percentage="row.assigned > 0 ? Math.round(row.resolved / row.assigned * 100) : 0"
                  :stroke-width="8"
                  :show-text="true"
                  :format="(p: number) => p + '%'"
                />
              </template>
            </el-table-column>
            <el-table-column :label="t('report.avgResolve')" width="120">
              <template #default="{ row }">{{ formatHours(row.avg_resolve_time) }}</template>
            </el-table-column>
          </el-table>
          <TdEmptyState v-if="!userPerformance.length" preset="no-data" :title="t('report.noPerformance')" />
        </section>
      </div>
    </template>

    <template v-if="activeTab === 'worklogs'">
      <div v-loading="loading.worklogs" class="tab-content">
        <!-- 汇总卡片 -->
        <section class="card">
          <div class="kpis">
            <div class="kpi"><div class="k">{{ t('report.totalHours') }}</div><div class="v">{{ formatWorklogTime(worklogStats.summary?.total_time_sec || 0) }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.entryCount') }}</div><div class="v">{{ worklogStats.summary?.total_entries || 0 }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.participants') }}</div><div class="v">{{ worklogStats.summary?.active_users || 0 }}</div></div>
            <div class="kpi"><div class="k">{{ t('report.dailyAvg') }}</div><div class="v">{{ formatWorklogTime(worklogStats.summary?.avg_daily_time_sec || 0) }}</div></div>
          </div>
        </section>

        <div class="stretch-row">
          <!-- 每日工时柱状图 -->
          <section class="card">
            <div class="card-head">
              <h2>{{ t('report.dailyHours') }}</h2>
            </div>
            <div class="daily-worklog-chart">
              <div v-if="worklogStats.daily_stats?.length" class="bar-chart">
                <div v-for="item in worklogStats.daily_stats" :key="item.date" class="bar-item">
                  <div class="bar-value">{{ formatWorklogTime(item.total_time_sec) }}</div>
                  <div class="bar-fill" :style="{ height: getDailyBarHeight(item.total_time_sec) + '%' }"></div>
                  <div class="bar-label">{{ formatShortDate(item.date) }}</div>
                </div>
              </div>
              <TdEmptyState v-else preset="no-data" :title="t('report.noWorklog')" />
            </div>
          </section>

          <!-- 工作类型分布 -->
          <section class="card">
            <div class="card-head">
              <h2>{{ t('report.workTypeDist') }}</h2>
            </div>
            <TdBarChart :items="worklogTypeItems" :unit="t('report.minutes')" :height="chartHeight(worklogTypeItems)" />
          </section>
        </div>

        <!-- 个人工时排行 -->
        <section class="card">
          <div class="card-head">
            <h2>{{ t('report.worklogRanking') }}</h2>
          </div>
          <el-table v-if="worklogStats.user_stats?.length" :data="worklogStats.user_stats || []" stripe size="small" :header-cell-style="{ background: 'var(--td-table-header-bg)', color: 'var(--td-text-regular)', fontWeight: 600 }">
            <el-table-column :label="t('report.rank')" width="60" align="center">
              <template #default="{ $index }">
                <span :class="['rank-badge', { 'top-3': $index < 3 }]">{{ $index + 1 }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="display_name" :label="t('report.userCol')" min-width="150">
              <template #default="{ row }">
                <div class="report-user">
                  <el-avatar :size="24" class="dist-avatar" :style="{ color: personColor(row.display_name) }">{{ row.display_name?.charAt(0) }}</el-avatar>
                  <span>{{ row.display_name }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="t('report.totalHours')" min-width="120">
              <template #default="{ row }">
                <span class="worklog-time-value">{{ formatWorklogTime(row.total_time_sec) }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="entry_count" :label="t('report.entries')" width="100" align="center" />
            <el-table-column :label="t('report.share')" width="200">
              <template #default="{ row }">
                <el-progress :percentage="getUserPercentage(row.total_time_sec)" :stroke-width="8" :show-text="true" :format="(p: number) => p.toFixed(1) + '%'" />
              </template>
            </el-table-column>
          </el-table>
          <TdEmptyState v-if="!worklogStats.user_stats?.length" preset="no-data" :title="t('report.noWorklog')" />
        </section>

        <!-- 工时明细网格（用户 × 日期） -->
        <section class="card">
          <div class="card-head">
            <h2>{{ t('report.worklogDetail') }}</h2>
            <div class="grow"></div>
            <div class="grid-month-picker">
              <el-button text :icon="ArrowLeft" size="small" @click="changeGridMonth(-1)" />
              <span class="grid-month-label">{{ gridMonthLabel }}</span>
              <el-button text :icon="ArrowRight" size="small" @click="changeGridMonth(1)" />
            </div>
          </div>
          <div class="worklog-grid-wrap">
            <el-table
              v-if="worklogStats.grid?.length"
              :data="worklogGridData"
              stripe
              size="small"
              border
              show-summary
              :summary-method="worklogGridSummary"
              :header-cell-style="{ background: 'var(--td-table-header-bg)', color: 'var(--td-text-regular)', fontWeight: 600, fontSize: '12px', padding: '6px 0' }"
              :cell-style="worklogGridCellStyle"
              class="worklog-grid-table"
            >
              <el-table-column prop="display_name" :label="t('report.userCol')" fixed width="120">
                <template #default="{ row }">
                  <div class="report-user">
                    <el-avatar :size="22" class="dist-avatar" :style="{ color: personColor(row.display_name) }">{{ row.display_name?.charAt(0) }}</el-avatar>
                    <span>{{ row.display_name }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column
                v-for="d in (worklogStats.grid_dates || [])"
                :key="d"
                :prop="'d_' + d"
                :label="formatGridDate(d)"
                min-width="62"
                align="center"
                :class-name="isWeekend(d) ? 'weekend-col' : ''"
              >
                <template #default="{ row }">
                  <el-popover
                    v-if="row['d_' + d]"
                    placement="top"
                    trigger="hover"
                    :width="260"
                    :show-after="200"
                  >
                    <template #reference>
                      <span class="grid-cell-value">{{ formatGridHours(row['d_' + d]) }}</span>
                    </template>
                    <div class="grid-popover">
                      <div class="grid-popover-header">
                        <span>{{ row.display_name }} · {{ dayjs(d).format(t('report.dayFormat')) }}</span>
                        <span class="grid-popover-total">{{ formatGridHours(row['d_' + d]) }}</span>
                      </div>
                      <div v-for="item in row['_details_' + d]" :key="item.issue_key" class="grid-popover-item">
                        <router-link :to="`/issues/${item.issue_key}`" class="grid-popover-key">{{ item.issue_key }}</router-link>
                        <span class="grid-popover-title">{{ item.title }}</span>
                        <span class="grid-popover-time">{{ formatGridHours(item.time_sec) }}</span>
                      </div>
                    </div>
                  </el-popover>
                </template>
              </el-table-column>
              <el-table-column prop="_total" :label="t('report.grandTotal')" fixed="right" width="80" align="center">
                <template #default="{ row }">
                  <span class="grid-total-value">{{ formatGridHours(row._total) }}</span>
                </template>
              </el-table-column>
            </el-table>
            <TdEmptyState v-else preset="no-data" :title="t('report.noWorklogDetail')" />
          </div>
        </section>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import TdBarChart from '@/components/chart/TdBarChart.vue'
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, ArrowRight } from '@element-plus/icons-vue'
import { getIssueStats, getSLAReport, getAlertStats, getUserPerformance, getWorklogStats } from '@/api/report'
import { getAllProjects } from '@/api/project'
import type { IssueStats, SLAReport, AlertStats, UserPerformance, WorklogStats } from '@/types/report'
import type { Project } from '@/types/project'
import dayjs from 'dayjs'
import { personColor } from '@/utils/avatar'


const { t, te } = useI18n()

const route = useRoute()
const router = useRouter()

// 多色调色板
// 分布图的配色由 TdBarChart 统一从 --td-cat-* token 取用，
// 页面侧不再持有任何颜色数组。

// 从 URL query 初始化筛选条件
const initQuery = route.query
const defaultStart = dayjs().subtract(30, 'day').format('YYYY-MM-DD')
const defaultEnd = dayjs().format('YYYY-MM-DD')

const dateRange = ref<[string, string]>([
  (initQuery.start_date as string) || defaultStart,
  (initQuery.end_date as string) || defaultEnd,
])

const dateShortcuts = [
  { text: t('report.lastWeek'), value: () => [dayjs().subtract(7, 'day').toDate(), dayjs().toDate()] },
  { text: t('report.lastMonth'), value: () => [dayjs().subtract(30, 'day').toDate(), dayjs().toDate()] },
  { text: t('report.lastQuarter'), value: () => [dayjs().subtract(90, 'day').toDate(), dayjs().toDate()] },
]

const selectedProject = ref((initQuery.project_key as string) || '')
const projects = ref<Project[]>([])

const validTabs = ['issues', 'sla', 'alerts', 'performance', 'worklogs']
const activeTab = ref(validTabs.includes(initQuery.tab as string) ? (initQuery.tab as string) : 'issues')

// 筛选条件同步到 URL query params
const syncQueryToUrl = () => {
  const query: Record<string, string> = {}
  if (activeTab.value && activeTab.value !== 'issues') query.tab = activeTab.value
  if (selectedProject.value) query.project_key = selectedProject.value
  // 仅当日期非默认值时持久化
  if (dateRange.value[0] !== defaultStart) query.start_date = dateRange.value[0]
  if (dateRange.value[1] !== defaultEnd) query.end_date = dateRange.value[1]
  router.replace({ query })
}
const loading = reactive({
  issues: false,
  sla: false,
  alerts: false,
  performance: false,
  worklogs: false })

const issueStats = ref<Partial<IssueStats>>({})
const slaReport = ref<Partial<SLAReport>>({})
const alertStats = ref<Partial<AlertStats>>({})
const userPerformance = ref<UserPerformance[]>([])
const worklogStats = ref<Partial<WorklogStats>>({})

// 工时明细月份（默认当月）
const gridMonth = ref(dayjs().format('YYYY-MM'))
const gridMonthLabel = computed(() => dayjs(gridMonth.value + '-01').format(t('report.monthFormat')))

const loadProjects = async () => {
  try {
    const { data } = await getAllProjects()
    projects.value = data.data
  } catch {
    // ignored
  }
}

// 加载工单统计
const loadIssueStats = async () => {
  loading.issues = true
  try {
    const { data } = await getIssueStats({
      project_key: selectedProject.value || undefined,
      start_date: dateRange.value[0],
      end_date: dateRange.value[1] })
    issueStats.value = data.data
  } catch {
    // ignored
  } finally {
    loading.issues = false
  }
}

// 加载 SLA 报表
const loadSLAReport = async () => {
  loading.sla = true
  try {
    const { data } = await getSLAReport({
      project_key: selectedProject.value || undefined,
      start_date: dateRange.value[0],
      end_date: dateRange.value[1] })
    slaReport.value = data.data
  } catch {
    // ignored
  } finally {
    loading.sla = false
  }
}

// 加载告警统计
const loadAlertStats = async () => {
  loading.alerts = true
  try {
    const { data } = await getAlertStats({
      project_key: selectedProject.value || undefined,
      start_date: dateRange.value[0],
      end_date: dateRange.value[1] })
    alertStats.value = data.data
  } catch {
    // ignored
  } finally {
    loading.alerts = false
  }
}

// 加载用户绩效
const loadUserPerformance = async () => {
  loading.performance = true
  try {
    const { data } = await getUserPerformance({
      project_key: selectedProject.value || undefined,
      start_date: dateRange.value[0],
      end_date: dateRange.value[1] })
    userPerformance.value = data.data
  } catch {
    // ignored
  } finally {
    loading.performance = false
  }
}

// 加载工时统计
const loadWorklogStats = async () => {
  loading.worklogs = true
  try {
    const { data } = await getWorklogStats({
      project_key: selectedProject.value || undefined,
      start_date: dateRange.value[0],
      end_date: dateRange.value[1],
      grid_month: gridMonth.value })
    worklogStats.value = data.data
  } catch {
    // ignored
  } finally {
    loading.worklogs = false
  }
}

// 根据当前 tab 加载数据
const loadData = () => {
  switch (activeTab.value) {
    case 'issues':
      loadIssueStats()
      break
    case 'sla':
      loadSLAReport()
      break
    case 'alerts':
      loadAlertStats()
      break
    case 'performance':
      loadUserPerformance()
      break
    case 'worklogs':
      loadWorklogStats()
      break
  }
}

const handleDateChange = () => {
  syncQueryToUrl()
  loadData()
}

// 原生 tab：点一下切换并触发原来的加载逻辑
const reportTabs = validTabs

const selectTab = (tab: string) => {
  activeTab.value = tab
  handleTabChange()
}

const handleTabChange = () => {
  syncQueryToUrl()
  loadData()
}

const handleProjectChange = () => {
  syncQueryToUrl()
  loadData()
}

// 格式化函数
// 时长不可能为负：出现负值只说明源数据有问题（例如 resolved_at 早于 created_at）。
// 直接渲染会变成"-8420 分钟"这种读不懂的数字，宁可显示"-"表示无有效值。
const isValidDuration = (value: number) => Number.isFinite(value) && value >= 0

const formatHours = (hours: number) => {
  if (!isValidDuration(hours)) return '-'
  if (hours < 1) return t('report.durationMinutes', { n: Math.round(hours * 60) })
  if (hours < 24) return t('report.durationHours', { n: hours.toFixed(1) })
  return t('report.durationDays', { n: (hours / 24).toFixed(1) })
}

const formatMinutes = (minutes: number) => {
  if (!isValidDuration(minutes)) return '-'
  if (minutes < 60) return t('report.durationMinutes', { n: Math.round(minutes) })
  if (minutes < 1440) return t('report.durationHours', { n: (minutes / 60).toFixed(1) })
  return t('report.durationDays', { n: (minutes / 1440).toFixed(1) })
}

// 超时时长单独走一遍：前缀"+"只在真的有超时数值时才有意义
const formatOverdue = (minutes: number) => (isValidDuration(minutes) ? `+${formatMinutes(minutes)}` : '-')

const formatPercent = (value: number) => `${value.toFixed(1)}%`

const formatDate = (date: string) => {
  if (!date) return '-'
  return dayjs(date).format('YYYY-MM-DD')
}

// 状态相关
type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger'

const getStatusLabel = (status: string) => {
    // 状态文案统一走语言包：它同时出现在列表、详情、报表、看板，
  // 各处各写一份必然改一处漏三处
  return t(`issue.statusMap.${status}`)
}

// 把后端返回的分布数据转成图表组件需要的形状。
// 可选的 labelFn 用于把状态码这类枚举值翻译成中文；
// colorFn 用于"有语义"的维度（状态/优先级/告警级别）——它们必须走语义色，
// 不传则由组件按固定顺序取分类色阶。
type DistItem = { name: string; value: number; ratio: number }
// 状态与优先级是有序维度，必须按固定顺序呈现。
// 后端按数量倒序返回，直接画出来会出现 P3 排在 P0 前面、
// 「已完成」排在「待处理」前面 —— 读者要在脑子里重排一遍才能比较，
// 而流程/紧急度的顺序本身就是这两张图要传达的信息之一。
const STATUS_ORDER = ['open', 'in_progress', 'pending_review', 'resolved', 'closed', 'merged']
const PRIORITY_ORDER = ['P0', 'P1', 'P2', 'P3']

const chartItems = (
  list: DistItem[] | undefined,
  labelFn?: (v: string) => string,
  colorFn?: (v: string) => string,
  order?: string[],
) => {
  const items = (list || []).map((it) => ({
    name: labelFn ? labelFn(it.name) : it.name,
    value: it.value,
    ratio: it.ratio,
    color: colorFn ? colorFn(it.name) : undefined,
    // 排序要按原始值，labelFn 之后拿到的是译文，顺序表对不上
    rawName: it.name }))
  // 没有指定顺序表的维度（类型 / 指派人 / Epic）后端是遍历 map 返回的，
  // 顺序随机 —— 刷新一次条目就换个位置。按值降序，大头在上，顺序也稳定。
  if (!order) return items.sort((a, b) => b.value - a.value)
  // 不在顺序表里的（后端新增了状态值）排到末尾，不因此丢数据
  const rank = (n: string) => {
    const i = order.indexOf(n)
    return i === -1 ? order.length : i
  }
  return items.sort((a, b) => rank(a.rawName) - rank(b.rawName))
}

// 工时统计的字段名与其它分布不同（work_type / total_time_sec），单独归一
const workTypeLabel = (code?: string) => {
  if (!code) return t('report.uncategorized')
  const key = `issue.workTypeMap.${code}`
  return te(key) ? t(key) : code
}

const worklogTypeItems = computed(() => {
  const list = worklogStats.value?.type_stats || []
  const total = list.reduce((sum, it) => sum + it.total_time_sec, 0)
  return list.map((it) => ({
    // work_type 存的是 code（dev / test / ops …），语言包里有内置映射。
    // 但工作类型在系统设置里可以自定义，自定义的 code 一定不在语言包里，
    // 此时 t() 会把 key 原样吐出来（页面上出现 "issue.workTypeMap.xxx"），
    // 所以先用 te() 判一下，没有就退回原始 code。
    name: workTypeLabel(it.work_type),
    value: Math.round(it.total_time_sec / 60),
    // 条上直接写「1d 7h」，和这一页别处的工时格式一致；裸分钟数读不出来
    valueText: formatWorklogTime(it.total_time_sec),
    ratio: total ? (it.total_time_sec / total) * 100 : 0 }))
})

// 高度按条数算，避免固定高度下少数几条被拉得过高、或十几条挤成一团
const chartHeight = (list: DistItem[] | undefined) => {
  const n = (list || []).length
  return Math.max(120, Math.min(n * 34 + 24, 420))
}

const getStatusColor = (status: string) => {
  // 状态是有语义的，走语义色而不是分类色
  // 全部走语义色。之前 pending_review 用 --td-cat-2、merged 用 --td-cat-6，
  // 那是给「无高低之分的分布维度」准备的分类色阶，状态是有语义的，
  // 混用会让同一屏里两套配色互相打架（见 CLAUDE.md 3.2）。
  //
  // 待处理与已终止都落到中性色：它们同为「不在处理中」。
  // 条形图每条都有文字直标，识别不依赖颜色，同色不会造成歧义。
  const map: Record<string, string> = {
    open: 'var(--td-color-info)',
    in_progress: 'var(--td-color-primary)',
    pending_review: 'var(--td-color-warning)',
    resolved: 'var(--td-color-success)',
    closed: 'var(--td-color-info)',
    merged: 'var(--td-color-info)' }
  return map[status] || 'var(--td-color-info)'
}

const getPriorityColor = (priority: string) => {
  const map: Record<string, string> = {
    P0: 'var(--td-color-danger)',
    P1: 'var(--td-color-warning)',
    P2: 'var(--td-color-primary)',
    P3: 'var(--td-color-info)' }
  return map[priority] || 'var(--td-color-info)'
}

const getPriorityType = (priority: string): TagType => {
  const map: Record<string, TagType> = { P0: 'danger', P1: 'warning', P2: 'primary', P3: 'info' }
  return map[priority] || 'info'
}

const getSeverityLabel = (severity: string) => {
  const map: Record<string, string> = { critical: t('alert.severityMap.critical'), warning: t('alert.severityMap.warning'), info: t('alert.severityMap.info') }
  return map[severity] || severity
}

const getSeverityColor = (severity: string) => {
  const map: Record<string, string> = {
    critical: 'var(--td-color-danger)',
    warning: 'var(--td-color-warning)',
    info: 'var(--td-color-info)' }
  return map[severity] || 'var(--td-color-info)'
}

const getSeverityTagType = (severity: string): TagType => {
  const map: Record<string, TagType> = { critical: 'danger', warning: 'warning', info: 'info' }
  return map[severity] || 'info'
}

// 用户绩效汇总（从数组计算）
const performanceSummary = computed(() => {
  const list = userPerformance.value
  const userCount = list.length
  const totalAssigned = list.reduce((s, u) => s + u.assigned, 0)
  const totalResolved = list.reduce((s, u) => s + u.resolved, 0)
  const avgResolveRate = totalAssigned > 0 ? (totalResolved / totalAssigned) * 100 : 0
  return { userCount, totalAssigned, totalResolved, avgResolveRate }
})

const getSLARateClass = (rate?: number) => {
  if (!rate) return ''
  if (rate >= 90) return 'success'
  if (rate >= 70) return 'warning'
  return 'danger'
}

// 工时格式化：秒 → 可读字符串
const formatWorklogTime = (seconds: number) => {
  if (!seconds || seconds <= 0) return '0h'
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (hours >= 8) {
    const days = Math.floor(hours / 8)
    const remainHours = hours % 8
    if (remainHours > 0) return `${days}d ${remainHours}h`
    return `${days}d`
  }
  if (hours > 0 && minutes > 0) return `${hours}h ${minutes}m`
  if (hours > 0) return `${hours}h`
  return `${minutes}m`
}

const formatShortDate = (date: string) => {
  if (!date) return ''
  return dayjs(date).format('MM/DD')
}

// 每日柱状图高度计算
const getDailyBarHeight = (timeSec: number) => {
  const maxTime = Math.max(...(worklogStats.value.daily_stats || []).map(d => d.total_time_sec), 1)
  return Math.max((timeSec / maxTime) * 100, 2)
}

const getUserPercentage = (timeSec: number) => {
  const total = worklogStats.value.summary?.total_time_sec || 1
  return Math.round((timeSec / total) * 1000) / 10
}

// ========== 工时明细网格 ==========

// 将后端 grid 数据展平为 el-table 可用的行（d_YYYY-MM-DD 作为动态列 prop）
const worklogGridData = computed(() => {
  return (worklogStats.value.grid || []).map((row) => {
    const flat: Record<string, unknown> = {
      user_id: row.user_id,
      display_name: row.display_name,
      _total: row.total_sec }
    for (const [date, sec] of Object.entries(row.daily || {})) {
      flat['d_' + date] = sec
    }
    for (const [date, details] of Object.entries(row.daily_details || {})) {
      flat['_details_' + date] = details
    }
    return flat
  })
})

// 网格日期列头格式
const formatGridDate = (date: string) => {
  const d = dayjs(date)
  const weekDay = t('report.weekDays').split(',')[d.day()]
  return d.format('MM/DD') + '\n' + weekDay
}

// 网格单元格显示小时数
const formatGridHours = (sec: number) => {
  if (!sec) return ''
  const h = sec / 3600
  if (h >= 1) return h % 1 === 0 ? h + 'h' : h.toFixed(1) + 'h'
  return Math.round(sec / 60) + 'm'
}

// 判断是否周末
const isWeekend = (date: string) => {
  const day = dayjs(date).day()
  return day === 0 || day === 6
}

// 合计行
const worklogGridSummary = ({ columns }: { columns: { property: string }[] }) => {
  return columns.map((col, idx) => {
    if (idx === 0) return t('report.grandTotal')
    const prop = col.property
    if (prop === '_total') {
      const total = (worklogStats.value.grid || []).reduce((s, r) => s + r.total_sec, 0)
      return formatGridHours(total)
    }
    if (prop?.startsWith('d_')) {
      const date = prop.slice(2)
      const val = worklogStats.value.grid_totals?.[date]
      return val ? formatGridHours(val) : ''
    }
    return ''
  })
}

// 周末列灰底（通过 class-name 控制，不再需要 inline style）
const worklogGridCellStyle = () => ({})

// 月份切换
const changeGridMonth = (delta: number) => {
  gridMonth.value = dayjs(gridMonth.value + '-01').add(delta, 'month').format('YYYY-MM')
  loadWorklogStats()
}

// 初始化
onMounted(() => {
  loadProjects()
  loadData()
})
</script>

<style scoped lang="scss">
// 页面骨架在 _apple.scss 里，这里只留报表页特有的排版。

.date-range { width: 260px; }

// 图表两栏
.stretch-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
  margin-bottom: 14px;
  /* 两列条数不一样时不要拉等高：告警页左边只有 2 条严重程度、右边 7 行趋势，
     等高会在左卡下面留 290px 空白。让卡片按自己的内容收，宁可底边参差。 */
  align-items: start;
}

@media (max-width: 1100px) {
  .stretch-row { grid-template-columns: 1fr; }
}

.tab-content {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

// 图表容器内边距：卡片本身不留边，图表自己给
.card :deep(.td-bar-chart),
.chart-body {
  padding: 14px 18px;
}

// 卡片里嵌的表格贴边（表头的发丝线要贯通整张卡）
.card :deep(.el-table) {
  border-radius: 0;
}

.empty { padding: 40px 0; }

/* ── 表格内的单元格排版 ─────────────────────────
   下面这些类在模板里一直有用，样式是上一轮重写时连同旧的页面骨架一起删掉的，
   结果是「用户」列里头像和名字竖排、每日工时的柱状图完全没有形状。 */
/* 不能叫 .user-cell —— .ap 里已经有一个全局的同名类，表示「名字在上、
   @username 在下」的两行单元格（用户管理在用）。同名会被那条压掉，
   头像和名字就竖起来了。 */
.report-user {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

/* 头像底色统一走中性淡底，身份靠字母颜色区分（personColor）。
   原来五个人全是同一个蓝底蓝字，头像等于没带任何信息。 */
.dist-avatar {
  flex-shrink: 0;
  background: var(--td-bg-section);
  font-size: 11px;
  font-weight: 600;
}

.project-key-badge {
  font-family: var(--td-font-mono);
  font-size: 11.5px;
  color: var(--td-text-secondary);
  background: var(--td-bg-section);
  border-radius: 5px;
  padding: 1px 6px;
  margin-right: 7px;
}

.project-name-text {
  font-size: 13px;
  color: var(--td-text-primary);
}

.issue-link,
.grid-popover-key {
  font-family: var(--td-font-mono);
  font-size: 12.5px;
  color: var(--td-color-primary);
  text-decoration: none;

  &:hover { text-decoration: underline; }
}

.alert-count-value,
.grid-cell-value,
.worklog-time-value {
  font-variant-numeric: tabular-nums;
  font-family: var(--td-font-mono);
  font-size: 12.5px;
}

.grid-total-value {
  font-variant-numeric: tabular-nums;
  font-family: var(--td-font-mono);
  font-size: 12.5px;
  font-weight: 590;
  color: var(--td-text-primary);
}

.text-danger { color: var(--td-color-danger); }

/* ── 工单趋势的三个计数 ─────────────────────────
   用淡底而不是实心：它们只是数字，不是状态（§3.1） */
.timeline-date {
  font-family: var(--td-font-mono);
  font-size: 12.5px;
  color: var(--td-text-secondary);
  font-variant-numeric: tabular-nums;
}

.timeline-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 22px;
  padding: 0 6px;
  height: 18px;
  border-radius: 5px;
  font-size: 11.5px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  background: var(--td-bg-section);
  color: var(--td-text-secondary);

  &.created { background: var(--td-tag-primary-bg); color: var(--td-tag-primary-text); }
  &.resolved { background: var(--td-tag-success-bg); color: var(--td-tag-success-text); }
  &.closed { background: var(--td-bg-section); color: var(--td-text-secondary); }
  &.in-progress { background: var(--td-tag-orange-bg); color: var(--td-tag-orange-text); }
}

/* ── 每日工时柱状图 ─────────────────────────────
   自绘的柱图，没有这几条就是一串没有形状的文字 */
.daily-worklog-chart {
  padding: 14px 18px;
}

.bar-chart {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  height: 160px;
  overflow-x: auto;
}

.bar-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-end;
  gap: 4px;
  flex: 1 0 34px;
  height: 100%;
  min-width: 34px;
}

.bar-value {
  font-size: 10.5px;
  color: var(--td-text-placeholder);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.bar-fill {
  /* 柱子撑满整列时是 95px 宽的饱和蓝色板砖，一屏里最重的东西就是它 ——
     暗色下尤其扎眼，而旁边「工作类型分布」用的是 10px 细条。收窄到读得出
     高度差、又不喧宾夺主的宽度。 */
  width: 100%;
  max-width: 30px;
  min-height: 2px;
  border-radius: 4px 4px 0 0;
  background: var(--td-color-primary);
}

.bar-label {
  font-size: 10.5px;
  color: var(--td-text-placeholder);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

/* ── 工时明细网格 ───────────────────────────────*/
.worklog-grid-wrap {
  overflow-x: auto;
}

.worklog-grid-table :deep(.el-table__cell) {
  padding: 4px 0;
}

.grid-month-picker {
  display: flex;
  align-items: center;
  gap: 2px;
}

.grid-month-label {
  font-size: 12.5px;
  color: var(--td-text-primary);
  font-variant-numeric: tabular-nums;
  min-width: 68px;
  text-align: center;
}

.grid-popover {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 240px;
}

.grid-popover-header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--td-divider-color);
  font-size: 12.5px;
  font-weight: 590;
  color: var(--td-text-primary);
}

.grid-popover-total {
  font-family: var(--td-font-mono);
  font-variant-numeric: tabular-nums;
  color: var(--td-text-secondary);
  font-weight: 500;
}

.grid-popover-item {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 12px;
}

.grid-popover-title {
  flex: 1;
  min-width: 0;
  color: var(--td-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.grid-popover-time {
  font-family: var(--td-font-mono);
  font-variant-numeric: tabular-nums;
  color: var(--td-text-placeholder);
  white-space: nowrap;
}

/* 排名徽章：前三名加重一档，其余弱色 */
.rank-badge {
  font-family: var(--td-font-mono);
  font-size: 12.5px;
  font-variant-numeric: tabular-nums;
  color: var(--td-text-placeholder);
}

.rank-badge.top-3 {
  color: var(--td-text-primary);
  font-weight: 590;
}

/* 工时网格里的周末列：底色压一档，一眼看出哪两列是周末 */
.worklog-grid-table :deep(.weekend-col) {
  background: var(--td-bg-section);
}

/* 表头 label 是 "09/04\n五"，但默认 white-space 把 \n 当普通空白，
   62px 宽里就在数字中间随便断，显示成「09/0 4 五」。按原文的换行符断行。 */
.worklog-grid-table :deep(thead .cell) {
  white-space: pre-line;
  line-height: 1.3;
  /* 默认 12px 左右内边距吃掉 62px 列宽的 24px，"09/04" 只剩 37px，差一点就断行。
     日期列本来就居中，内边距没用。 */
  padding-left: 2px;
  padding-right: 2px;
}
</style>
