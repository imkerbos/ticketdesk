# TicketDesk 业务术语表（中 → 英）

出英文版时的**唯一译法基准**。前端语言包、后端消息表、API 文档、邮件模板一律以此为准。

新增术语必须先加进本表再使用；同一概念出现两种译法就是 bug。

---

## 核心实体

| 中文 | English | 说明 |
|---|---|---|
| 工单 | **Issue** | 不用 Ticket。产品定位是「一切问题都是工单」，Issue 更贴近 Jira 语境，也与 `issue_key` / `/issues` 路由一致 |
| 工单号 | Issue key | 形如 `OPS-1`，不译作 ID |
| 项目 | Project | |
| 需求池 | Requirement pool | |
| 需求 | Requirement | |
| 告警 | **Alert** | |
| 告警规则 | Alert rule | 自动建单规则 |
| 告警静默 | **Silence** | 沿用 Alertmanager 术语，不用 Mute |
| 数据源 | Data source | Prometheus / 夜莺等 |
| 工作流 | Workflow | |
| 工作流实例 | Workflow instance | |
| 审批 | Approval | |
| 工作节点 | Work node | |
| 审批节点 | Approval node | |
| 字段方案 | Field scheme | |
| 工时 | **Worklog** | 不用 Timesheet；对应 `issue_worklogs` 表 |
| 关注人 | **Watcher** | 不用 Follower |
| 指派人 | **Assignee** | |
| 报告人 | **Reporter** | |
| 项目成员 | Project member | |
| 项目角色 | Project role | |

## 状态与枚举

工单状态（`issues.status`）：

| 中文 | English | 值 |
|---|---|---|
| 待处理 | Open | `open` |
| 进行中 | In progress | `in_progress` |
| 待确认 | In review | `pending_review` |
| 已完成 | Resolved | `resolved` |
| 已终止 | Closed | `closed` |
| 重新打开 | Reopened | `reopened` |
| 已合并 | Merged | `merged` |

> 「已完成 = Resolved」「已终止 = Closed」这个映射刻意区分：
> Resolved 表示问题解决了，Closed 表示流程终止（可能并未解决）。

优先级：`P0 - Critical` / `P1 - High` / `P2 - Medium` / `P3 - Low`

告警严重程度（`alerts.severity`）：`Critical` / `Warning` / `Info`

告警状态：`Firing` / `Resolved` / `Acknowledged`（确认 = Acknowledge，动词 Ack）

解决结果（`issues.resolution`）：

| 中文 | English |
|---|---|
| 已修复 | Fixed |
| 不予修复 | Won't fix |
| 重复 | Duplicate |
| 无法复现 | Cannot reproduce |
| 设计如此 | Works as designed |
| 信息不足 | Incomplete |
| 完成 | Done |

## 动作

| 中文 | English |
|---|---|
| 创建 | Create |
| 编辑 | Edit |
| 删除 | Delete |
| 移除 | Remove（从集合中移出，对象仍存在） |
| 指派 | Assign |
| 认领 / 分配给我 | Assign to me |
| 关注 / 取消关注 | Watch / Unwatch |
| 确认（告警） | Acknowledge |
| 静默 | Silence |
| 合并 | Merge |
| 流转 | Transition |
| 通过 / 拒绝（审批） | Approve / Reject |
| 保存视图 | Save view |
| 重置 | Reset |

> Delete 与 Remove 不可互换：删除工单是 Delete，把成员移出项目是 Remove。

## 系统与配置

| 中文 | English |
|---|---|
| 系统设置 | Settings |
| 品牌设置 | Branding |
| 通用配置 | General |
| 邮件配置 | Email |
| 安全设置 | Security |
| 限流配置 | Rate limiting |
| SSO 认证 | Single sign-on |
| 两步验证 | Two-factor authentication（缩写 2FA，不用 MFA 面向用户） |
| 访问令牌 | Access token |
| 个人访问令牌 | Personal access token |
| 通知渠道 | Notification channel |
| 日报 | Daily digest |
| 站内通知 | In-app notification |

## 语气约定

- 按钮用祈使式动词原形：`Save`，不用 `Saving` / `Save it`
- 错误信息说明**发生了什么 + 怎么办**，不道歉、不含糊：
  - ✅ `Issue not found. It may have been deleted or merged.`
  - ❌ `Sorry, something went wrong.`
- 面向用户的文案用用户认知的词，不用系统实现词：
  - ✅ `Notifications`　❌ `Webhook config`
- 空状态说明"这里会出现什么"，而不只是"没有数据"
