import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getBrandConfig } from '@/api/system'
import type { BrandConfig } from '@/types/system'
import { i18n } from '@/i18n'

// store 在组件外初始化，默认文案只能走 i18n 全局实例
const t = i18n.global.t

export const useBrandStore = defineStore('brand', () => {
  const systemName = ref('TicketDesk')
  const systemDescription = ref(t('common.defaultSystemDesc'))
  const copyrightText = ref('© 2026 TicketDesk. All rights reserved.')
  const logoUrl = ref('')
  const faviconUrl = ref('')
  const loginTitle = ref(t('common.defaultLoginTitle'))
  const loginDescription = ref(
    t('common.defaultLoginDesc'),
  )
  const loaded = ref(false)

  async function loadBrandConfig() {
    try {
      const res = await getBrandConfig()
      const config = res.data.data
      updateBrand(config)
      loaded.value = true
    } catch {
      // 品牌配置获取失败使用默认值
      loaded.value = true
    }
  }

  function updateBrand(config: BrandConfig) {
    systemName.value = config.system_name || 'TicketDesk'
    systemDescription.value = config.system_description || t('common.defaultSystemDesc')
    copyrightText.value = config.copyright_text || '© 2026 TicketDesk. All rights reserved.'
    logoUrl.value = config.logo_url || ''
    faviconUrl.value = config.favicon_url || ''
    loginTitle.value = config.login_title || t('common.defaultLoginTitle')
    loginDescription.value =
      config.login_description ||
      t('common.defaultLoginDesc')

    // 动态更新 Favicon
    applyFavicon(config.favicon_url)
  }

  function applyFavicon(url: string) {
    const link = document.querySelector<HTMLLinkElement>("link[rel='icon']")
    if (link) {
      link.href = url || '/favicon.svg'
    }
  }

  return {
    systemName,
    systemDescription,
    copyrightText,
    logoUrl,
    faviconUrl,
    loginTitle,
    loginDescription,
    loaded,
    loadBrandConfig,
    updateBrand,
  }
})
