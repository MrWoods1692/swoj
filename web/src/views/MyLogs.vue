<script setup>
// 个人日志页：登录用户查看自己的操作记录。
// 服务端强制绑定 claims.UserID，用户无法通过伪造 user_id 查询参数越权——前端也不暴露该字段。
// 与全局日志共享后端 scanLogs；仅过滤条件少一个 user_id。
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { api, auth } from '../api'
import Pager from '../components/Pager.vue'

const err = ref('')
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const list = ref([])
const actions = ref([])

const filter = reactive({
  action: '',
  path: '',
  from: '',
  to: '',
})

const ok = computed(() => !!auth.user)

async function load() {
  if (!auth.user) return
  loading.value = true
  try {
    const q = { page: page.value, page_size: pageSize.value }
    if (filter.action) q.action = filter.action
    if (filter.path) q.path = filter.path
    if (filter.from) q.from = filter.from
    if (filter.to) q.to = filter.to
    const d = await api.get('/api/logs', q)
    list.value = d.list || []
    total.value = d.total || 0
  } catch (e) {
    err.value = e.message
  } finally {
    loading.value = false
  }
}

function reload() { page.value = 1; load() }

async function loadActions() {
  try {
    actions.value = (await api.get('/api/logs/actions')) || []
  } catch { actions.value = [] }
}

function statusText(code) {
  if (code >= 500) return 'error'
  if (code >= 400) return 'warn'
  return 'ok'
}

function actionKind(action) {
  return action.startsWith('api:') ? 'access' : 'biz'
}

function clear() {
  Object.assign(filter, { action: '', path: '', from: '', to: '' })
  reload()
}

onMounted(async () => {
  if (!auth.user) return
  await load()
  await loadActions()
})

watch(() => filter.action, reload)
</script>

<template>
  <div v-if="!ok" class="panel muted">请先登录后查看操作日志。</div>

  <template v-else>
    <div class="panel head">
      <div class="head__row">
        <h2 style="margin:0">我的操作日志</h2>
        <span class="chip small">共 {{ total }} 条</span>
      </div>
      <p class="small muted" style="margin:6px 0 0">
        记录你在本站的所有 API 访问与业务操作（提交、收藏、修改等）。
        <router-link v-if="auth.isAdmin" to="/admin/logs">管理员可查看全局日志 →</router-link>
      </p>
    </div>

    <div class="panel">
      <h3>过滤</h3>
      <div class="filters">
        <label class="field">
          <span>动作</span>
          <input v-model="filter.action" list="my-act-options" placeholder="如 submit / api:POST" @input="reload" />
          <datalist id="my-act-options">
            <option v-for="a in actions" :key="a" :value="a" />
          </datalist>
        </label>
        <label class="field">
          <span>路径</span>
          <input v-model="filter.path" placeholder="如 /api/points" @input="reload" />
        </label>
        <label class="field">
          <span>开始</span>
          <input v-model="filter.from" type="datetime-local" @change="reload" />
        </label>
        <label class="field">
          <span>结束</span>
          <input v-model="filter.to" type="datetime-local" @change="reload" />
        </label>
        <button class="btn" @click="clear">清空</button>
      </div>
    </div>

    <div class="panel" v-if="err" style="color: var(--error)">加载失败：{{ err }}</div>
    <div class="panel" v-else-if="!list.length">
      <p class="muted">暂无匹配的操作记录。</p>
    </div>

    <div class="panel" v-else>
      <table class="tbl">
        <thead>
          <tr>
            <th>时间</th><th>动作</th><th>目标</th><th>路径</th>
            <th>状态</th><th>IP</th><th>详情</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="l in list" :key="l.id">
            <td class="nowrap">{{ l.created_at }}</td>
            <td>
              <span class="chip small" :class="actionKind(l.action) === 'access' ? 'kind-access' : 'kind-biz'">
                {{ actionKind(l.action) === 'access' ? '访问' : '业务' }}
              </span>
              <code class="small">{{ l.action }}</code>
            </td>
            <td class="small">{{ l.target || '-' }}</td>
            <td class="mono small">{{ l.path || '-' }}</td>
            <td>
              <span class="chip small" :class="statusText(l.status_code)" v-if="l.status_code">
                {{ l.status_code }}
              </span>
              <span v-else class="muted small">-</span>
            </td>
            <td class="mono small">{{ l.ip || '-' }}</td>
            <td class="small">{{ l.detail || '-' }}</td>
          </tr>
        </tbody>
      </table>
      <Pager :page="page" :size="pageSize" :total="total" @go="p => { page = p; load() }" />
    </div>
  </template>
</template>

<style scoped>
.head__row { display: flex; align-items: center; gap: 12px; }
.filters {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 10px 14px;
  align-items: end;
}
.field { display: flex; flex-direction: column; gap: 4px; }
.field span { font-size: 12px; color: var(--text-muted); }
.field input {
  padding: 6px 8px; border-radius: 6px;
  border: 1px solid var(--border); background: var(--panel2); color: var(--text);
}
.btn { align-self: end; height: 32px; }
.chip { display: inline-block; padding: 1px 8px; border-radius: 999px; font-size: 11px; }
.kind-access { background: var(--accent-soft); color: var(--accent); }
.kind-biz { background: var(--ok-soft); color: var(--ok); }
.chip.ok { background: var(--ok-soft); color: var(--ok); }
.chip.warn { background: var(--warn-soft); color: var(--warn); }
.chip.error { background: var(--error-soft); color: var(--error); }
.nowrap { white-space: nowrap; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
</style>
