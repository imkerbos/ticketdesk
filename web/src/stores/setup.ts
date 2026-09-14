import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getSetupStatus } from '@/api/setup'

/**
 * 初始化状态
 *
 * 默认为 true（已初始化）：拉取失败时不该把一个正常运行的实例
 * 拦在向导页上——那比少弹一次向导严重得多。真正未初始化时后端
 * 会对业务接口返回 503，用户不会误以为系统能用。
 */
export const useSetupStore = defineStore('setup', () => {
  const initialized = ref(true)
  const loaded = ref(false)

  const fetchStatus = async () => {
    try {
      const res = await getSetupStatus()
      initialized.value = res.data.data.initialized !== false
    } catch {
      initialized.value = true
    } finally {
      loaded.value = true
    }
  }

  /** 向导提交成功后本地置位，省一次往返 */
  const markInitialized = () => {
    initialized.value = true
  }

  return { initialized, loaded, fetchStatus, markInitialized }
})
