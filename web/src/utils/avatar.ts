// 头像的身份色：同一个人每次进页面颜色要一致，所以按名字取哈希而不是随机。
//
// 身份色只染字母，不做实心底 —— 一屏几十个饱和实心圆是表格里最重的东西，
// 而它们只承载「谁」，不承载状态。底色统一走中性淡底（见 _apple.scss 的 .ava）。
const AVA_COLORS = ['#8e8e93', '#34595f', '#6a5acd', '#b8860b', '#2f6f5e', '#8a5a44']

/** 按名字取一个稳定的身份色，用于头像字母。 */
export const personColor = (name?: string): string => {
  if (!name) return AVA_COLORS[0]
  let h = 0
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) % 997
  return AVA_COLORS[h % AVA_COLORS.length]
}
