<template>
  <div class="page">
    <div class="page-head">
      <h1>{{ t('nav.profile') }}</h1>
    </div>

    <div class="grid-2 profile-grid">
      <!-- 左：我是谁 -->
      <div class="col">
        <section class="card profile-card">
          <div class="profile-header">
            <div class="avatar-wrapper">
              <el-avatar :size="100" class="user-avatar">
                {{ profile?.display_name?.charAt(0) || profile?.username?.charAt(0) || '?' }}
              </el-avatar>
              <div class="avatar-overlay">
                <el-icon><Camera /></el-icon>
              </div>
            </div>
            <h2 class="username">{{ profile?.display_name || profile?.username }}</h2>
            <p class="user-role">
              <!-- 管理员不是危险状态，红色留给真正的破坏性/错误语义（§3.1） -->
              <el-tag :type="profile?.roles?.includes('admin') ? 'primary' : 'info'" size="small" effect="plain">
                {{ profile?.roles?.includes('admin') ? t('user.roleAdmin') : t('user.profile.normalUser') }}
              </el-tag>
            </p>
          </div>

          <el-divider />

          <!--
            左卡片只留"我是谁"：邮箱可以在右侧「基本信息」里直接改，
            认证方式在「账户安全 → 登录密码」那一行已经说明（SSO 托管 / 已设置），
            在这里再只读地列一遍，是把一屏高度花在重复上。
          -->
          <div class="profile-info">
            <div class="info-item">
              <el-icon><User /></el-icon>
              <div class="info-content">
                <span class="info-label">{{ t('user.profile.username') }}</span>
                <span class="info-value">{{ profile?.username }}</span>
              </div>
            </div>
            <div class="info-item">
              <el-icon><Calendar /></el-icon>
              <div class="info-content">
                <span class="info-label">{{ t('user.profile.registeredAt') }}</span>
                <span class="info-value num">{{ formatDate(profile?.created_at) }}</span>
              </div>
            </div>
          </div>
        </section>
      </div>

      <!-- 右：设置表单 -->
      <div class="col">
        <!-- 基本信息 -->
        <section class="card">
          <div class="card-head">
            <h2>{{ t('user.profile.basic') }}</h2>
          </div>

          <el-form
            ref="profileFormRef"
            :model="profileForm"
            :rules="profileRules"
            label-width="100px"
            class="settings-form"
          >
            <el-form-item :label="t('user.displayName')" prop="display_name">
              <el-input
                v-model="profileForm.display_name"
                :placeholder="t('user.profile.displayNamePlaceholder')"
                maxlength="50"
                show-word-limit
              />
            </el-form-item>
            <el-form-item :label="t('user.profile.emailLabel')" prop="email">
              <el-input
                v-model="profileForm.email"
                :placeholder="t('user.profile.emailPlaceholder')"
                type="email"
              />
            </el-form-item>
            <el-form-item :label="t('user.profile.larkOpenId')" prop="lark_open_id">
              <el-input
                v-model="profileForm.lark_open_id"
                :placeholder="t('user.profile.larkPlaceholder')"
                maxlength="64"
                clearable
              />
              <div class="form-tip-inline">
                {{ t('user.profile.larkTip') }}
              </div>
            </el-form-item>
            <el-form-item label="Telegram ID" prop="telegram_user_id">
              <el-input
                v-model="profileForm.telegram_user_id"
                :placeholder="t('user.profile.telegramPlaceholder')"
                maxlength="32"
                clearable
              />
              <div class="form-tip-inline">
                {{ t('user.profile.telegramTipPrefix') }}
                <a href="https://t.me/userinfobot" target="_blank" rel="noopener">@userinfobot</a>
                {{ t('user.profile.telegramTipSuffix') }}
              </div>
            </el-form-item>
            <el-form-item :label="t('user.profile.language')" prop="locale">
              <el-select
                v-model="profileForm.locale"
                :placeholder="t('user.profile.languageFollowSite')"
                style="width: 100%"
              >
                <el-option :label="t('user.profile.languageFollowSite')" value="" />
                <el-option :label="t('lang.zh-CN')" value="zh-CN" />
                <el-option :label="t('lang.en-US')" value="en-US" />
              </el-select>
              <div class="form-tip-inline">
                {{ t('user.profile.languageTip') }}
              </div>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="profileLoading" @click="submitProfile">
                {{ t('user.profile.saveChanges') }}
              </el-button>
            </el-form-item>
          </el-form>
        </section>

        <!-- 账户安全 -->
        <section class="card">
          <div class="card-head">
            <h2>{{ t('user.profile.security') }}</h2>
          </div>

          <div class="security-info">
            <div class="security-item">
              <div class="security-left">
                <el-icon class="security-icon" :class="profile?.auth_source === 'sso' ? 'warning' : 'success'">
                  <component :is="profile?.auth_source === 'sso' ? Warning : CircleCheck" />
                </el-icon>
                <div class="security-content">
                  <span class="security-title">{{ t('user.profile.loginPassword') }}</span>
                  <span class="security-desc">
                    {{ profile?.auth_source === 'sso' ? t('user.profile.ssoManaged') : t('user.profile.passwordSet') }}
                  </span>
                </div>
              </div>
              <el-button v-if="profile?.auth_source !== 'sso'" link type="primary" @click="openPasswordDialog">{{ t('user.profile.modify') }}</el-button>
            </div>

            <el-divider />

            <div class="security-item">
              <div class="security-left">
                <el-icon class="security-icon" :class="profile?.email ? 'success' : 'warning'">
                  <component :is="profile?.email ? CircleCheck : Warning" />
                </el-icon>
                <div class="security-content">
                  <span class="security-title">{{ t('user.profile.emailBinding') }}</span>
                  <span class="security-desc">
                    {{ profile?.email ? t('user.profile.emailBound', { email: profile.email }) : t('user.profile.emailUnbound') }}
                  </span>
                </div>
              </div>
              <el-button link type="primary" @click="scrollToProfile">
                {{ profile?.email ? t('user.profile.modify') : t('user.profile.bind') }}
              </el-button>
            </div>

            <el-divider />

            <div class="security-item">
              <div class="security-left">
                <el-icon class="security-icon" :class="mfaStatus?.enabled ? 'success' : 'warning'">
                  <component :is="mfaStatus?.enabled ? CircleCheck : Warning" />
                </el-icon>
                <div class="security-content">
                  <span class="security-title">{{ t('user.profile.mfa') }}</span>
                  <span class="security-desc">
                    {{ mfaStatus?.enabled ? t('user.profile.mfaOn') : t('user.profile.mfaOff') }}
                  </span>
                </div>
              </div>
              <el-button link type="primary" @click="mfaStatus?.enabled ? showDisableMFADialog() : startMFASetup()">
                {{ mfaStatus?.enabled ? t('user.disable') : t('user.enable') }}
              </el-button>
            </div>
          </div>
        </section>

        <!-- API 密钥管理 -->
        <ApiTokenSection />
      </div>
    </div>

    <!-- MFA 设置对话框 -->
    <!--
      改密码从常驻表单改成弹窗：它和「账户安全 → 登录密码 → 修改」本来就是
      同一件事的两个入口（那个"修改"只是滚动到这张表单），
      而一个常年空着的三行密码表单会一直占着首屏。
    -->
    <el-dialog v-model="passwordDialogVisible" :title="t('user.profile.changePassword')" width="460px" @closed="resetPasswordForm">
      <el-form
        ref="passwordFormRef"
        :model="passwordForm"
        :rules="passwordRules"
        label-position="top"
      >
        <el-form-item :label="t('user.profile.currentPassword')" prop="old_password">
          <el-input
            v-model="passwordForm.old_password"
            type="password"
            :placeholder="t('user.profile.currentPasswordPlaceholder')"
            show-password
          />
        </el-form-item>
        <el-form-item :label="t('user.newPassword')" prop="new_password">
          <el-input
            v-model="passwordForm.new_password"
            type="password"
            :placeholder="t('user.profile.newPasswordPlaceholder')"
            show-password
          />
        </el-form-item>
        <el-form-item :label="t('user.confirmPassword')" prop="confirm_password">
          <el-input
            v-model="passwordForm.confirm_password"
            type="password"
            :placeholder="t('user.profile.confirmPasswordPlaceholder')"
            show-password
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="passwordLoading" @click="submitPassword">
          {{ t('user.profile.changePassword') }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="mfaSetupDialogVisible" :title="t('user.profile.mfaSetupTitle')" width="500px" :close-on-click-modal="false">
      <div class="mfa-setup">
        <div class="mfa-step">
          <h4>{{ t('user.profile.mfaStep1') }}</h4>
          <p>{{ t('user.profile.mfaStep1Desc') }}</p>
        </div>

        <div class="mfa-step">
          <h4>{{ t('user.profile.mfaStep2') }}</h4>
          <p>{{ t('user.profile.mfaStep2Desc') }}</p>
          <div v-if="mfaSetupData" class="qr-code">
            <img :src="qrCodeUrl" alt="MFA QR Code" />
          </div>
          <p v-if="mfaSetupData" class="manual-key">
            {{ t('user.profile.mfaSecret') }}<code>{{ mfaSetupData.secret }}</code>
          </p>
        </div>

        <div class="mfa-step">
          <h4>{{ t('user.profile.mfaStep3') }}</h4>
          <p>{{ t('user.profile.mfaStep3Desc') }}</p>
          <el-input
            v-model="mfaVerifyCode"
            placeholder="000000"
            maxlength="6"
            class="verify-code-input"
            @keyup.enter="confirmEnableMFA"
          />
        </div>
      </div>

      <template #footer>
        <el-button @click="mfaSetupDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="mfaEnabling" @click="confirmEnableMFA">
          {{ t('user.profile.mfaEnable') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- MFA 禁用对话框 -->
    <el-dialog v-model="mfaDisableDialogVisible" :title="t('user.profile.mfaDisableTitle')" width="400px">
      <p>{{ t('user.profile.mfaDisableDesc') }}</p>
      <el-input
        v-model="mfaDisableCode"
        placeholder="000000"
        maxlength="6"
        class="verify-code-input"
        @keyup.enter="confirmDisableMFA"
      />
      <template #footer>
        <el-button @click="mfaDisableDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="danger" :loading="mfaDisabling" @click="confirmDisableMFA">
          {{ t('user.profile.mfaDisable') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { applyAccountLocale } from '@/i18n'
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import {
  User,
  Calendar,
  Camera,
  CircleCheck,
  Warning } from '@element-plus/icons-vue'
import { getCurrentUser, updateCurrentUser, updatePassword, getMFAStatus, setupMFA, enableMFA, disableMFA } from '@/api/user'
import type { UserProfile, UpdatePasswordRequest } from '@/types/user'
import type { MFAStatusResponse, MFASetupResponse } from '@/api/user'
import dayjs from 'dayjs'
import ApiTokenSection from './components/ApiTokenSection.vue'

const { t } = useI18n()

// 用户信息
const profile = ref<UserProfile & { created_at?: string } | null>(null)

// 基本信息表单
const profileFormRef = ref<FormInstance>()
const profileLoading = ref(false)
const profileForm = reactive({
  display_name: '',
  email: '',
  lark_open_id: '',
  telegram_user_id: '',
  locale: '' })

const profileRules: FormRules = {
  display_name: [
    { required: true, message: t('user.displayNameRequired'), trigger: ['blur', 'change'] },
    { max: 50, message: t('user.profile.displayNameMax'), trigger: 'blur' },
  ],
  email: [
    { type: 'email', message: t('user.profile.emailInvalid'), trigger: 'blur' },
  ],
  lark_open_id: [
    { max: 64, message: t('user.profile.larkMax'), trigger: 'blur' },
  ],
  telegram_user_id: [
    { pattern: /^\d*$/, message: t('user.profile.telegramDigits'), trigger: 'blur' },
    { max: 32, message: t('user.profile.telegramMax'), trigger: 'blur' },
  ] }

// 密码表单
const passwordFormRef = ref<FormInstance>()
const passwordDialogVisible = ref(false)
const passwordLoading = ref(false)
const passwordForm = reactive({
  old_password: '',
  new_password: '',
  confirm_password: '' })

const validateConfirmPassword = (_rule: unknown, value: string, callback: (error?: Error) => void) => {
  if (value !== passwordForm.new_password) {
    callback(new Error(t('user.passwordMismatch')))
  } else {
    callback()
  }
}

const passwordRules: FormRules = {
  old_password: [
    { required: true, message: t('user.profile.currentPasswordRequired'), trigger: ['blur', 'change'] },
  ],
  new_password: [
    { required: true, message: t('user.profile.newPasswordRequired'), trigger: ['blur', 'change'] },
    { min: 6, message: t('user.profile.passwordMin'), trigger: 'blur' },
  ],
  confirm_password: [
    { required: true, message: t('user.profile.confirmRequired'), trigger: ['blur', 'change'] },
    { validator: validateConfirmPassword, trigger: 'blur' },
  ] }

// 加载用户信息
const loadProfile = async () => {
  try {
    const { data } = await getCurrentUser()
    profile.value = data.data
    profileForm.display_name = data.data.display_name || ''
    profileForm.email = data.data.email || ''
    profileForm.lark_open_id = data.data.lark_open_id || ''
    profileForm.telegram_user_id = data.data.telegram_user_id || ''
    profileForm.locale = data.data.locale || ''
  } catch {
    // ignored
  }
}

// 保存基本信息
const submitProfile = async () => {
  if (!profileFormRef.value) return

  await profileFormRef.value.validate(async (valid) => {
    if (!valid) return

    profileLoading.value = true
    try {
      await updateCurrentUser({
        display_name: profileForm.display_name,
        email: profileForm.email,
        lark_open_id: profileForm.lark_open_id,
        telegram_user_id: profileForm.telegram_user_id,
        locale: profileForm.locale })
      ElMessage.success(t('common.saveSuccess'))
      // 立刻切到新语言，不必等下次登录
      applyAccountLocale(profileForm.locale)
      loadProfile()
    } catch {
      // ignored
    } finally {
      profileLoading.value = false
    }
  })
}

// 修改密码
const submitPassword = async () => {
  if (!passwordFormRef.value) return

  await passwordFormRef.value.validate(async (valid) => {
    if (!valid) return

    passwordLoading.value = true
    try {
      const data: UpdatePasswordRequest = {
        old_password: passwordForm.old_password,
        new_password: passwordForm.new_password }
      await updatePassword(data)
      ElMessage.success(t('user.profile.passwordChanged'))
      passwordDialogVisible.value = false
    } catch {
      // ignored
    } finally {
      passwordLoading.value = false
    }
  })
}

// 打开修改密码弹窗
const openPasswordDialog = () => {
  passwordDialogVisible.value = true
}

// 弹窗关闭后清空，避免下次打开时还留着上次输入的密码
const resetPasswordForm = () => {
  passwordForm.old_password = ''
  passwordForm.new_password = ''
  passwordForm.confirm_password = ''
  passwordFormRef.value?.clearValidate()
}

// 滚动到基本信息区域
const scrollToProfile = () => {
  profileFormRef.value?.$el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}

// 格式化日期
const formatDate = (date?: string) => {
  if (!date) return '-'
  return dayjs(date).format('YYYY-MM-DD')
}

// ============ MFA 相关 ============
const mfaStatus = ref<MFAStatusResponse | null>(null)
const mfaSetupDialogVisible = ref(false)
const mfaDisableDialogVisible = ref(false)
const mfaSetupData = ref<MFASetupResponse | null>(null)
const mfaVerifyCode = ref('')
const mfaDisableCode = ref('')
const mfaEnabling = ref(false)
const mfaDisabling = ref(false)
const qrCodeUrl = ref('')

// 加载 MFA 状态
const loadMFAStatus = async () => {
  try {
    const { data } = await getMFAStatus()
    mfaStatus.value = data.data
  } catch {
    // ignored
  }
}

// 开始 MFA 设置
const startMFASetup = async () => {
  try {
    const { data } = await setupMFA()
    mfaSetupData.value = data.data
    // 直接使用后端返回的 base64 编码的 QR 码图片
    qrCodeUrl.value = data.data.qr_code_data
    mfaVerifyCode.value = ''
    mfaSetupDialogVisible.value = true
  } catch {
    ElMessage.error(t('user.profile.mfaSetupFailed'))
  }
}

// 确认启用 MFA
const confirmEnableMFA = async () => {
  if (mfaVerifyCode.value.length !== 6) {
    ElMessage.warning(t('user.profile.codeRequired'))
    return
  }

  mfaEnabling.value = true
  try {
    await enableMFA(mfaVerifyCode.value)
    ElMessage.success(t('user.profile.mfaEnabled'))
    mfaSetupDialogVisible.value = false
    loadMFAStatus()
  } catch {
    ElMessage.error(t('user.profile.codeWrong'))
  } finally {
    mfaEnabling.value = false
  }
}

// 显示禁用 MFA 对话框
const showDisableMFADialog = () => {
  mfaDisableCode.value = ''
  mfaDisableDialogVisible.value = true
}

// 确认禁用 MFA
const confirmDisableMFA = async () => {
  if (mfaDisableCode.value.length !== 6) {
    ElMessage.warning(t('user.profile.codeRequired'))
    return
  }

  mfaDisabling.value = true
  try {
    await disableMFA(mfaDisableCode.value)
    ElMessage.success(t('user.profile.mfaDisabled'))
    mfaDisableDialogVisible.value = false
    loadMFAStatus()
  } catch {
    ElMessage.error(t('user.profile.codeWrong'))
  } finally {
    mfaDisabling.value = false
  }
}

// 初始化
onMounted(() => {
  loadProfile()
  loadMFAStatus()
})
</script>

<style scoped lang="scss">
// 骨架在 _apple.scss 里；这一页只留左侧身份卡和表单区的排版。

.profile-grid { grid-template-columns: 320px 1fr; }

@media (max-width: 1100px) {
  .profile-grid { grid-template-columns: 1fr; }
}

.profile-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 24px 18px 18px;
}

.avatar-wrapper { position: relative; }

.user-avatar {
  background: var(--td-tag-primary-bg);
  color: var(--td-tag-primary-text);
  font-size: 30px;
  font-weight: var(--td-weight-semibold);
}

.username {
  font-size: 17px;
  font-weight: var(--td-weight-semibold);
  margin: 0;
}

.user-role { margin: 0; }

.profile-info {
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--td-divider-color);
}

.info-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 18px;
  border-bottom: 1px solid var(--td-divider-color);

  &:last-child { border-bottom: 0; }
}

/* label 在左、value 在右：两个 span 直接相邻会连成「用户名admin」 */
.info-content {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  flex: 1;
  min-width: 0;
}

.info-label { font-size: 12px; color: var(--td-text-placeholder); white-space: nowrap; }

.info-value {
  font-size: 13px;
  color: var(--td-text-primary);
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ── 账户安全 ─────────────────────────────────── */
.security-info :deep(.el-divider--horizontal) {
  margin: 0;
}

.security-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 18px;
}

.security-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.security-icon {
  font-size: 15px;
  flex-shrink: 0;

  &.success { color: var(--td-color-success); }
  &.warning { color: var(--td-color-warning); }
}

.security-content {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.security-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-primary);
}

.security-desc {
  font-size: 11.5px;
  color: var(--td-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

// 表单区：卡片头下面留一段内边距
.card :deep(.el-form) { padding: 16px 18px; }

.settings-form :deep(.el-form-item__label) {
  font-size: 12.5px;
  color: var(--td-text-secondary);
}

/* 字段下面的说明文字 */
.form-tip-inline {
  margin-top: 4px;
  font-size: 11.5px;
  line-height: 1.55;
  color: var(--td-text-secondary);
}

/* 头像上的相机角标 */
.avatar-overlay {
  position: absolute;
  right: 2px;
  bottom: 2px;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--td-bg-card);
  border: 1px solid var(--td-border-color);
  color: var(--td-text-secondary);
  font-size: 11px;
  cursor: pointer;

  &:hover { background: var(--td-bg-section); }
}

/* ── 双因素认证弹窗 ─────────────────────────────
   这几个类模板里一直在用，样式是上一轮重写时连同旧骨架一起删掉的：
   三步之间没有间距、二维码贴边、密钥和上面的图挤在一起、
   验证码输入框还是整行宽。 */
.mfa-setup {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.mfa-step {
  h4 {
    margin: 0 0 4px;
    font-size: 13px;
    font-weight: 590;
    letter-spacing: -0.01em;
    color: var(--td-text-primary);
  }

  p {
    margin: 0;
    font-size: 12.5px;
    line-height: 1.6;
    color: var(--td-text-secondary);
  }
}

.qr-code {
  display: flex;
  justify-content: center;
  margin: 12px 0 10px;

  img {
    width: 168px;
    height: 168px;
    padding: 8px;
    background: #fff; // 二维码必须白底才扫得出来，暗色下也不能跟着变
    border: 1px solid var(--td-border-color);
    border-radius: 10px;
  }
}

.manual-key {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;

  code {
    font-family: var(--td-font-mono);
    font-size: 11.5px;
    color: var(--td-text-primary);
    background: var(--td-code-bg);
    border: 1px solid var(--td-border-color-light);
    border-radius: 5px;
    padding: 2px 7px;
    word-break: break-all;
  }
}

/* 六位数字：窄、居中、拉开字距，照着验证器 App 的样子 */
.verify-code-input {
  width: 180px;
  margin-top: 10px;

  :deep(.el-input__inner) {
    text-align: center;
    font-family: var(--td-font-mono);
    font-size: 18px;
    letter-spacing: 0.28em;
    text-indent: 0.28em;
  }
}
</style>
