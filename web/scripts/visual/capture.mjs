#!/usr/bin/env node
/**
 * 视觉回归：采集截图。
 *
 * 用途：改样式 / 拆组件这类"不该有视觉变化"的改动，改前跑一次存基线，
 * 改后再跑一次，用 diff.mjs 比对。逐像素比对能抓到肉眼在整页截图上
 * 根本看不出来的偏移（实践中 0.03% 的差异就已经是真问题了）。
 *
 * 用法：
 *   node scripts/visual/capture.mjs <输出目录>
 *
 * 环境变量：
 *   TD_BASE       站点地址，默认 http://127.0.0.1:5173
 *   TD_USER       登录用户名
 *   TD_PASS       登录密码
 *   TD_TOKEN      直接给 access token（给了就不走登录）
 *   TD_API        后端地址，默认与 TD_BASE 同源
 *   TD_CHROME     Chrome 可执行文件路径
 *   TD_ISSUE_KEY  详情页取样用的工单号，默认取列表第一条
 *   TD_PROJECT_KEY 项目页取样用的项目标识，默认取列表第一个
 *   TD_ONLY       只拍这几个页面，逗号分隔（名字取自 pages.json）
 */
import puppeteer from 'puppeteer-core'
import { readFileSync, mkdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const outDir = process.argv[2]
if (!outDir) {
  console.error('用法: node scripts/visual/capture.mjs <输出目录>')
  process.exit(1)
}

const BASE = (process.env.TD_BASE || 'http://127.0.0.1:5173').replace(/\/$/, '')
const API = (process.env.TD_API || BASE).replace(/\/$/, '')
const CHROME = process.env.TD_CHROME || defaultChrome()

function defaultChrome() {
  if (process.platform === 'darwin') return '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
  if (process.platform === 'win32') return 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'
  return '/usr/bin/google-chrome'
}

async function api(path, init) {
  const res = await fetch(API + path, init)
  const body = await res.json().catch(() => null)
  if (!res.ok || body?.code === undefined) throw new Error(`${path} -> ${res.status}`)
  return body
}

async function getToken() {
  if (process.env.TD_TOKEN) return process.env.TD_TOKEN
  const user = process.env.TD_USER
  const pass = process.env.TD_PASS
  if (!user || !pass) throw new Error('需要 TD_TOKEN，或同时给 TD_USER / TD_PASS')
  const body = await api('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: user, password: pass }),
  })
  if (!body.data?.access_token) throw new Error('登录失败: ' + (body.message || '未知原因'))
  return body.data.access_token
}

/** 详情页要一个真实存在的 key，写死会在别的环境上 404 */
async function resolveSamples(token) {
  const auth = { headers: { Authorization: 'Bearer ' + token } }
  let issueKey = process.env.TD_ISSUE_KEY
  let projectKey = process.env.TD_PROJECT_KEY
  if (!issueKey) {
    const r = await api('/api/v1/issues?page=1&page_size=1', auth).catch(() => null)
    issueKey = r?.data?.items?.[0]?.issue_key
  }
  if (!projectKey) {
    const r = await api('/api/v1/projects?page=1&page_size=1', auth).catch(() => null)
    projectKey = r?.data?.items?.[0]?.project_key
  }
  return { issueKey, projectKey }
}

const config = JSON.parse(readFileSync(resolve(here, 'pages.json'), 'utf8'))
const only = process.env.TD_ONLY ? new Set(process.env.TD_ONLY.split(',').map(s => s.trim())) : null

mkdirSync(outDir, { recursive: true })

const token = await getToken()
const me = await api('/api/v1/users/me', { headers: { Authorization: 'Bearer ' + token } })
const { issueKey, projectKey } = await resolveSamples(token)

const browser = await puppeteer.launch({
  executablePath: CHROME,
  headless: 'new',
  // font-render-hinting=none：不同机器的字体微调会让同一份代码渲染出差异，
  // 关掉它比对才有意义
  args: ['--no-sandbox', '--disable-gpu', '--hide-scrollbars', '--font-render-hinting=none'],
  defaultViewport: { width: 1600, height: 1000, deviceScaleFactor: 1 },
})
const page = await browser.newPage()

let shot = 0
let skipped = 0
for (const theme of config.themes) {
  await page.goto(BASE + '/login', { waitUntil: 'domcontentloaded' })
  await page.evaluate((t, user, th) => {
    localStorage.setItem('token', t)
    localStorage.setItem('refresh_token', t)
    localStorage.setItem('user', JSON.stringify(user))
    localStorage.setItem('theme-mode', th)
    localStorage.setItem('app-locale', 'zh-CN')
  }, token, me.data, theme)

  for (const item of config.pages) {
    if (only && !only.has(item.name)) continue
    const path = item.path
      .replace('{ISSUE_KEY}', issueKey || '')
      .replace('{PROJECT_KEY}', projectKey || '')
    if (path.includes('{')) { console.log('跳过', item.name, '（缺少取样数据）'); skipped++; continue }
    try {
      await page.goto(BASE + path, { waitUntil: 'networkidle2', timeout: 40000 })
      // 等异步区块渲染完；不等的话首屏骨架屏会被拍进去，两次跑的结果不稳定
      await new Promise(r => setTimeout(r, 2200))

      if (item.tabs) {
        // 带 tabs 的页面逐个点开拍：默认只拍到第一个 tab，
        // 其余内容根本没渲染，改坏了也看不出来
        const sel = item.tabs === true ? '.el-tabs__item' : item.tabs
        const count = await page.$$eval(sel, els => els.length).catch(() => 0)
        for (let i = 0; i < count; i++) {
          const els = await page.$$(sel)
          if (!els[i]) break
          const label = (await page.evaluate(e => e.textContent.trim(), els[i]))
            .replace(/[^\w\u4e00-\u9fa5]/g, '') || String(i)
          await els[i].click()
          await new Promise(r => setTimeout(r, 1600))
          await page.screenshot({ path: `${outDir}/${item.name}-${i}-${label}-${theme}.png`, fullPage: true })
          shot++
        }
      } else {
        await page.screenshot({ path: `${outDir}/${item.name}-${theme}.png`, fullPage: true })
        shot++
      }
    } catch (e) {
      console.log('✗', item.name, theme, String(e).slice(0, 60))
    }
  }
}

await browser.close()
console.log(`已采集 ${shot} 张${skipped ? `，跳过 ${skipped} 个` : ''} → ${outDir}`)
