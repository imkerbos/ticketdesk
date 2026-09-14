<template>
  <div class="td-empty-state" :class="`td-empty-state--${tone}`">
    <!--
      必须套 el-icon：Element 的图标组件本身是一个没有尺寸的 <svg>，
      宽高来自 .el-icon 这层包装。原来直接渲染 <component :is>，
      svg 的实际尺寸是 0x0 —— 全站空状态的图标一个都没显示过。
    -->
    <el-icon class="td-empty-state__icon">
      <component :is="iconComponent" />
    </el-icon>
    <h3 class="td-empty-state__title">{{ title }}</h3>
    <p v-if="description" class="td-empty-state__description">{{ description }}</p>
    <div v-if="$slots.default" class="td-empty-state__actions">
      <slot></slot>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Box, Search, WarningFilled, CirclePlus, Folder } from '@element-plus/icons-vue'

type Preset = 'no-data' | 'no-result' | 'error' | 'no-permission' | 'first-time'

const props = withDefaults(defineProps<{
  preset?: Preset
  title: string
  description?: string
  tone?: 'neutral' | 'primary' | 'warning' | 'danger'
}>(), {
  preset: 'no-data',
  tone: 'neutral',
  description: '',
})

const iconComponent = computed(() => {
  switch (props.preset) {
    case 'no-result': return Search
    case 'error': return WarningFilled
    case 'no-permission': return Folder
    // first-time 的意思是「这里还空着，去建第一个」；原来用 DataBoard
    // （数据看板那个投影幕图形），配在「还没有 API 密钥」上完全不搭
    case 'first-time': return CirclePlus
    case 'no-data':
    default: return Box
  }
})
</script>

<style scoped lang="scss">
.td-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  // 方向 A：空状态不该占掉半屏。原来是 72px 的圆角色块 + 32px 图标 + 上下 48px 内边距，
  // 工作台上两个空状态各占约 400px 高，把真正有内容的区块挤到折线以下。
  // 现在收成一行提示，图标降为同字号的弱色符号。
  padding: var(--td-space-8) var(--td-space-6);
  gap: var(--td-space-1);
  text-align: center;
  color: var(--td-text-secondary);

  &__icon {
    width: auto;
    height: auto;
    border-radius: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: var(--td-space-2);
    // 20px + disabled 色在白底上几乎看不见，空状态退化成一行孤零零的灰字，
    // 和 Element 默认的「暂无数据」没有区别。放大一档、提到 placeholder 色：
    // 仍然是弱色，但至少能看出这里是"空"而不是"没加载出来"。
    font-size: 28px;
    color: var(--td-text-placeholder);
    background: transparent;
    box-shadow: none;
    transition: var(--td-transition-color);
  }

  &__title {
    font-size: var(--td-font-md);
    font-weight: var(--td-weight-medium);
    color: var(--td-text-regular);
    margin: 0;
  }

  &__description {
    font-size: var(--td-font-sm);
    color: var(--td-text-placeholder);
    margin: 0;
    max-width: 380px;
    line-height: var(--td-leading-normal);
  }

  &__actions {
    margin-top: var(--td-space-2);
    display: flex;
    gap: var(--td-space-2);
  }

  // tone 只改图标颜色，不加底色：图标已经是 width/height auto + 无圆角，
  // 再补一层 background 就成了一块方角色片，而且四个空状态里只有一个有，
  // 看上去像渲染坏了
  &--primary &__icon { color: var(--td-color-primary); }
  &--warning &__icon { color: var(--td-color-warning); }
  &--danger &__icon  { color: var(--td-color-danger); }
}
</style>
