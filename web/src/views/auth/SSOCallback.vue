<template>
  <div class="sso-callback-page">
    <div class="callback-container">
      <!-- 加载中 -->
      <div v-if="loading" class="callback-loading">
        <el-icon class="loading-icon" :size="48"><Loading /></el-icon>
        <p class="loading-text">{{ t('auth.ssoCallbackLoading') }}</p>
      </div>

      <!-- 错误 -->
      <div v-else-if="error" class="callback-error">
        <el-icon class="error-icon" :size="48"><CircleCloseFilled /></el-icon>
        <p class="error-text">{{ error }}</p>
        <el-button type="primary" size="large" class="back-button" @click="goToLogin">
          {{ t('auth.backToLogin') }}
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { applyAccountLocale } from '@/i18n'
import { useI18n } from 'vue-i18n'
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Loading, CircleCloseFilled } from '@element-plus/icons-vue'
import { ssoCallback } from '@/api/auth'
import { useUserStore } from '@/stores/user'

const { t } = useI18n()

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()
const loading = ref(true)
const error = ref('')

const goToLogin = () => {
  router.push('/login')
}

onMounted(async () => {
  const code = route.query.code as string
  const state = route.query.state as string

  if (!code) {
    // 没有 code，可能是 IdP-initiated 直接跳过来但没带参数，走一遍完整授权流程
    router.replace('/auth/sso/login')
    return
  }

  if (!state) {
    // 有 code 但没有 state（IdP-initiated 直接带 code），无法验证安全性
    // 重新走一遍完整的 OIDC 授权流程（用户已登录 EIAM 会秒过）
    router.replace('/auth/sso/login')
    return
  }

  try {
    const res = await ssoCallback({ code, state })
    const { access_token, refresh_token, user } = res.data.data

    // SSO 回调不会走本地 MFA 流程（二次验证由身份提供方负责），
    // 令牌字段理应齐备；缺失说明后端响应异常，明确报错而不是带着空值往下走
    if (!access_token || !refresh_token || !user) {
      error.value = t('auth.ssoRespError')
      loading.value = false
      return
    }

    userStore.login(access_token, refresh_token, user)
    // 账号里存的语言优先于本机选择：换设备登录要跟上账号设置
    applyAccountLocale(user.locale)
    ElMessage.success(t('auth.welcomeUser', { name: user.display_name || user.username }))
    router.push('/')
  } catch (err: any) {
    const message = err?.response?.data?.message || t('auth.ssoFailed')
    error.value = message
    loading.value = false
  }
})
</script>

<style scoped>
.sso-callback-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background-color: var(--td-bg-page);
}

.callback-container {
  text-align: center;
  padding: 48px;
}

.callback-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}

.loading-icon {
  color: var(--td-color-primary);
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.loading-text {
  font-size: 16px;
  color: var(--td-text-secondary);
}

.callback-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}

.error-icon {
  color: var(--td-color-danger);
}

.error-text {
  font-size: 16px;
  color: var(--td-text-regular);
}

.back-button {
  margin-top: 8px;
  min-width: 160px;
  height: 44px;
  border-radius: 8px;
}
</style>
