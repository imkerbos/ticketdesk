<template>
  <div class="setup-page">
    <div class="setup-card">
      <header class="setup-header">
        <h1 class="setup-title">{{ t('setup.title') }}</h1>
        <p class="setup-subtitle">{{ t('setup.subtitle') }}</p>
      </header>

      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="setup-form">
        <!-- 令牌 -->
        <section class="setup-section">
          <h2 class="section-title">{{ t('setup.tokenSection') }}</h2>
          <el-form-item :label="t('setup.token')" prop="token">
            <el-input v-model="form.token" :placeholder="t('setup.tokenPlaceholder')" show-password />
            <div class="field-tip">
              <span>{{ t('setup.tokenTipK8s') }}</span>
              <!-- 命令是字面量，不走语言包：里面的花括号会被 vue-i18n 当成插值占位符，
                   编译期直接报错、把整段渲染掉；而且 shell 命令两种语言下本就一样 -->
              <code>{{ k8sTokenCommand }}</code>
              <span>{{ t('setup.tokenTipOther') }}</span>
            </div>
          </el-form-item>
        </section>

        <!-- 管理员 -->
        <section class="setup-section">
          <h2 class="section-title">{{ t('setup.adminSection') }}</h2>
          <p class="section-hint">{{ t('setup.adminTip') }}</p>
          <div class="field-grid">
            <el-form-item :label="t('setup.username')" prop="username">
              <el-input v-model="form.username" :placeholder="t('setup.usernamePlaceholder')" />
            </el-form-item>
            <el-form-item :label="t('setup.displayName')" prop="display_name">
              <el-input v-model="form.display_name" :placeholder="t('setup.displayNamePlaceholder')" />
            </el-form-item>
            <el-form-item :label="t('setup.password')" prop="password">
              <el-input v-model="form.password" type="password" :placeholder="t('setup.passwordPlaceholder')" show-password />
            </el-form-item>
            <el-form-item :label="t('setup.confirmPassword')" prop="confirm">
              <el-input v-model="form.confirm" type="password" :placeholder="t('setup.confirmPlaceholder')" show-password />
            </el-form-item>
          </div>
          <el-form-item :label="t('setup.email')" prop="email">
            <el-input v-model="form.email" :placeholder="t('setup.emailPlaceholder')" />
          </el-form-item>
        </section>

        <!-- 站点 -->
        <section class="setup-section">
          <h2 class="section-title">{{ t('setup.siteSection') }}</h2>
          <el-form-item :label="t('setup.systemName')" prop="system_name">
            <el-input v-model="form.system_name" :placeholder="t('setup.systemNamePlaceholder')" maxlength="50" />
            <div class="field-tip"><span>{{ t('setup.systemNameTip') }}</span></div>
          </el-form-item>
          <el-form-item :label="t('setup.siteUrl')" prop="site_url">
            <el-input v-model="form.site_url" :placeholder="t('setup.siteUrlPlaceholder')" />
            <div class="field-tip"><span>{{ t('setup.siteUrlTip') }}</span></div>
          </el-form-item>
          <el-form-item :label="t('setup.language')" prop="language">
            <el-select v-model="form.language" style="width: 100%">
              <el-option :label="t('lang.zh-CN')" value="zh-CN" />
              <el-option :label="t('lang.en-US')" value="en-US" />
            </el-select>
            <div class="field-tip"><span>{{ t('setup.languageTip') }}</span></div>
          </el-form-item>
        </section>

        <el-button type="primary" size="large" class="setup-submit" :loading="loading" @click="submit">
          {{ loading ? t('setup.submitting') : t('setup.submit') }}
        </el-button>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { submitSetup } from '@/api/setup'
import { login as loginApi } from '@/api/auth'
import { useSetupStore } from '@/stores/setup'
import { useUserStore } from '@/stores/user'
import { applyAccountLocale, getLocale } from '@/i18n'

const { t } = useI18n()
const router = useRouter()
const setupStore = useSetupStore()
const userStore = useUserStore()

// 取 setup token 的命令。刻意用字符串拼接把花括号藏起来，
// 直接写在模板里的 {{ }} 会被 Vue 当成插值
const k8sTokenCommand =
  "kubectl get secret <release>-secret -o jsonpath='" + '{.data.setup-token}' + "' | base64 -d"

const formRef = ref<FormInstance>()
const loading = ref(false)

const form = reactive({
  token: '',
  username: '',
  password: '',
  confirm: '',
  display_name: '',
  email: '',
  system_name: 'TicketDesk',
  site_url: window.location.origin,
  // 默认跟当前界面语言一致：装机的人正在用哪种语言看这个页面，
  // 多半就想让平台默认是哪种
  language: getLocale(),
})

const rules: FormRules = {
  token: [{ required: true, message: t('setup.tokenRequired'), trigger: ['blur', 'change'] }],
  username: [
    { required: true, message: t('setup.usernameRequired'), trigger: ['blur', 'change'] },
    { min: 3, max: 50, message: t('setup.usernameLength'), trigger: 'blur' },
  ],
  password: [
    { required: true, message: t('setup.passwordRequired'), trigger: ['blur', 'change'] },
    { min: 6, message: t('setup.passwordMin'), trigger: 'blur' },
  ],
  confirm: [
    { required: true, message: t('setup.confirmRequired'), trigger: ['blur', 'change'] },
    {
      validator: (_r: unknown, value: string, cb: (e?: Error) => void) => {
        if (value !== form.password) cb(new Error(t('setup.passwordMismatch')))
        else cb()
      },
      trigger: 'blur',
    },
  ],
  display_name: [{ required: true, message: t('setup.displayNameRequired'), trigger: ['blur', 'change'] }],
  email: [
    { required: true, message: t('setup.emailRequired'), trigger: ['blur', 'change'] },
    { type: 'email', message: t('setup.emailInvalid'), trigger: 'blur' },
  ],
  system_name: [{ required: true, message: t('setup.systemNameRequired'), trigger: ['blur', 'change'] }],
  site_url: [
    { required: true, message: t('setup.siteUrlRequired'), trigger: ['blur', 'change'] },
    { type: 'url', message: t('setup.siteUrlInvalid'), trigger: 'blur' },
  ],
}

// 选了哪种语言，界面立刻跟着变。
//
// 首装时这是唯一能改语言的地方，选了不变会让人以为没生效；
// 而且这个值会成为管理员自己的 locale，登录后本来就是这个语言，
// 现在不切反倒是前后割裂。
watch(
  () => form.language,
  (lang) => applyAccountLocale(lang),
)

const submit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    loading.value = true
    try {
      await submitSetup({
        token: form.token,
        username: form.username,
        password: form.password,
        display_name: form.display_name,
        email: form.email,
        system_name: form.system_name,
        site_url: form.site_url,
        language: form.language,
      })
      setupStore.markInitialized()
      ElMessage.success(t('setup.success'))

      // 用刚设好的账号直接登录，免得再让人输一遍
      const res = await loginApi({ username: form.username, password: form.password })
      const data = res.data.data
      // refresh_token / user 在 MFA 场景下可能为空；刚建的账号不会开 MFA，
      // 但类型上要收敛，拿不全就老实回登录页
      if (data?.access_token && data.refresh_token && data.user) {
        userStore.login(data.access_token, data.refresh_token, data.user)
        applyAccountLocale(data.user.locale)
        router.replace('/')
      } else {
        router.replace('/login')
      }
    } catch (error: any) {
      // 409：别的副本抢先完成了初始化，这不是错误，送去登录页即可
      if (error?.response?.status === 409) {
        setupStore.markInitialized()
        ElMessage.info(t('setup.alreadyDone'))
        router.replace('/login')
      }
      // 其余错误已由请求拦截器提示
    } finally {
      loading.value = false
    }
  })
}
</script>

<style scoped lang="scss">
.setup-page {
  min-height: 100vh;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 56px 20px;
  background: var(--td-bg-page);
  color: var(--td-text-primary);
}

.setup-card {
  width: 100%;
  max-width: 680px;
  background: var(--td-bg-card);
  border: 1px solid var(--td-border-color);
  border-radius: 12px;
  padding: 32px 32px 28px;
}

.setup-header {
  margin-bottom: 28px;
}

.setup-title {
  margin: 0 0 6px;
  font-size: 26px;
  font-weight: 600;
  letter-spacing: -0.022em;
  line-height: 1.15;
  color: var(--td-text-primary);
}

.setup-subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--td-text-secondary);
}

.setup-section {
  padding-top: 22px;
  margin-top: 22px;
  border-top: 1px solid var(--td-divider-color);

  &:first-of-type {
    padding-top: 0;
    margin-top: 0;
    border-top: none;
  }
}

.section-title {
  margin: 0 0 4px;
  font-size: 15px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--td-text-primary);
}

.section-hint {
  margin: 0 0 14px;
  font-size: 12px;
  color: var(--td-text-secondary);
}

.field-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;

  @media (max-width: 640px) {
    grid-template-columns: 1fr;
  }
}

.field-tip {
  margin-top: 6px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--td-text-secondary);

  code {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 11px;
    padding: 6px 9px;
    border-radius: 7px;
    background: var(--td-code-bg);
    border: 1px solid var(--td-border-color-light);
    color: var(--td-text-regular);
    word-break: break-all;
  }
}

.setup-submit {
  width: 100%;
  margin-top: 4px;
  border-radius: 8px;
  font-weight: 500;
  letter-spacing: -0.01em;
}
</style>
