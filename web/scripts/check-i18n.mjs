#!/usr/bin/env node
/**
 * 语言包一致性检查。
 *
 * 存在的原因：新增文案时很容易只加中文，英文那边悄悄缺 key。
 * vue-i18n 缺 key 时会回落到基准语言——界面不报错，但英文会话里
 * 会突然冒出一句中文，而且没有任何地方会提示。
 * 这里在构建前把这类漂移变成硬失败。
 *
 * 用法：node scripts/check-i18n.mjs
 */
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve, join, relative } from 'node:path'

const here = dirname(fileURLToPath(import.meta.url))
const localesDir = resolve(here, '../src/i18n/locales')
const BASE = 'zh-CN'
const LOCALES = ['zh-CN', 'en-US']

/** 从 TS 语言包里抽出所有叶子 key（不执行代码，避免引入构建依赖） */
function extractKeys(file) {
  const src = readFileSync(file, 'utf8')
  const keys = new Set()
  const stack = []
  // 逐行扫描：`ident: {` 入栈，`}` 出栈，`ident: '...'` 记为叶子
  for (const rawLine of src.split('\n')) {
    const line = rawLine.trim()
    if (!line || line.startsWith('//') || line.startsWith('*') || line.startsWith('/*')) continue

    const open = line.match(/^'?([\w-]+)'?\s*:\s*\{$/)
    if (open) { stack.push(open[1]); continue }

    if (line.startsWith('}')) { stack.pop(); continue }

    const leaf = line.match(/^'?([\w-]+)'?\s*:\s*['"`]/)
    if (leaf) keys.add([...stack, leaf[1]].join('.'))
  }
  return keys
}

const sets = Object.fromEntries(
  LOCALES.map((l) => [l, extractKeys(resolve(localesDir, `${l}.ts`))]),
)

let failed = false
const base = sets[BASE]

for (const locale of LOCALES) {
  if (locale === BASE) continue
  const missing = [...base].filter((k) => !sets[locale].has(k)).sort()
  const extra = [...sets[locale]].filter((k) => !base.has(k)).sort()

  if (missing.length) {
    failed = true
    console.error(`\n[${locale}] 缺少 ${missing.length} 个 key（会静默回落到 ${BASE}）:`)
    missing.forEach((k) => console.error(`  - ${k}`))
  }
  if (extra.length) {
    failed = true
    console.error(`\n[${locale}] 多出 ${extra.length} 个 ${BASE} 没有的 key（多半是改名后没同步）:`)
    extra.forEach((k) => console.error(`  + ${k}`))
  }
}

// ---- 检查 3：源码里 t('...') 引用的 key 必须在语言包里存在 ----
//
// 由来：改 key 名时漏改调用点，界面上就会直接显示 "project.settings"
// 这种原始 key。语言包两边一致并不能发现这类问题 —— 两边都没有那个 key。
// 只查静态字面量；t(`a.${b}`) 这种动态拼接跳过，本来也查不了。
const srcDir = resolve(here, '../src')
const referenced = []
function walkSrc(dir) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) {
      if (!/node_modules|i18n\/locales/.test(full)) walkSrc(full)
      continue
    }
    if (!/\.(vue|ts)$/.test(name)) continue
    const src = readFileSync(full, 'utf8')
    for (const m of src.matchAll(/[^\w]t\(\s*'([a-zA-Z][\w.]*)'\s*[,)]/g)) {
      referenced.push({ file: relative(resolve(here, '..'), full), key: m[1] })
    }
  }
}
walkSrc(srcDir)

const unknown = referenced.filter((r) => !base.has(r.key))
if (unknown.length) {
  failed = true
  const seen = new Set()
  console.error(`\n源码引用了 ${unknown.length} 处语言包里不存在的 key（界面会直接显示原始 key）:`)
  for (const u of unknown) {
    const sig = u.file + u.key
    if (seen.has(sig)) continue
    seen.add(sig)
    console.error(`  ${u.file} → ${u.key}`)
  }
}

// ---- 检查 4：每条文案都要能被 vue-i18n 的消息编译器编过 ----
//
// 由来：vue-i18n 的文案不是纯字符串，是一门小语言 —— @ 是链接消息
// （@:key）的起始符号，{} 是插值。文案里直接写字面量 @ 或大括号，
// 编译器会报错，而且**不会**报到界面上：只在浏览器 console 里出现，
// 用到这条文案的组件整个渲染不出来。
//
// 本项目已经被这两种各坑过一次：
//   - 「日报 @ 全员」让项目设置的「通知渠道」tab 整个白屏
//   - 「{.data.setup-token}」让初始化向导的令牌输入框静默消失
// 都是不点到那个页面就发现不了。
//
// 这里不用正则猜，直接调 vue-i18n 运行时用的同一个编译器。
const { baseCompile } = await import('@intlify/message-compiler')

/** 从语言包里抽出「key → 文案」，值支持单/双引号与反引号 */
function extractEntries(file) {
  const src = readFileSync(file, 'utf8')
  const out = []
  src.split('\n').forEach((line, i) => {
    const m = line.match(/^\s*'?([\w-]+)'?\s*:\s*(['"`])(.*)\2\s*,?\s*$/)
    if (m) out.push({ line: i + 1, key: m[1], value: m[3] })
  })
  return out
}

const compileErrors = []
for (const locale of LOCALES) {
  for (const { line, key, value } of extractEntries(resolve(localesDir, `${locale}.ts`))) {
    const errs = []
    try {
      baseCompile(value, { onError: (e) => errs.push(e.message) })
    } catch (e) {
      errs.push(String(e?.message || e))
    }
    if (errs.length) {
      compileErrors.push(`${locale}.ts:${line}  ${key}\n      ${errs[0]}\n      → ${value.slice(0, 70)}`)
    }
  }
}

if (compileErrors.length) {
  failed = true
  console.error(`\n${compileErrors.length} 条文案无法被 vue-i18n 编译（用到它的组件会整个渲染失败，且只在 console 报错）:`)
  compileErrors.forEach((e) => console.error('  ' + e))
  console.error("\n  字面量 @ 写成 {'@'}，字面量大括号写成 {'{'} / {'}'}；含转义的行改用双引号包裹")
}

if (failed) {
  console.error('\n语言包检查未通过，构建终止。')
  process.exit(1)
}

console.log(
  `语言包一致：${LOCALES.join(' / ')}，各 ${base.size} 个 key；` +
    `源码 ${referenced.length} 处引用全部可解析。`,
)
