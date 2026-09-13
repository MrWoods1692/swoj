# 前端架构模块

## 技术栈

- Vue 3.4（Composition API + `<script setup>`）
- Vite 5（构建、开发服务器）
- vue-router 4（hash 路由）
- Pinia（状态管理）
- CodeMirror 6（在线编辑器，Ace 主题）
- 无 UI 框架（纯 CSS 变量）

## 目录结构

```
web/
├── index.html
├── package.json
├── vite.config.js
└── src/
    ├── main.js              入口：挂载 Pinia + router
    ├── App.vue              根组件：顶栏 + 主区域
    ├── api.js               fetch 封装（登录态、CSRF、FormData）
    ├── router.js            vue-router 配置
    ├── stores/              Pinia stores
    │   └── auth.js          登录态、用户信息、isAdmin/isTeacher
    ├── i18n.js              中英文
    ├── views/               页面（每个路由一个 .vue）
    │   ├── Home.vue
    │   ├── Problems.vue
    │   ├── ProblemDetail.vue
    │   ├── Submissions.vue
    │   ├── AI.vue
    │   ├── Admin.vue        管理后台聚合入口
    │   ├── AdminLogs.vue
    │   ├── MyLogs.vue
    │   ├── Login.vue
    │   ├── Profile.vue
    │   ├── Terms.vue
    │   ├── Changelog.vue
    │   ├── Materials.vue
    │   ├── Contests.vue
    │   ├── Assignments.vue
    │   ├── Training.vue
    │   ├── Discussions.vue
    │   ├── Leaderboard.vue
    │   ├── Points.vue
    │   ├── Shop.vue
    │   ├── Status.vue
    │   └── ...
    ├── components/          可复用组件
    │   ├── RestReminder.vue
    │   ├── TermsModal.vue
    │   └── CodeEditor.vue
    └── assets/              CSS 变量、图标
```

## API 封装（`api.js`）

```js
const BASE = '/api';
let csrfToken = null;

async function loadCsrf() {
    const r = await fetch('/api/csrf');
    csrfToken = (await r.json()).data.token;
}

export async function get(path, params = {}) { ... }
export async function post(path, body) { ... }
export async function put(path, body) { ... }
export async function del(path) { ... }
export async function upload(path, formData) { ... }  // multipart
```

- 自动带上 `swoj_csrf` Cookie（`credentials: 'include'`）
- 写方法自动附加 `X-CSRF-Token` 头
- 401 响应：清除 `authState`，跳 `/login`
- 403 响应：toast 提示

## 路由

`router.js` 使用 hash 路由（`createWebHashHistory`），便于静态托管：

```js
{ path: '/',         component: () => import('./views/Home.vue') },
{ path: '/problems', component: () => import('./views/Problems.vue') },
{ path: '/p/:id',    component: () => import('./views/ProblemDetail.vue') },
{ path: '/admin',    component: () => import('./views/Admin.vue'), meta: { admin: true } },
{ path: '/admin/logs', component: () => import('./views/AdminLogs.vue'), meta: { admin: true } },
{ path: '/logs',     component: () => import('./views/MyLogs.vue'), meta: { auth: true } },
{ path: '/ai',       component: () => import('./views/AI.vue'), meta: { auth: true } },
...
```

`router.beforeEach` 检查 `authState.isAdmin` / `authState.isTeacher` / `authState.isLoggedIn` 决定是否放行。

## 权限控制

`stores/auth.js`：

```js
const isAdmin = computed(() => authState.value?.role === 'admin' || authState.value?.role === 'super');
const isTeacher = computed(() => isAdmin.value || authState.value?.role === 'teacher');
```

页面内用 `v-if="isAdmin"` 隐藏按钮；导航条也按 `isAdmin` 显示/隐藏「管理后台」入口。

## 主题

`src/style.css` 用 CSS 变量定义两套主题。**默认亮色**（浅灰底 + 白卡片）；`html[data-theme="dark"]` 覆盖为暗色。

顶栏「🌙 / ☀️」按钮切换，写入 `localStorage.swoj_theme`。`index.html` 在 Vue 挂载前同步 `data-theme`，避免首屏闪白。

亮色关键变量：`--bg #f4f7fb`、`--panel #ffffff`、`--text #1b2437`、`--accent #2563eb`。状态徽章走 `--ok-soft` / `--warn-soft` / `--err-soft`，不要在页面里写死暗色前景。

## 构建

```bash
cd web
npm install
npm run dev       # 开发模式（http://localhost:5173）
npm run build     # 生产构建 → dist/
```

Vite 配置代理：`/api` → `http://localhost:18080`（Go 后端）。生产构建后 `dist/` 由 Go 服务静态托管（`SWOJ_STATIC`）。

## 验证

前端不写单测；由 `verify_*.py` 通过 HTTP 请求覆盖端到端行为。构建产物由 Go 服务提供，访问首页可验证。

## 常见问题

**Q: 页面 404？**
A: Go 服务需设置 `SWOJ_STATIC=./web/dist`。

**Q: 中文乱码？**
A: 页面 `<meta charset="UTF-8">`；后端 JSON 响应为 UTF-8。

**Q: 前端请求返回 403？**
A: Cookie 里的 `swoj_csrf` 与 `X-CSRF-Token` 头必须一致；`api.js` 会自动处理。
