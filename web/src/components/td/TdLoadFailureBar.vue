<template>
  <!-- 常驻，不自动消失：toast 三秒就没了，剩下一张空表看着像「本来就没数据」 -->
  <div v-if="loadFailure" class="load-failure" role="alert">
    <el-icon class="load-failure__icon"><WarningFilled /></el-icon>
    <div class="load-failure__text">
      <div class="load-failure__title">{{ t('common.loadFailedTitle') }}</div>
      <div class="load-failure__desc">
        {{ loadFailure === 'network' ? t('common.loadFailedNetwork') : t('common.loadFailedServer') }}
      </div>
    </div>
    <button class="btn" @click="retry">{{ t('common.retry') }}</button>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { WarningFilled } from '@element-plus/icons-vue'
import { loadFailure, clearLoadFailure } from '@/utils/load-failure'

const { t } = useI18n()

// 整页重载而不是重跑某一个请求：一页上通常有好几个并发请求一起失败，
// 逐个重试要每个页面自己记住「刚才都请求了什么」，重载是唯一一处能做对的。
const retry = () => {
  clearLoadFailure()
  window.location.reload()
}
</script>

<style scoped lang="scss">
.load-failure {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 0 0 14px;
  padding: 12px 16px;
  border: 1px solid var(--td-tag-danger-border);
  border-radius: var(--td-radius-lg);
  background: var(--td-tag-danger-bg);
}

.load-failure__icon {
  flex-shrink: 0;
  font-size: 17px;
  color: var(--td-color-danger);
}

.load-failure__text {
  flex: 1;
  min-width: 0;
}

.load-failure__title {
  font-size: 13.5px;
  font-weight: 590;
  letter-spacing: -0.01em;
  color: var(--td-text-primary);
}

.load-failure__desc {
  font-size: 12.5px;
  color: var(--td-text-secondary);
  line-height: var(--td-leading-normal);
}
</style>
