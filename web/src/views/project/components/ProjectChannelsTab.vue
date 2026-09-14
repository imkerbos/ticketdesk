<template>
  <div class="project-channels">
    <!-- 外部渠道说明 -->
    <div class="notification-section-label">
      <span class="section-label-text">{{ t('project.settings.externalTitle') }}</span>
      <span class="section-label-desc">{{ t('project.settings.externalDesc') }}</span>
    </div>

    <!-- 渠道列表 -->
    <div v-if="channels.length > 0" class="channels-list">
      <div v-for="channel in channels" :key="channel.id" class="channel-row" :class="{ disabled: !channel.enabled }">
        <div class="channel-row-left">
          <div class="channel-icon" :class="channel.channel_type">
            <span v-if="channel.channel_type === 'lark'" class="channel-icon-text">{{ t('project.settings.lark') }}</span>
            <span v-else-if="channel.channel_type === 'telegram'" class="channel-icon-text">TG</span>
          </div>
          <div class="channel-row-info">
            <div class="channel-row-name">
              <span>{{ channel.name }}</span>
              <el-tag
                :type="channel.channel_type === 'lark' ? 'primary' : 'info'"
                size="small"
                effect="plain"
                class="channel-type-tag"
              >
                {{ channel.channel_type === 'lark' ? t('project.settings.larkBot') : 'Telegram Bot' }}
              </el-tag>
            </div>
            <div class="channel-row-detail">
              <span class="channel-config-text">
                <template v-if="channel.channel_type === 'lark'">
                  Webhook: {{ maskUrl((channel.config as any)?.webhook_url) }}
                </template>
                <template v-else-if="channel.channel_type === 'telegram'">
                  Chat ID: {{ (channel.config as any)?.chat_id || '-' }}
                </template>
              </span>
            </div>
            <div v-if="channel.event_types && channel.event_types.length > 0" class="channel-row-events">
              <el-tag
                v-for="ev in channel.event_types"
                :key="ev"
                size="small"
                type="info"
                effect="plain"
                class="channel-event-tag"
              >
                {{ t(getNotificationEventLabelKey(ev)) }}
              </el-tag>
            </div>
          </div>
        </div>
        <div class="channel-row-right">
          <span class="channel-status-indicator" :class="channel.enabled ? 'active' : 'inactive'">
            <span class="status-dot"></span>
            {{ channel.enabled ? t('project.settings.channelRunning') : t('project.settings.channelStopped') }}
          </span>
          <el-button
            size="small"
            :loading="testingChannelId === channel.id"
            @click="handleTestChannel(channel)"
          >
            <el-icon><Promotion /></el-icon>
            {{ t('project.settings.test') }}
          </el-button>
          <el-button size="small" @click="handleEditChannel(channel)">
            <el-icon><Edit /></el-icon>
            {{ t('common.edit') }}
          </el-button>
          <el-button size="small" type="danger" text class="is-destructive" @click="handleDeleteChannel(channel)">
            <el-icon><Delete /></el-icon>
            {{ t('common.delete') }}
          </el-button>
        </div>
      </div>
    </div>

    <!-- 空状态 -->
    <TdEmptyState
      v-if="!channelsLoading && channels.length === 0"
      preset="first-time"
      :title="t('project.settings.noChannelTitle')"
      :description="t('project.settings.noChannelDesc')"
    >
      <el-button type="primary" @click="openChannelDialog">
        <el-icon><Plus /></el-icon>
        {{ t('project.settings.addFirstChannel') }}
      </el-button>
    </TdEmptyState>

    <!-- 创建/编辑通知渠道对话框 -->
    <el-dialog
      v-model="channelDialogVisible"
      :title="isEditingChannel ? t('project.settings.channelDialogEdit') : t('project.settings.channelDialogAdd')"
      width="600px"
      destroy-on-close
      class="custom-dialog channel-dialog"
    >
      <el-form ref="channelFormRef" :model="channelForm" :rules="channelRules" label-position="top">
        <!-- 渠道类型选择 - 卡片式 -->
        <el-form-item :label="t('project.settings.channelType')" prop="channel_type" class="channel-type-form-item">
          <div class="channel-type-cards" :class="{ disabled: isEditingChannel }">
            <div
              class="channel-type-card"
              :class="{ active: channelForm.channel_type === 'lark', disabled: isEditingChannel }"
              @click="!isEditingChannel && (channelForm.channel_type = 'lark')"
            >
              <div class="type-card-icon lark">
                <span>{{ t('project.settings.lark') }}</span>
              </div>
              <div class="type-card-content">
                <div class="type-card-name">{{ t('project.settings.larkBot') }}</div>
                <div class="type-card-desc">{{ t('project.settings.larkDesc') }}</div>
              </div>
              <div v-if="channelForm.channel_type === 'lark'" class="type-card-check">
                <el-icon><CircleCheck /></el-icon>
              </div>
            </div>
            <div
              class="channel-type-card"
              :class="{ active: channelForm.channel_type === 'telegram', disabled: isEditingChannel }"
              @click="!isEditingChannel && (channelForm.channel_type = 'telegram')"
            >
              <div class="type-card-icon telegram">
                <span>TG</span>
              </div>
              <div class="type-card-content">
                <div class="type-card-name">Telegram Bot</div>
                <div class="type-card-desc">{{ t('project.settings.telegramDesc') }}</div>
              </div>
              <div v-if="channelForm.channel_type === 'telegram'" class="type-card-check">
                <el-icon><CircleCheck /></el-icon>
              </div>
            </div>
          </div>
        </el-form-item>

        <!-- 基本信息 -->
        <div class="dialog-section">
          <div class="dialog-section-title">{{ t('project.settings.tabBasic') }}</div>
          <el-form-item :label="t('project.settings.channelName')" prop="name">
            <el-input v-model="channelForm.name" :placeholder="t('project.settings.channelNamePlaceholder')" maxlength="100" show-word-limit>
              <template #prefix>
                <el-icon><Bell /></el-icon>
              </template>
            </el-input>
          </el-form-item>
          <el-form-item :label="t('project.settings.channelEnabled')">
            <div class="enable-switch-row">
              <el-switch
                v-model="channelForm.enabled"
                active-color="var(--td-color-success)"
              />
              <span class="enable-switch-label" :class="{ active: channelForm.enabled }">
                {{ channelForm.enabled ? t('project.settings.channelEnabledOn') : t('project.settings.channelEnabledOff') }}
              </span>
            </div>
          </el-form-item>
          <el-form-item :label="t('project.settings.subscribedEvents')" prop="event_types">
            <div class="event-types-wrapper">
              <el-checkbox-group v-model="channelForm.event_types" class="event-types-group">
                <el-checkbox
                  v-for="opt in NOTIFICATION_EVENT_OPTIONS"
                  :key="opt.value"
                  :value="opt.value"
                  class="event-type-checkbox"
                >
                  <span class="event-type-label">{{ t(opt.labelKey) }}</span>
                  <span class="event-type-desc">{{ t(opt.descKey) }}</span>
                </el-checkbox>
              </el-checkbox-group>
              <div class="form-tip-small">
                <el-icon><InfoFilled /></el-icon>
                <span>{{ t('project.settings.eventsTip') }}</span>
              </div>
            </div>
          </el-form-item>
          <el-form-item v-if="channelForm.channel_type === 'lark'" :label="t('project.settings.mentionAll')">
            <div class="enable-switch-row">
              <el-switch
                v-model="channelForm.mention_all"
                active-color="var(--td-color-success)"
              />
              <span class="enable-switch-label" :class="{ active: channelForm.mention_all }">
                {{ channelForm.mention_all ? t('project.settings.mentionAllOn') : t('project.settings.mentionAllOff') }}
              </span>
            </div>
            <div class="form-tip-small">
              <el-icon><InfoFilled /></el-icon>
              <span>{{ t('project.settings.mentionAllTip') }}</span>
            </div>
          </el-form-item>
        </div>

        <!-- 飞书配置 -->
        <div v-if="channelForm.channel_type === 'lark'" class="dialog-section">
          <div class="dialog-section-title">
            <span class="section-title-icon lark">{{ t('project.settings.lark') }}</span>
            {{ t('project.settings.connConfig') }}
          </div>
          <el-form-item label="Webhook URL" prop="lark_webhook_url">
            <el-input v-model="channelForm.lark_webhook_url" placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/xxx">
              <template #prefix>
                <el-icon><Connection /></el-icon>
              </template>
            </el-input>
            <div class="form-tip-small">
              <el-icon><InfoFilled /></el-icon>
              <span>{{ t('project.settings.larkWebhookTip') }}</span>
            </div>
          </el-form-item>
          <el-form-item :label="t('project.settings.larkSecret')">
            <el-input v-model="channelForm.lark_secret" :placeholder="t('project.settings.larkSecretPlaceholder')" show-password>
              <template #prefix>
                <el-icon><Key /></el-icon>
              </template>
            </el-input>
            <div class="form-tip-small">
              <el-icon><InfoFilled /></el-icon>
              <span>{{ t('project.settings.larkSecretTip') }}</span>
            </div>
          </el-form-item>
        </div>

        <!-- Telegram 配置 -->
        <div v-if="channelForm.channel_type === 'telegram'" class="dialog-section">
          <div class="dialog-section-title">
            <span class="section-title-icon telegram">TG</span>
            {{ t('project.settings.connConfig') }}
          </div>
          <el-form-item label="Bot Token" prop="telegram_bot_token">
            <el-input v-model="channelForm.telegram_bot_token" :placeholder="t('project.settings.tgTokenPlaceholder')" show-password>
              <template #prefix>
                <el-icon><Key /></el-icon>
              </template>
            </el-input>
            <div class="form-tip-small">
              <el-icon><InfoFilled /></el-icon>
              <span>{{ t('project.settings.tgTokenTip') }}</span>
            </div>
          </el-form-item>
          <el-form-item label="Chat ID" prop="telegram_chat_id">
            <el-input v-model="channelForm.telegram_chat_id" :placeholder="t('project.settings.tgChatPlaceholder')">
              <template #prefix>
                <el-icon><Promotion /></el-icon>
              </template>
            </el-input>
            <div class="form-tip-small">
              <el-icon><InfoFilled /></el-icon>
              <span>{{ t('project.settings.tgChatTip') }}</span>
            </div>
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button size="large" @click="channelDialogVisible = false">{{ t('common.cancel') }}</el-button>
          <el-button type="primary" :loading="channelSubmitLoading" size="large" @click="submitChannel">
            <el-icon><Check /></el-icon>
            {{ isEditingChannel ? t('project.settings.saveChanges') : t('project.settings.addChannel') }}
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
/**
 * 项目设置 - 外部通知渠道（飞书 / Telegram）。
 *
 * 从 ProjectSettings.vue 拆出：渠道配置是一个自成一体的集成功能
 * （模板 + 增删改查 + 样式近 700 行），和项目本身的配置没有共享状态，
 * 只需要知道当前是哪个项目。增删改查在这里闭环，不往上抛事件。
 *
 * 同一个 tab 里的「每日日报」没有一起搬：它保存时要回写项目详情
 * （loadProjectDetail），状态在父组件，搬过来只会把耦合换个地方。
 */
import { reactive, ref, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus, Edit, Delete, Connection, InfoFilled } from '@element-plus/icons-vue'
import {
  getNotificationChannels, createNotificationChannel, updateNotificationChannel,
  deleteNotificationChannel, testNotificationChannel,
} from '@/api/project'
import type { NotificationChannel, NotificationEventType } from '@/types/project'
import { NOTIFICATION_EVENT_OPTIONS, DEFAULT_NOTIFICATION_EVENTS, getNotificationEventLabelKey } from '@/constants/notification'

const { t } = useI18n()

const props = defineProps<{ projectKey: string }>()


const channelsLoading = ref(false)

const channels = ref<NotificationChannel[]>([])

const channelDialogVisible = ref(false)

const isEditingChannel = ref(false)

const editingChannelId = ref<number | null>(null)

const channelSubmitLoading = ref(false)

const testingChannelId = ref<number | null>(null)

const channelFormRef = ref<FormInstance>()

const channelForm = reactive({
  channel_type: 'lark' as 'lark' | 'telegram',
  name: '',
  enabled: true,
  mention_all: false,
  event_types: [...DEFAULT_NOTIFICATION_EVENTS] as NotificationEventType[],
  lark_webhook_url: '',
  lark_secret: '',
  telegram_bot_token: '',
  telegram_chat_id: '',
})

const channelRules: FormRules = {
  channel_type: [{ required: true, message: t('project.settings.channelTypeRequired'), trigger: 'change' }],
  name: [{ required: true, message: t('project.settings.channelNameRequired'), trigger: ['blur', 'change'] }],
  event_types: [
    {
      type: 'array',
      required: true,
      message: t('project.settings.eventsRequired'),
      trigger: 'change',
      validator: (_rule, value, callback) => {
        if (!Array.isArray(value) || value.length === 0) {
          callback(new Error(t('project.settings.eventsRequired')))
          return
        }
        callback()
      },
    },
  ],
  lark_webhook_url: [{ required: true, message: t('project.settings.larkWebhookRequired'), trigger: ['blur', 'change'] }],
  telegram_bot_token: [{ required: true, message: t('project.settings.tgTokenRequired'), trigger: ['blur', 'change'] }],
  telegram_chat_id: [{ required: true, message: t('project.settings.tgChatRequired'), trigger: ['blur', 'change'] }],
}

const loadChannels = async () => {
  channelsLoading.value = true
  try {
    const { data } = await getNotificationChannels(props.projectKey)
    channels.value = data.data || []
  } catch {
    // ignored
  } finally {
    channelsLoading.value = false
  }
}

const openChannelDialog = () => {
  isEditingChannel.value = false
  editingChannelId.value = null
  Object.assign(channelForm, {
    channel_type: 'lark',
    name: '',
    enabled: true,
    mention_all: false,
    event_types: [...DEFAULT_NOTIFICATION_EVENTS],
    lark_webhook_url: '',
    lark_secret: '',
    telegram_bot_token: '',
    telegram_chat_id: '',
  })
  channelDialogVisible.value = true
}

const handleEditChannel = (channel: NotificationChannel) => {
  isEditingChannel.value = true
  editingChannelId.value = channel.id
  const config = channel.config as any
  const existing = Array.isArray(channel.event_types) && channel.event_types.length > 0
    ? [...channel.event_types]
    : [...DEFAULT_NOTIFICATION_EVENTS]
  Object.assign(channelForm, {
    channel_type: channel.channel_type,
    name: channel.name,
    enabled: channel.enabled,
    mention_all: channel.mention_all === true,
    event_types: existing,
    lark_webhook_url: channel.channel_type === 'lark' ? (config?.webhook_url || '') : '',
    lark_secret: '', // 密钥不回显
    telegram_bot_token: '', // Token 不回显
    telegram_chat_id: channel.channel_type === 'telegram' ? (config?.chat_id || '') : '',
  })
  channelDialogVisible.value = true
}

const handleDeleteChannel = async (channel: NotificationChannel) => {
  try {
    await ElMessageBox.confirm(t('project.settings.confirmDeleteChannel', { name: channel.name }), t('issue.list.deleteTitle'), {
      type: 'warning',
    })
    await deleteNotificationChannel(props.projectKey, channel.id)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    loadChannels()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('issue.msg.deleteFailed2'))
    }
  }
}

const handleTestChannel = async (channel: NotificationChannel) => {
  testingChannelId.value = channel.id
  try {
    await testNotificationChannel(props.projectKey, channel.id)
    ElMessage.success(t('project.settings.testSent'))
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || t('project.settings.testFailed'))
  } finally {
    testingChannelId.value = null
  }
}

const submitChannel = async () => {
  if (!channelFormRef.value) return
  await channelFormRef.value.validate(async (valid) => {
    if (!valid) return
    channelSubmitLoading.value = true
    try {
      // 构建 config
      let config: any = {}
      if (channelForm.channel_type === 'lark') {
        config = {
          webhook_url: channelForm.lark_webhook_url,
        }
        if (channelForm.lark_secret) {
          config.secret = channelForm.lark_secret
        }
      } else if (channelForm.channel_type === 'telegram') {
        config = {
          chat_id: channelForm.telegram_chat_id,
        }
        if (channelForm.telegram_bot_token) {
          config.bot_token = channelForm.telegram_bot_token
        }
      }

      if (isEditingChannel.value && editingChannelId.value) {
        const updateData: any = {
          name: channelForm.name,
          enabled: channelForm.enabled,
          event_types: channelForm.event_types,
          mention_all: channelForm.mention_all,
        }
        // 只有填写了配置才更新 config
        const hasConfig = channelForm.channel_type === 'lark'
          ? channelForm.lark_webhook_url
          : channelForm.telegram_chat_id
        if (hasConfig) {
          updateData.config = config
        }
        await updateNotificationChannel(props.projectKey, editingChannelId.value, updateData)
        ElMessage.success(t('issue.msg.updateSuccess'))
      } else {
        await createNotificationChannel(props.projectKey, {
          channel_type: channelForm.channel_type,
          name: channelForm.name,
          config,
          event_types: channelForm.event_types,
          mention_all: channelForm.mention_all,
          enabled: channelForm.enabled,
        })
        ElMessage.success(t('common.createSuccess'))
      }
      channelDialogVisible.value = false
      loadChannels()
    } catch (error: any) {
      ElMessage.error(error?.response?.data?.message || (isEditingChannel.value ? t('project.settings.updateFailed') : t('project.settings.createFailed')))
    } finally {
      channelSubmitLoading.value = false
    }
  })
}

const maskUrl = (url: string | undefined) => {
  if (!url) return '-'
  try {
    const u = new URL(url)
    const path = u.pathname
    if (path.length > 20) {
      return u.origin + path.substring(0, 10) + '...' + path.substring(path.length - 6)
    }
    return url
  } catch {
    if (url.length > 30) {
      return url.substring(0, 15) + '...' + url.substring(url.length - 10)
    }
    return url
  }
}

// 计数和「新增渠道」按钮在 tab 顶部的区块头里，位置在日报配置之上。
// 那块头属于渠道，但搬进来会改变视觉顺序，所以把数据抛上去、
// 把开弹窗的方法暴露出去，让父组件的头继续用。
const emit = defineEmits<{ (e: 'update:count', n: number): void }>()
watch(channels, () => emit('update:count', channels.value.length), { deep: true })
defineExpose({ openDialog: () => openChannelDialog() })

onMounted(loadChannels)
</script>

<style scoped lang="scss">
@use '../project-settings-shared.scss' as *;

.notification-section-label {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 12px;
  padding-bottom: 9px;
  border-bottom: 1px solid var(--td-divider-color);

  .section-label-text {
    font-size: 13.5px;
    font-weight: 590;
    letter-spacing: -0.01em;
    color: var(--td-text-primary);
  }

  .section-label-desc {
    font-size: 11.5px;
    color: var(--td-text-secondary);
  }
}

/* ── 渠道列表 ─────────────────────────────────── */
.channels-list {
  display: flex;
  flex-direction: column;
  background: var(--td-bg-card);
  border: 1px solid var(--td-border-color);
  border-radius: 10px;
  overflow: hidden;
}

.channel-row {
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

  &.disabled .channel-row-left {
    opacity: 0.55;
  }
}

.channel-row-left {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

/* 飞书 / Telegram 的品牌色标：它代表「这是哪个平台」，属于数据而不是装饰，
   所以保留色块也保留原始色值（不随明暗主题变），但尺寸从 40px 收到 24px，
   不再是一屏里最重的那个东西。 */
.channel-icon {
  width: 24px;
  height: 24px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: #fff;

  &.lark {
    background: #3370ff;
  }

  &.telegram {
    background: #2aabee;
  }
}

.channel-icon-text {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.channel-row-info {
  flex: 1;
  min-width: 0;
}

.channel-row-name {
  display: flex;
  align-items: center;
  gap: 7px;

  > span {
    font-size: 13px;
    font-weight: 590;
    letter-spacing: -0.01em;
    color: var(--td-text-primary);
  }

  .channel-type-tag {
    font-size: 11px;
  }
}

.channel-row-detail {
  margin-top: 2px;

  .channel-config-text {
    font-size: 11.5px;
    color: var(--td-text-placeholder);
    font-family: var(--td-font-mono);
  }
}

.channel-row-events {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 5px;

  .channel-event-tag {
    font-size: 11px;
  }
}

.channel-row-right {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  margin-left: 14px;
}

/* 状态药丸：淡底 + 深色文字 + 圆点，不发光（§3.1 禁止发光） */
.channel-status-indicator {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11.5px;
  padding: 1px 8px 1px 7px;
  border-radius: 5px;
  margin-right: 6px;

  .status-dot {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  &.active {
    color: var(--td-tag-success-text);
    background: var(--td-tag-success-bg);

    .status-dot {
      background: var(--td-color-success);
    }
  }

  &.inactive {
    color: var(--td-text-secondary);
    background: var(--td-bg-section);

    .status-dot {
      background: var(--td-text-placeholder);
    }
  }
}

/* 行内破坏性操作悬停才显形，列宽不跳动；触屏下常显 */
.is-destructive {
  opacity: 0;
  transition: opacity 150ms ease-out;
}

@media (hover: none) {
  .is-destructive {
    opacity: 1;
  }
}

/* ── 渠道类型选择卡 ───────────────────────────── */
.channel-type-form-item {
  :deep(.el-form-item__label) {
    font-weight: 500;
    font-size: 12.5px;
  }
}

.channel-type-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  width: 100%;

  &.disabled {
    pointer-events: none;
  }
}

.channel-type-card {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--td-border-color);
  border-radius: 10px;
  cursor: pointer;
  background: var(--td-bg-card);
  transition: border-color 150ms ease-out, background-color 150ms ease-out;

  &:hover:not(.disabled) {
    border-color: var(--td-border-color-dark);
    background: var(--td-bg-section);
  }

  &.active {
    border-color: var(--td-color-primary);
    box-shadow: var(--td-focus-ring);
  }

  &.disabled {
    opacity: 0.7;
    cursor: not-allowed;
  }

  .type-card-icon {
    width: 30px;
    height: 30px;
    border-radius: 7px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #fff;
    font-size: 11px;
    font-weight: 700;
    flex-shrink: 0;

    &.lark {
      background: #3370ff;
    }

    &.telegram {
      background: #2aabee;
    }
  }

  .type-card-content {
    flex: 1;
    min-width: 0;

    .type-card-name {
      font-size: 13px;
      font-weight: 590;
      letter-spacing: -0.01em;
      color: var(--td-text-primary);
    }

    .type-card-desc {
      font-size: 11px;
      color: var(--td-text-secondary);
      line-height: 1.45;
      margin-top: 2px;
    }
  }

  .type-card-check {
    position: absolute;
    top: 7px;
    right: 7px;
    color: var(--td-color-primary);
    font-size: 15px;
  }
}
</style>
