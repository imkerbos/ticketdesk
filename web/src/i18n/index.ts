/**
 * 国际化入口。
 *
 * 设计要点：
 * - 语言选择持久化在 localStorage，刷新后不回退；未选择过时按浏览器语言猜测，
 *   猜不中回落到简体中文（这是产品的基准语言）。
 * - Element Plus 自己的内置文案（表格空态、分页、日期选择器等）必须跟着一起切，
 *   否则会出现"界面是英文、组件是中文"的割裂。二者的映射放在 elementLocale。
 * - 后端返回的错误信息目前仍是中文硬编码，切到英文时那部分不会跟着变；
 *   需要后端支持 Accept-Language 才能闭环，见 README 的待办。
 */
import { createI18n } from 'vue-i18n'
import zhCnElement from 'element-plus/es/locale/lang/zh-cn'
import enUsElement from 'element-plus/es/locale/lang/en'

import zhCN from './locales/zh-CN'
import enUS from './locales/en-US'

export const SUPPORTED_LOCALES = ['zh-CN', 'en-US'] as const
export type AppLocale = (typeof SUPPORTED_LOCALES)[number]

export const DEFAULT_LOCALE: AppLocale = 'zh-CN'
const STORAGE_KEY = 'app-locale'

/** Element Plus 语言包映射，随应用语言一起切换 */
export const elementLocaleMap = {
  'zh-CN': zhCnElement,
  'en-US': enUsElement,
}

function isSupported(v: string | null | undefined): v is AppLocale {
  return !!v && (SUPPORTED_LOCALES as readonly string[]).includes(v)
}

/** 读取初始语言：本地存储 > 浏览器语言 > 默认 */
export function resolveInitialLocale(): AppLocale {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (isSupported(stored)) return stored
  } catch {
    // 隐私模式 / 禁用存储时读不到，按浏览器语言继续
  }
  const nav = typeof navigator !== 'undefined' ? navigator.language : ''
  if (nav.toLowerCase().startsWith('zh')) return 'zh-CN'
  if (nav.toLowerCase().startsWith('en')) return 'en-US'
  return DEFAULT_LOCALE
}

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: resolveInitialLocale(),
  fallbackLocale: DEFAULT_LOCALE,
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS,
  },
  // 缺 key 时不在控制台刷屏，但保留 fallback 行为；
  // 真正的缺失由 scripts/check-i18n.mjs 在构建前拦截
  missingWarn: false,
  fallbackWarn: false,
})

/** 切换语言并持久化，同时更新 <html lang> 以便屏幕阅读器与浏览器正确处理 */
export function setLocale(locale: AppLocale) {
  i18n.global.locale.value = locale
  try {
    localStorage.setItem(STORAGE_KEY, locale)
  } catch {
    // 存不了就只在本次会话生效，不阻断切换
  }
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('lang', locale)
  }
}

export function getLocale(): AppLocale {
  return i18n.global.locale.value as AppLocale
}

/**
 * 按账号里存的偏好语言切换
 *
 * 登录时调用。账号设置优先于本机 localStorage：在另一台设备上改过语言，
 * 这台一登录就该跟上；本机切换会同步写回账号，两边不会长期打架。
 * 值为空或不认识时保持当前语言不动 —— 空表示"跟随站点设置"，不是"切回中文"。
 */
export function applyAccountLocale(locale?: string | null) {
  if (isSupported(locale)) setLocale(locale)
}
