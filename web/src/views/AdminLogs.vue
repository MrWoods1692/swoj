<script setup>
// 全局日志页：管理员查看全站操作日志（含中间件访问日志与业务侧 logOp 语义动作）。
// 支持按用户/动作/路径/时间范围过滤；顶部汇总指标；下方近 N 天每日趋势。
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { api, auth } from '../api'
import Pager from '../components/Pager.vue'

const err = ref('')
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const list = ref([])
const summary = ref(null)
const daily = ref([])
const userOptions = ref([])
const actionOptions = ref([])
const actions = ref([])

const filter = reactive({
  user_id: '',
  action: '',
  path: '',
  from: '',
  to: '',
})

const ok = computed(() => auth.isAdmin)

async function load() {
  if (!auth.isAdmin) return
  loading.value = true
  try {
    const q = { page: page.value, page_size: pageSize.value }
    if (filter.user_id) q.user_id = filter.user_id
    if (filter.action) q.action = filter.action
    if (filter.path) q.path = filter.path
    if (filter.from) q.from = filter.from
    if (filter.to) q.to = filter.to
    const d = await api.get('/api/admin/logs', q)
    list.value = d.list || []
    total.value = d.total || 0
    summary.value = d.summary || null
  } catch (e) {
    err.value = e.message
  } finally {
    loading.value = false
  }
}

async function loadDaily() {
  try {
    daily.value = (await api.get('/api/logs/daily', { days: 30 })) || []
  } catch { daily.value = [] }
}

async function loadActions() {
  try {
    actions.value = (await api.get('/api/logs/actions')) || []
  } catch { actions.value = [] }
}

const maxDaily = computed(() => Math.max(1, ...daily.value.map(d => d.count)))

onMounted(async () => {
  if (!auth.isAdmin) return
  await load()
  await loadDaily()
  await loadActions()
})

// 重置到第一页并重载。
function reload() {
  page.value = 1
  load()
}

function onDateChange(which) {
  const v = event.target.value
  if (which === 'from') filter.from = v
  if (which === 'to') filter.to = v
  reload()
}

watch(() => filter.action, reload)

function statusText(code) {
  if (code >= 500) return 'error'
  if (code >= 400) return 'warn'
  return 'ok'
}

function actionKind(action) {
  // api:* 中间件访问日志 vs 业务语义动作
  return action.startsWith('api:') ? 'access' : 'biz'
}

function clear() {
  Object.assign(filter, { user_id: '', action: '', path: '', from: '', to: '' })
  reload()
}
</script>

<template>
  <div v-if="!ok" class="panel muted">只有管理员可以查看全局日志。</div>

  <template v-else>
    <div class="panel head">
      <div class="head__row">
        <h2 style="margin:0">全局日志</h2>
        <span class="chip small">共 {{ total }} 条</span>
      </div>
      <p class="small muted" style="margin:6px 0 0">
        中间件层记录所有 API 请求，业务侧写入语义动作（提交、创建、修改等）。两类共用同一张表。
      </p>
    </div>

    <div v-if="summary" class="grid grid--4">
      <div class="stat"><b>{{ summary.total }}</b><span>累计条目</span></div>
      <div class="stat"><b>{{ summary.today }}</b><span>近 24 小时</span></div>
      <div class="stat"><b>{{ summary.errors_30d }}</b><span>近 30 天错误响应</span></div>
      <div class="stat"><b>{{ summary.distinct_users }}</b><span>活跃用户</span></div>
    </div>

    <div class="panel">
      <h3>过滤</h3>
      <div class="filters">
        <label class="field">
          <span>用户 ID</span>
          <input v-model="filter.user_id" type="number" min="1" placeholder="可选" @change="reload" />
        </label>
        <label class="field">
          <span>动作</span>
          <input v-model="filter.action" list="act-options" placeholder="如 submit / api:POST" @input="reload" />
          <datalist id="act-options">
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

    <div class="panel">
      <h3>近 30 天每日日志量</h3>
      <div class="bars">
        <div v-for="d in daily" :key="d.date" class="bar" :title="`${d.date} · ${d.count} 条`">
          <div :style="{ height: (d.count / maxDaily) * 100 + '%' }"></div>
          <span>{{ d.date.slice(5) }}</span>
        </div>
        <div v-if="!daily.length" class="muted small">暂无数据</div>
      </div>
    </div>

    <div class="panel" v-if="err" style="color: var(--error)">加载失败：{{ err }}</div>

    <div class="panel" v-else-if="!list.length">
      <p class="muted">没有匹配的记录。</p>
    </div>

    <div class="panel" v-else>
      <table class="tbl">
        <thead>
          <tr>
            <th>时间</th><th>用户</th><th>动作</th><th>目标</th><th>路径</th>
            <th>状态</th><th>IP</th><th>详情</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="l in list" :key="l.id">
            <td class="nowrap">{{ l.created_at }}</td>
            <td>
              <span v-if="l.user_id">
                <span class="mono">{{ l.user_id }}</span>
                <span class="small muted">{{ l.username }}</span>
              </span>
              <span v-else class="muted small">未登录</span>
            </td>
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
.bars { display: flex; gap: 4px; align-items: end; height: 120px; overflow-x: auto; }
.bar { flex: 0 0 26px; display: flex; flex-direction: column; align-items: center; gap: 4px; height: 100%; }
.bar > div { width: 100%; background: var(--accent); border-radius: 3px 3px 0 0; min-height: 2px; }
.bar span { font-size: 10px; color: var(--text-muted); }
</style>
