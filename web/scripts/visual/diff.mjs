#!/usr/bin/env node
/**
 * 视觉回归：比对两批截图。
 *
 * 用法：
 *   node scripts/visual/diff.mjs <基线目录> <改后目录> [--out 差异图目录] [--threshold 0.02]
 *
 * 退出码 0 通过，1 有超出阈值的差异或尺寸不一致。
 *
 * 关于阈值：页面里只要有随时间变化的内容（相对时间、SLA 剩余分钟、
 * 表单里的"当前时间"默认值），同一份代码连跑两次也不会完全一致。
 * 先跑两次基线量出这个噪声底，阈值取它的几倍。实测本项目噪声底
 * 约 0.008%，默认 0.02% 足够区分真回归。
 */
import { readdirSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { PNG } from 'pngjs'
import pixelmatch from 'pixelmatch'

const args = process.argv.slice(2)
const [dirA, dirB] = args.filter(a => !a.startsWith('--'))
const flag = (name, def) => {
  const i = args.indexOf('--' + name)
  return i >= 0 ? args[i + 1] : def
}
if (!dirA || !dirB) {
  console.error('用法: node scripts/visual/diff.mjs <基线目录> <改后目录> [--out 差异图目录] [--threshold 0.02]')
  process.exit(2)
}
const outDir = flag('out', null)
const threshold = Number(flag('threshold', '0.02'))
if (outDir) mkdirSync(outDir, { recursive: true })

/** 差异像素按行带聚合：定位比总数有用得多，能直接指出是哪一块变了 */
function hotBands(diff, width, height, band = 40) {
  const bands = new Map()
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const i = (y * width + x) * 4
      if (diff[i] > 200 && diff[i + 1] < 100) {
        const k = Math.floor(y / band) * band
        bands.set(k, (bands.get(k) || 0) + 1)
      }
    }
  }
  return [...bands.entries()].sort((p, q) => q[1] - p[1]).slice(0, 3)
}

const files = readdirSync(dirA).filter(f => f.endsWith('.png')).sort()
if (!files.length) { console.error('基线目录里没有 png:', dirA); process.exit(2) }

let failed = false
let worst = 0
const rows = []

for (const f of files) {
  if (!existsSync(`${dirB}/${f}`)) { rows.push([f, '改后缺失']); failed = true; continue }
  const a = PNG.sync.read(readFileSync(`${dirA}/${f}`))
  const b = PNG.sync.read(readFileSync(`${dirB}/${f}`))
  if (a.width !== b.width || a.height !== b.height) {
    // 尺寸变了说明布局高度变了，多半是内容变了而不是样式变了 —— 先查数据漂移
    rows.push([f, `尺寸 ${a.width}x${a.height} → ${b.width}x${b.height}`])
    failed = true
    continue
  }
  const out = new PNG({ width: a.width, height: a.height })
  const n = pixelmatch(a.data, b.data, out.data, a.width, a.height, { threshold: 0.1 })
  if (!n) continue
  const pct = (n / (a.width * a.height)) * 100
  worst = Math.max(worst, pct)
  const where = hotBands(out.data, a.width, a.height).map(([y, c]) => `y≈${y}(${c}px)`).join(' ')
  rows.push([f, `${n} px  ${pct.toFixed(4)}%  ${where}`])
  if (pct > threshold) failed = true
  if (outDir) writeFileSync(`${outDir}/${f}`, PNG.sync.write(out))
}

if (!rows.length) {
  console.log(`✅ ${files.length} 张全部逐像素一致`)
} else {
  console.log(`比对 ${files.length} 张，${rows.length} 张有差异（阈值 ${threshold}%）:`)
  for (const [f, d] of rows) console.log('  ', f.padEnd(30), d)
  if (outDir) console.log(`差异图已输出到 ${outDir}`)
}
console.log(failed ? `❌ 超出阈值，最大 ${worst.toFixed(4)}%` : `✅ 通过，最大差异 ${worst.toFixed(4)}%`)
process.exit(failed ? 1 : 0)
