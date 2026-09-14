<template>
  <div class="login-page">
    <!-- 左侧品牌区域 -->
    <div class="brand-section">
      <div class="brand-content">
        <div class="brand-logo">
          <img v-if="brandStore.logoUrl" :src="brandStore.logoUrl" :alt="brandStore.systemName" class="logo-custom" />
          <span class="logo-text">{{ brandStore.systemName }}</span>
        </div>
        <h1 class="brand-title">{{ brandStore.loginTitle }}</h1>
        <p class="brand-description" v-html="brandDescriptionHtml"></p>
        <div class="feature-list">
          <div class="feature-item stagger" style="--i: 0">
            <el-icon class="feature-icon"><Tickets /></el-icon>
            <span>{{ t('auth.featureIssue') }}</span>
          </div>
          <div class="feature-item stagger" style="--i: 1">
            <el-icon class="feature-icon"><Bell /></el-icon>
            <span>{{ t('auth.featureAlert') }}</span>
          </div>
          <div class="feature-item stagger" style="--i: 2">
            <el-icon class="feature-icon"><Connection /></el-icon>
            <span>{{ t('auth.featureWorkflow') }}</span>
          </div>
        </div>
      </div>
      <div class="brand-footer">
        <span>{{ brandStore.copyrightText }}</span>
      </div>
    </div>

    <!-- 右侧登录表单区域 -->
    <div class="login-section">
      <div :class="['login-container', { shake: shaking }]" @animationend="shaking = false">
        <div class="login-header fade-up" style="--i: 0">
          <h2 class="login-title">{{ mfaRequired ? t('auth.mfaTitle') : t('auth.welcomeBack') }}</h2>
          <p class="login-subtitle">
            {{ mfaRequired ? t('auth.mfaSubtitle') : t('auth.loginSubtitle') }}
          </p>
        </div>

        <el-form
          v-if="!mfaRequired"
          ref="formRef"
          :model="form"
          :rules="rules"
          class="login-form"
          label-position="top"
          hide-required-asterisk
          @submit.prevent="handleLogin"
        >
          <el-form-item prop="username" :label="t('auth.username')" class="fade-up" style="--i: 1">
            <el-input
              v-model="form.username"
              :placeholder="t('auth.usernamePlaceholder')"
              size="large"
              class="form-input"
            >
              <template #prefix>
                <el-icon class="input-icon"><User /></el-icon>
              </template>
            </el-input>
          </el-form-item>

          <el-form-item prop="password" :label="t('auth.password')" class="fade-up" style="--i: 2">
            <el-input
              v-model="form.password"
              type="password"
              :placeholder="t('auth.passwordPlaceholder')"
              size="large"
              show-password
              class="form-input"
              @keyup.enter="handleLogin"
            >
              <template #prefix>
                <el-icon class="input-icon"><Lock /></el-icon>
              </template>
            </el-input>
          </el-form-item>

          <el-form-item class="remember-row fade-up" style="--i: 3">
            <div class="remember-forgot">
              <el-checkbox v-model="rememberMe">{{ t('auth.rememberMe') }}</el-checkbox>
              <router-link to="/forgot-password" class="forgot-link">{{ t('auth.forgotPassword') }}</router-link>
            </div>
          </el-form-item>

          <el-form-item class="fade-up" style="--i: 4">
            <el-button
              type="primary"
              size="large"
              class="login-button"
              :loading="loading"
              @click="handleLogin"
            >
              {{ loading ? t('auth.loggingIn') : t('auth.login') }}
            </el-button>
          </el-form-item>
        </el-form>

        <!-- 两步验证：密码校验通过后凭挑战令牌提交 TOTP 码 -->
        <el-form v-else class="login-form" label-position="top" @submit.prevent="handleVerifyMFA">
          <el-form-item :label="t('auth.mfaCode')">
            <el-input
              ref="mfaInputRef"
              v-model="mfaCode"
              :placeholder="t('auth.mfaPlaceholder')"
              size="large"
              maxlength="6"
              inputmode="numeric"
              autocomplete="one-time-code"
              class="form-input"
              @input="mfaCode = mfaCode.replace(/\D/g, '')"
              @keyup.enter="handleVerifyMFA"
            >
              <template #prefix>
                <el-icon class="input-icon"><Key /></el-icon>
              </template>
            </el-input>
          </el-form-item>

          <el-form-item>
            <el-button
              type="primary"
              size="large"
              class="login-button"
              :loading="loading"
              :disabled="mfaCode.length !== 6"
              @click="handleVerifyMFA"
            >
              {{ t('auth.mfaSubmit') }}
            </el-button>
          </el-form-item>

          <el-form-item>
            <el-button link class="mfa-back" @click="cancelMFA">{{ t('auth.mfaBack') }}</el-button>
          </el-form-item>
        </el-form>

        <!-- SSO 登录区域 -->
        <div v-if="ssoConfig?.enabled && !mfaRequired" class="sso-section fade-up" style="--i: 5">
          <div class="sso-divider">
            <span class="sso-divider-text">{{ t('auth.or') }}</span>
          </div>
          <el-button
            size="large"
            class="sso-button"
            :loading="ssoLoading"
            @click="handleSSOLogin"
          >
            {{ ssoConfig.provider_name || t('auth.ssoLogin') }}
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { applyAccountLocale } from '@/i18n'
import { useI18n } from 'vue-i18n'
import { ref, reactive, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { User, Lock, Key, Tickets, Bell, Connection } from '@element-plus/icons-vue'
import { login, verifyMFALogin, getSSOConfig, getSSOAuthorizeURL, type SSOConfigResponse, type LoginResponse } from '@/api/auth'
import { useUserStore } from '@/stores/user'
import { useBrandStore } from '@/stores/brand'

const { t } = useI18n()

const router = useRouter()
const userStore = useUserStore()
const brandStore = useBrandStore()
const formRef = ref<FormInstance>()
const loading = ref(false)
const rememberMe = ref(false)
const ssoConfig = ref<SSOConfigResponse | null>(null)
const ssoLoading = ref(false)
const shaking = ref(false)

// 品牌描述来自后台可配置项，经公开的 GET /api/v1/brand 下发。
// 之前直接把原文交给 v-html，只替换了换行 —— 任何能改品牌配置的人都能把脚本
// 注入到所有人的登录页（此时 token 就存在 localStorage 里，可被直接读走）。
// 这里先做 HTML 转义，再把换行还原成 <br />，保证只有换行是「标签」。
const escapeHtml = (raw: string): string =>
  raw
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')

const brandDescriptionHtml = computed(() => {
  return escapeHtml(brandStore.loginDescription ?? '').replace(/\n/g, '<br />')
})

const form = reactive({
  username: '',
  password: '',
})

const rules: FormRules = {
  username: [
    { required: true, message: t('auth.usernameRequired'), trigger: ['blur', 'change'] },
  ],
  password: [
    { required: true, message: t('auth.passwordRequired'), trigger: ['blur', 'change'] },
    { min: 6, message: t('auth.passwordMinLen'), trigger: 'blur' },
  ],
}

// 两步验证状态：mfaToken 是密码校验通过后由后端下发的短期挑战令牌
const mfaRequired = ref(false)
const mfaToken = ref('')
const mfaCode = ref('')
const mfaInputRef = ref()

// 登录成功的收尾动作（普通登录与两步验证共用）
const finishLogin = (data: LoginResponse) => {
  if (!data.access_token || !data.refresh_token || !data.user) {
    ElMessage.error(t('auth.loginRespError'))
    return
  }
  userStore.login(data.access_token, data.refresh_token, data.user)
  // 账号里存的语言优先于本机选择：换设备登录要跟上账号设置
  applyAccountLocale(data.user.locale)
  ElMessage.success(t('auth.welcomeUser', { name: data.user.display_name || data.user.username }))
  router.push('/')
}

const cancelMFA = () => {
  mfaRequired.value = false
  mfaToken.value = ''
  mfaCode.value = ''
  form.password = ''
}

const handleVerifyMFA = async () => {
  if (mfaCode.value.length !== 6 || loading.value) return

  loading.value = true
  try {
    const res = await verifyMFALogin({ mfa_token: mfaToken.value, code: mfaCode.value })
    finishLogin(res.data.data)
  } catch {
    // 错误提示已由 request 拦截器统一处理
    mfaCode.value = ''
    shaking.value = true
  } finally {
    loading.value = false
  }
}

const handleLogin = async () => {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (!valid) {
      shaking.value = true
      return
    }

    loading.value = true
    try {
      const res = await login({
        username: form.username,
        password: form.password,
      })

      const data = res.data.data

      // 账号启用了 MFA：此时尚未登录，切到第二步
      if (data.requires_mfa && data.mfa_token) {
        mfaToken.value = data.mfa_token
        mfaRequired.value = true
        mfaCode.value = ''
        await nextTick()
        mfaInputRef.value?.focus()
        return
      }

      finishLogin(data)
    } catch {
      // 错误已在 request 拦截器中处理
      shaking.value = true
    } finally {
      loading.value = false
    }
  })
}

// SSO 登录
const handleSSOLogin = async () => {
  ssoLoading.value = true
  try {
    const res = await getSSOAuthorizeURL()
    const { authorize_url } = res.data.data
    window.location.href = authorize_url
  } catch {
    ElMessage.error(t('auth.ssoUrlFailed'))
    ssoLoading.value = false
  }
}

// 获取 SSO 配置
onMounted(async () => {
  try {
    const res = await getSSOConfig()
    ssoConfig.value = res.data.data
  } catch {
    // SSO 配置获取失败不影响正常登录
  }
})
</script>

<style scoped>
.login-page {
  display: flex;
  min-height: 100vh;
  background-color: var(--td-bg-page);
}

/* 左侧品牌区域 */
.brand-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 48px;
  /* 侧边栏翻成浅色之后，这里再用 --td-sidebar-bg 配白字就是白底白字。
     品牌面板改成分区底色 + 正常文字色，和应用内保持一套。 */
  background: var(--td-bg-section);
  color: var(--td-text-primary);
  border-right: 1px solid var(--td-border-color);
}

.brand-content {
  max-width: 480px;
}

.brand-logo {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 48px;
}

.logo-custom {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  object-fit: contain;
}

.logo-text {
  font-size: 24px;
  font-weight: 700;
}

.brand-title {
  font-size: 36px;
  font-weight: 700;
  margin: 0 0 16px;
  line-height: 1.3;
}

.brand-description {
  font-size: 16px;
  line-height: 1.8;
  color: var(--td-text-secondary);
  margin: 0 0 48px;
}

.feature-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.feature-item {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 15px;
  color: var(--td-text-regular);
}

/* 只留图标，不套方块（§3.1 不放装饰性图标色块） */
.feature-icon {
  width: 18px;
  height: 18px;
  color: var(--td-text-placeholder);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
}

.brand-footer {
  font-size: 13px;
  color: var(--td-text-placeholder);
}

/* 右侧登录表单区域 */
.login-section {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px;
}

.login-container {
  width: 100%;
  max-width: 400px;
}

.login-header {
  margin-bottom: 36px;
}

.login-title {
  font-size: 26px;
  font-weight: 600;
  letter-spacing: -0.022em;
  color: var(--td-text-primary);
  margin: 0 0 8px;
}

.login-subtitle {
  font-size: 15px;
  color: var(--td-text-secondary);
  margin: 0;
}

.login-form {
  width: 100%;
}

.mfa-back {
  width: 100%;
  justify-content: center;
  color: var(--td-text-secondary);
}

.login-form :deep(.el-form-item__label) {
  font-size: 14px;
  font-weight: 500;
  color: var(--td-text-regular);
  padding-bottom: 8px;
}

.remember-forgot {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.forgot-link {
  font-size: 13px;
  color: var(--td-color-primary);
  text-decoration: none;
  transition: color 150ms ease-out;
  font-weight: 400;
}

.forgot-link:hover {
  color: var(--td-color-primary-hover);
}

.form-input :deep(.el-input__wrapper) {
  padding: 4px 12px;
  border-radius: 8px;
  box-shadow: 0 0 0 1px var(--td-border-color);
  transition: box-shadow 150ms ease-out;
}

.form-input :deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px var(--td-border-color-dark);
}

.form-input :deep(.el-input__wrapper.is-focus) {
  box-shadow: var(--td-focus-ring);
}

.input-icon {
  color: var(--td-text-placeholder);
  transition: color 150ms ease-out;
}

.form-input :deep(.el-input__wrapper.is-focus .input-icon) {
  color: var(--td-color-primary);
}

.remember-row {
  margin-bottom: 24px;
}

.remember-row :deep(.el-checkbox__label) {
  font-size: 14px;
  color: var(--td-text-regular);
}

.login-button {
  width: 100%;
  height: 48px;
  font-size: 16px;
  font-weight: 600;
  border-radius: 8px;
  background: var(--td-color-primary);
  border: none;
  transition: background-color 150ms ease-out, box-shadow 150ms ease-out;
}

/* hover 只换底色，不加光晕（方向 A / Apple 都是这条） */
.login-button:hover {
  background: var(--td-color-primary-hover);
}

.login-button:active {
  background: var(--td-color-primary-active);
  box-shadow: none;
}

/* SSO 登录区域 */
.sso-section {
  margin-top: 8px;
}

.sso-divider {
  display: flex;
  align-items: center;
  margin: 16px 0;
}

.sso-divider::before,
.sso-divider::after {
  content: '';
  flex: 1;
  border-top: 1px solid var(--td-border-color);
}

.sso-divider-text {
  padding: 0 16px;
  font-size: 13px;
  color: var(--td-text-placeholder);
}

.sso-button {
  width: 100%;
  height: 48px;
  font-size: 15px;
  font-weight: 500;
  border-radius: 8px;
  border: 1px solid var(--td-border-color-dark);
  background: var(--td-bg-card);
  color: var(--td-text-regular);
  transition: border-color 150ms ease-out, color 150ms ease-out, background-color 150ms ease-out;
}

.sso-button:hover {
  border-color: var(--td-color-primary);
  color: var(--td-color-primary);
  background: var(--td-tag-primary-bg);
}


/* prefers-reduced-motion 降级 */
@media (prefers-reduced-motion: reduce) {
  .fade-up,
  .stagger {
    opacity: 1;
    animation: none;
  }
  .shake {
    animation: none;
  }
}

/* 响应式设计 */
@media (max-width: 1024px) {
  .brand-section {
    display: none;
  }

  .login-section {
    flex: 1;
  }
}

@media (max-width: 480px) {
  .login-section {
    padding: 24px;
  }

  .login-title {
    font-size: 24px;
  }
}
</style>
