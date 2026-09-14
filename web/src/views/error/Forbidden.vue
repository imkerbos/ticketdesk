<template>
  <div class="error-page">
    <div class="error-content">
      <div class="error-code">403</div>
      <h2 class="error-title">{{ t('error.forbiddenTitle') }}</h2>
      <p class="error-desc">{{ t('error.forbiddenDesc') }}</p>
      <div class="error-actions">
        <el-button type="primary" @click="router.push('/dashboard')">{{ t('error.goHome') }}</el-button>
        <el-button @click="router.back()">{{ t('error.goBack') }}</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

const { t } = useI18n()
const router = useRouter()
</script>

<style scoped lang="scss">
.error-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: var(--td-bg-page);
}

.error-content {
  text-align: center;
  max-width: 420px;
}

/* 状态码是背景信息，不是这一屏的重点：真正要读的是下面那句说明和两个出口按钮。
   缩成一行小字标签，把视觉重量全部让给标题。 */
.error-code {
  font-size: 12px;
  font-weight: 590;
  letter-spacing: 0.06em;
  color: var(--td-text-placeholder);
  line-height: 1;
  margin-bottom: 12px;
  font-variant-numeric: tabular-nums;
}

.error-title {
  font-size: 26px;
  font-weight: 600;
  letter-spacing: -0.022em;
  line-height: 1.15;
  color: var(--td-text-primary);
  margin: 0 0 8px;
}

.error-desc {
  font-size: 14px;
  line-height: 1.5;
  color: var(--td-text-secondary);
  margin: 0 0 24px;
}

.error-actions {
  display: flex;
  justify-content: center;
  gap: 10px;
}

.error-actions :deep(.el-button) {
  border-radius: 8px;
  font-weight: 500;
  letter-spacing: -0.01em;
}
</style>
