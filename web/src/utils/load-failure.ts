// 「这一页的数据没加载出来」的全局信号。
//
// 原来接口挂掉只弹一条 toast，三秒后自己消失，剩下一张「暂无数据」的空表 ——
// 用户看到的是「本来就没有数据」，而不是「加载失败了」，而且全站没有任何
// 重试入口（CLAUDE.md 3.8 要求错误态说清「发生了什么 / 为什么 / 怎么办」
// 并给重试）。
//
// 记在一个地方，由外壳统一显示一条常驻横幅，切路由时自动清掉。
import { ref } from 'vue'

export type LoadFailureKind = 'server' | 'network'

export const loadFailure = ref<LoadFailureKind | null>(null)

export const reportLoadFailure = (kind: LoadFailureKind) => {
  // 网络不通比服务端 500 更根本，同时发生时报前者
  if (kind === 'network' || loadFailure.value === null) {
    loadFailure.value = kind
  }
}

export const clearLoadFailure = () => {
  loadFailure.value = null
}
