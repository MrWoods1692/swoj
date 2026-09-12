// 统一 API 层：同源部署，登录态靠 HttpOnly cookie(swoj_token) 自动携带，
// CSRF 令牌从可读 cookie(swoj_csrf) 取出并回传 X-CSRF-Token 头。
import { reactive } from 'vue'

const BASE = ''

function cookie(name) {
  const m = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'))
  return m ? decodeURIComponent(m[2]) : ''
}

async function request(method, path, body) {
  const opts = { method, headers: {}, credentials: 'same-origin' }
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  if (method !== 'GET' && method !== 'HEAD') {
    const csrf = cookie('swoj_csrf')
    if (csrf) opts.headers['X-CSRF-Token'] = csrf
  }
  const res = await fetch(BASE + path, opts)
  let data
  try { data = await res.json() } catch { data = { code: res.status, msg: '响应解析失败' } }
  if (res.status === 401) { auth.user = null; throw new ApiError(data.msg || '请先登录', 401, data) }
  if (!res.ok || (data.code && data.code !== 200)) {
    throw new ApiError(data.msg || '请求失败', data.code || res.status, data)
  }
  return data.data
}

// buildQuery 把对象序列化为 URL 查询串；undefined/null/'' 跳过。
// 前端调用示例：api.get('/api/logs', { page: 1, user_id: 3 })
function buildQuery(obj) {
  if (!obj) return ''
  const usp = new URLSearchParams()
  for (const [k, v] of Object.entries(obj)) {
    if (v === undefined || v === null || v === '') continue
    usp.append(k, String(v))
  }
  const s = usp.toString()
  return s ? '?' + s : ''
}

export class ApiError extends Error {
  constructor(msg, code, payload) { super(msg); this.code = code; this.payload = payload }
}

export const api = {
  get: (p, q) => request('GET', typeof p === 'string' ? p + buildQuery(q) : p),
  post: (p, b) => request('POST', p, b ?? {}),
  put: (p, b) => request('PUT', p, b ?? {}),
  del: (p) => request('DELETE', p),
  cookie,
}

// 当前登录用户，App 启动时经 /api/auth/me 填充。null 表示未登录。
// 必须做成响应式：auth.me() 异步完成后 App 与顶栏、协议门禁都依赖它重渲染，
// 普通对象赋值不会触发更新，登录态会一直显示为未登录。
export const auth = reactive({
  user: null,
  get isAdmin() { return ['admin', 'super', 'superadmin'].includes(this.user?.role) },
  // 老师与管理员都可维护资料，用于资料页显示录入入口。
  get isEditor() { return ['teacher', 'admin', 'super', 'superadmin'].includes(this.user?.role) },
  async me() { try { this.user = await api.get('/api/auth/me') } catch { this.user = null } return this.user },
  async logout() { await api.post('/api/auth/logout'); this.user = null },
  login(redirect = '/') { location.href = '/auth/campux?redirect=' + encodeURIComponent(redirect) },
})

export const STATUS = ['等待中', '通过', '解答错误', '编译错误', '时间超限', '内存超限', '运行时错误', '格式错误', '系统错误']
export const STATUS_COLOR = ['#8a94a6', '#22c55e', '#ef4444', '#a855f7', '#f59e0b', '#3b82f6', '#ec4899', '#14b8a6', '#6b7280']
export const DIFF_COLOR = { Easy: '#22c55e', Normal: '#f59e0b', Hard: '#ef4444', Legend: '#a855f7' }

export function fmtDate(s) {
  if (!s) return '-'
  return String(s).replace('T', ' ').replace(/\.\d+Z?$/, '').replace(/Z$/, '')
}


export function toast(msg, ok = true) {
  const el = document.createElement('div')
  el.className = 'toast ' + (ok ? 'toast--ok' : 'toast--err')
  el.textContent = msg
  document.body.appendChild(el)
  setTimeout(() => el.classList.add('toast--out'), 2200)
  setTimeout(() => el.remove(), 2600)
}

