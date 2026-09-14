<template>
  <div class="td-chart" :style="{ height: height + 'px' }">
    <!-- 不挂 echarts 的内置 dark 主题：它会带一个 backgroundColor: #100C2A 深紫底，
         在深灰卡片里就是一块发紫的色块。本组件的颜色全部从 --td-* 读，
         明暗自动跟随，用不上那套主题。 -->
    <VChart
      v-if="items.length"
      class="td-chart__canvas"
      :option="option"
      autoresize
    />
    <div v-else class="td-chart__empty">{{ t('common.noData') }}</div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { useThemeStore } from '@/stores/theme'

const { t } = useI18n()

// 只注册用到的模块，避免把整个 echarts 打进包里
use([BarChart, GridComponent, TooltipComponent, CanvasRenderer])

interface ChartItem {
  name: string
  value: number
  ratio?: number
  /** 指定该项的颜色；不传则按顺序取分类色阶 */
  color?: string
  /** 条右侧直接标注的文本；不传则用 value + unit。工时这类要格式化成 "1d 7h" 的走这个 */
  valueText?: string
}

const props = withDefaults(defineProps<{
  items: ChartItem[]
  /** 数值单位后缀，如 "条"、"分钟" */
  unit?: string
  height?: number
}>(), { unit: '', height: 260 })

const themeStore = useThemeStore()

/** 从 CSS 变量读色，保证图表与全站 token 同源、且随主题切换 */
function cssVar(name: string, fallback: string): string {
  if (typeof window === 'undefined') return fallback
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return v || fallback
}

/**
 * 把 `var(--x)` 解析成真实色值。
 *
 * echarts 走 canvas 渲染，拿到 `var(--td-color-danger)` 这种字符串时不会报错，
 * 只会静默画不出来——表现为"有轨道没有条"。凡是要交给 canvas 的颜色，
 * 都必须先在这里解析成 #rrggbb。
 */
function resolveColor(c: string | undefined, fallback: string): string {
  if (!c) return fallback
  const m = c.match(/^var\(\s*(--[\w-]+)\s*(?:,\s*(.+?))?\s*\)$/)
  if (!m) return c
  return cssVar(m[1], m[2]?.trim() || fallback)
}

const option = computed(() => {
  const ink = cssVar('--td-text-primary', '#16181d')
  const muted = cssVar('--td-text-secondary', '#666e7a')
  const line = cssVar('--td-border-color-light', '#eef0f2')
  const surface = cssVar('--td-bg-card', '#ffffff')
  // 分类色阶按固定顺序取用，不循环——超出 6 类的部分统一归入中性色，
  // 循环复用会让"第 7 类"和"第 1 类"看起来是同一个东西
  const cats = [1, 2, 3, 4, 5, 6].map((i) => cssVar(`--td-cat-${i}`, '#2f6fd0'))
  const neutral = cssVar('--td-color-info', '#5c6470')

  const names = props.items.map((i) => i.name)
  const values = props.items.map((i) => i.value)
  const max = Math.max(...values, 1)

  // 读一下主题状态，让切换明暗时这个 computed 重新求值（token 的计算值跟着变）
  void themeStore.isDark

  return {
    backgroundColor: 'transparent',
    // right 要放得下「数值 + 百分比」两段，56 只够数值本身，
    // 四位数的分钟数会把百分比挤出画布（「1890 63.」）
    grid: { left: 4, right: 96, top: 4, bottom: 4, containLabel: true },
    tooltip: {
      trigger: 'item',
      backgroundColor: surface,
      borderColor: line,
      borderWidth: 1,
      padding: [8, 12],
      textStyle: { color: ink, fontSize: 12 },
      formatter: (p: { name: string; value: number; dataIndex: number }) => {
        const it = props.items[p.dataIndex]
        const pct = it?.ratio != null ? ` · ${it.ratio.toFixed(1)}%` : ''
        return `${p.name}<br/><b>${p.value}</b>${props.unit}${pct}`
      },
    },
    xAxis: {
      type: 'value',
      max,
      // 轴与网格是背景信息，压到最弱
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { show: false },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'category',
      data: names,
      inverse: true,
      axisLine: { show: false },
      axisTick: { show: false },
      // 类目名用文本色，不用系列色——颜色只由色块承担身份
      axisLabel: { color: muted, fontSize: 12, margin: 10 },
    },
    series: [
      {
        type: 'bar',
        data: props.items.map((it, idx) => ({
          value: it.value,
          itemStyle: {
            color: resolveColor(it.color, idx < cats.length ? cats[idx] : neutral),
          },
        })),
        // 细条 + 数据端 4px 圆角，锚在基线上
        barWidth: 10,
        itemStyle: { borderRadius: [0, 4, 4, 0] },
        // 直接标注：每根条右侧给出数值，不必去查图例
        label: {
          show: true,
          position: 'right',
          color: ink,
          fontSize: 12,
          fontWeight: 500,
          formatter: (p: { dataIndex: number; value: number }) => {
            const it = props.items[p.dataIndex]
            // 之前这里只吐裸数字，unit 只进了 tooltip —— 工时那张图就成了
            // 「开发 1890 63.0%」，1890 是分钟数，页面别处却都写 "1d 7h"。
            const text = it?.valueText ?? `${p.value}${props.unit}`
            return it?.ratio != null ? `${text}  ${it.ratio.toFixed(1)}%` : text
          },
        },
        // 未填充部分给一条极淡的轨道，读者能感知"占比"而不是只看长度
        showBackground: true,
        backgroundStyle: { color: line, borderRadius: [0, 4, 4, 0] },
        emphasis: { focus: 'self', itemStyle: { opacity: 0.85 } },
      },
    ],
    animationDuration: 240,
  }
})
</script>

<style scoped lang="scss">
.td-chart {
  width: 100%;

  &__canvas {
    width: 100%;
    height: 100%;
  }

  &__empty {
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--td-text-placeholder);
    font-size: var(--td-font-sm);
  }
}
</style>
