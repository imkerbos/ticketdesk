#!/usr/bin/env node
// 色板可辨性校验 —— 六项
//
// 直接解析 web/src/styles/theme.scss，验的是代码里真实生效的值，
// 不是文档里抄的副本，所以改了 token 忘了跑这里会被 CI 拦住。
//
// 为什么不用肉眼：色觉障碍下相邻色收敛、暗色下彩度塌陷，这两件事
// 正常视力在亮色屏幕上完全看不出来。
//
// 用法：node scripts/palette-check.mjs [--candidate]
//   --candidate  额外用 Apple 系统色跑一遍，用于迁移前评估

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..')
const THEME = join(ROOT, 'web/src/styles/theme.scss')

/* ── 色彩转换 ─────────────────────────────────────────── */

const hex2rgb = (h) => {
  const s = h.trim().replace('#', '')
  const n = s.length === 3 ? s.split('').map((c) => c + c).join('') : s
  return [0, 2, 4].map((i) => parseInt(n.slice(i, i + 2), 16) / 255)
}

const toLinear = (c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
const linearRgb = (hex) => hex2rgb(hex).map(toLinear)

// Oklab (Björn Ottosson)
function oklab([r, g, b]) {
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b)
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b)
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b)
  return [
    0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
    1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
    0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
  ]
}

const oklch = (hex) => {
  const [L, a, b] = oklab(linearRgb(hex))
  return { L, C: Math.hypot(a, b), h: (Math.atan2(b, a) * 180) / Math.PI }
}

// Oklab 欧氏距离，×100 便于读数
const deltaE = (hexA, hexB, sim = (x) => x) => {
  const A = oklab(sim(linearRgb(hexA)))
  const B = oklab(sim(linearRgb(hexB)))
  return Math.hypot(A[0] - B[0], A[1] - B[1], A[2] - B[2]) * 100
}

// WCAG 相对亮度与对比度
const luminance = (hex) => {
  const [r, g, b] = linearRgb(hex)
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}
const contrast = (a, b) => {
  const [x, y] = [luminance(a), luminance(b)].sort((m, n) => n - m)
  return (x + 0.05) / (y + 0.05)
}

/* ── 色觉障碍模拟（Viénot 1999，线性 RGB 下的投影矩阵）────── */

const apply = (M) => ([r, g, b]) => [
  M[0][0] * r + M[0][1] * g + M[0][2] * b,
  M[1][0] * r + M[1][1] * g + M[1][2] * b,
  M[2][0] * r + M[2][1] * g + M[2][2] * b,
]

const CVD = {
  protan: apply([
    [0.1121, 0.8853, -0.0005],
    [0.1127, 0.8897, -0.0001],
    [0.0045, 0.0, 1.0019],
  ]),
  deutan: apply([
    [0.292, 0.7054, -0.0003],
    [0.2934, 0.7089, 0.0004],
    [-0.0209, 0.0257, 0.9924],
  ]),
  tritan: apply([
    [1.0, 0.1502, -0.1387],
    [0.0, 0.8437, 0.1771],
    [0.0, 0.2718, 0.7286],
  ]),
}

/* ── 解析 theme.scss ──────────────────────────────────── */

function parseTokens(css, selector) {
  // 同一个选择器可能出现多次（亮色基准 + 覆盖块），全部合并，后者覆盖前者
  const out = {}
  const re = new RegExp(`${selector}\\s*\\{`, 'g')
  let m
  while ((m = re.exec(css))) {
    let depth = 1
    let i = m.index + m[0].length
    const start = i
    while (i < css.length && depth > 0) {
      if (css[i] === '{') depth++
      else if (css[i] === '}') depth--
      i++
    }
    const body = css.slice(start, i - 1)
    for (const d of body.matchAll(/(--td-[\w-]+)\s*:\s*(#[0-9a-fA-F]{3,8})\s*;/g)) {
      out[d[1]] = d[2].toLowerCase()
    }
  }
  return out
}

const css = readFileSync(THEME, 'utf8')
const LIGHT = parseTokens(css, ':root')
const DARK = { ...LIGHT, ...parseTokens(css, 'html\\.dark') }

/* ── 阈值 ─────────────────────────────────────────────
 * 标定方式：先让现行色板（已人工验收、线上在用）全过，再用同一套
 * 阈值去卡候选色板。阈值不是拍脑袋定的下限，是"现状即基线"。
 * 放宽任何一条之前，先问是不是校验器写错了。
 */
const T = {
  catLightBand: [0.48, 0.78], // 分类色明度带（亮色主题）
  catDarkBand: [0.55, 0.82],  // 暗色主题
  chromaFloor: 0.055,          // 彩度下限：再低就是"脏灰"
  // 正常视力分两档：图例里相邻两项挨着显示，最容易被读成同一个，
  // 卡得严；隔开的两项有其它颜色垫在中间，只要不塌到一起即可。
  // CLAUDE.md 那句"顺序本身是结果的一部分"说的就是这件事。
  sepNormalAdjacent: 15,       // 正常视力：相邻两类
  sepNormalAny: 8,             // 正常视力：任意两类
  sepAdjacent: 9,              // 色觉障碍下：相邻两类之间
  textContrast: 4.5,           // 正文对比度（WCAG AA）
  uiContrast: 3.0,             // 语义色作为图形/边框
  hueDrift: 30,                // 同一 token 亮暗之间允许的色相漂移
}

const CATS = [1, 2, 3, 4, 5, 6].map((i) => `--td-cat-${i}`)
const SEMANTIC = [
  '--td-color-primary',
  '--td-color-danger',
  '--td-color-warning',
  '--td-color-success',
  '--td-color-info',
]

/* ── 六项校验 ─────────────────────────────────────────── */

const results = []
const record = (name, rows) => {
  const fails = rows.filter((r) => !r.ok)
  results.push({ name, rows, fails })
}

function checkBand(tokens, band, theme) {
  return CATS.map((k) => {
    const { L } = oklch(tokens[k])
    return {
      ok: L >= band[0] && L <= band[1],
      msg: `${theme} ${k} ${tokens[k]}  L=${L.toFixed(3)} (允许 ${band[0]}–${band[1]})`,
    }
  })
}

function checkChroma(tokens, theme) {
  return CATS.map((k) => {
    const { C } = oklch(tokens[k])
    return {
      ok: C >= T.chromaFloor,
      msg: `${theme} ${k} ${tokens[k]}  C=${C.toFixed(3)} (下限 ${T.chromaFloor})`,
    }
  })
}

function checkSeparationNormal(tokens, theme) {
  const rows = []
  for (let i = 0; i < CATS.length; i++) {
    for (let j = i + 1; j < CATS.length; j++) {
      const d = deltaE(tokens[CATS[i]], tokens[CATS[j]])
      const adjacent = j === i + 1
      const floor = adjacent ? T.sepNormalAdjacent : T.sepNormalAny
      rows.push({
        ok: d >= floor,
        msg: `${theme} cat-${i + 1} ↔ cat-${j + 1}${adjacent ? ' (相邻)' : ''}  ΔE=${d.toFixed(1)} (下限 ${floor})`,
      })
    }
  }
  return rows
}

function checkSeparationCVD(tokens, theme) {
  const rows = []
  for (const [type, sim] of Object.entries(CVD)) {
    // 只卡相邻：图例里相邻两项最容易被误读成同一个
    for (let i = 0; i < CATS.length - 1; i++) {
      const d = deltaE(tokens[CATS[i]], tokens[CATS[i + 1]], sim)
      rows.push({
        ok: d >= T.sepAdjacent,
        msg: `${theme} ${type} cat-${i + 1} ↔ cat-${i + 2}  ΔE=${d.toFixed(1)} (下限 ${T.sepAdjacent})`,
      })
    }
  }
  return rows
}

function checkContrast(tokens, theme) {
  const rows = []
  const card = tokens['--td-bg-card']
  const page = tokens['--td-bg-page']

  for (const k of ['--td-text-primary', '--td-text-secondary']) {
    for (const [bgName, bg] of [['card', card], ['page', page]]) {
      const c = contrast(tokens[k], bg)
      rows.push({
        ok: c >= T.textContrast,
        msg: `${theme} ${k} on ${bgName}  ${c.toFixed(2)}:1 (下限 ${T.textContrast})`,
      })
    }
  }
  for (const k of SEMANTIC) {
    const c = contrast(tokens[k], card)
    rows.push({
      ok: c >= T.uiContrast,
      msg: `${theme} ${k} on card  ${c.toFixed(2)}:1 (下限 ${T.uiContrast})`,
    })
  }
  return rows
}

function checkThemePairs() {
  return [...SEMANTIC, ...CATS].map((k) => {
    const a = oklch(LIGHT[k])
    const b = oklch(DARK[k])
    // 近中性色（灰）的色相是数值噪声：彩度趋近 0 时 atan2 的结果不稳定，
    // 拿它判漂移会得到假失败。这类 token 只要两头都还是灰就算通过。
    if (a.C < T.chromaFloor && b.C < T.chromaFloor) {
      return { ok: true, msg: `${k}  亮 ${LIGHT[k]} ↔ 暗 ${DARK[k]}  近中性，跳过色相判定` }
    }
    let d = Math.abs(a.h - b.h)
    if (d > 180) d = 360 - d
    return {
      ok: d <= T.hueDrift,
      msg: `${k}  亮 ${LIGHT[k]} ↔ 暗 ${DARK[k]}  色相差 ${d.toFixed(1)}° (上限 ${T.hueDrift}°)`,
    }
  })
}

record('1. 明度带', [
  ...checkBand(LIGHT, T.catLightBand, '亮'),
  ...checkBand(DARK, T.catDarkBand, '暗'),
])
record('2. 彩度下限', [...checkChroma(LIGHT, '亮'), ...checkChroma(DARK, '暗')])
record('3. 色觉障碍相邻分离度', [
  ...checkSeparationCVD(LIGHT, '亮'),
  ...checkSeparationCVD(DARK, '暗'),
])
record('4. 正常视力分离度', [
  ...checkSeparationNormal(LIGHT, '亮'),
  ...checkSeparationNormal(DARK, '暗'),
])
record('5. 对比度', [...checkContrast(LIGHT, '亮'), ...checkContrast(DARK, '暗')])
record('6. 亮暗一致性', checkThemePairs())

/* ── 输出 ─────────────────────────────────────────────── */

const verbose = process.argv.includes('--verbose')
let failed = 0

for (const r of results) {
  const bad = r.fails.length
  failed += bad
  console.log(`${bad ? '✗' : '✓'} ${r.name}  (${r.rows.length - bad}/${r.rows.length})`)
  for (const row of r.rows) {
    if (!row.ok) console.log(`    ✗ ${row.msg}`)
    else if (verbose) console.log(`      ${row.msg}`)
  }
}

console.log(failed ? `\n六项校验未通过：${failed} 条不合格` : '\n六项校验全过')
process.exit(failed ? 1 : 0)
