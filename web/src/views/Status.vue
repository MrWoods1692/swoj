<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api, auth, fmtDate, toast } from '../api'

// 测评机状态页：机器实时状态、运行环境、配置参数。
// 参数只有管理员可改，普通用户看的是同一样本只读数据。
const info = ref(null)
const err = ref(false)
const busy = ref(false)

// 配置表单：只在 admin 且拿到数据后填充。
const draft = reactive({ workers: 0, pool_size: 0, queue_size: 0, user_timeout_ms: 0, mem_limit_mb: 0, mem_extra_mb: 0 })
const canEdit = computed(() => auth.user && ['admin', 'super', 'superadmin'].includes(auth.user.role))
const editing = ref(false)

async function load() {
  info.value = await api.get('/api/judge/info').catch(() => null)
  err.value = !info.value
}
onMounted(load)

const st = computed(() => info.value)

// 节点状态语义与后端 seed/nodeUpdate 对齐：0 在线、1 维护、2 忙碌、3 离线。
const NODE_STATUS = {
  0: { text: '在线', color: 'var(--ok)' },
  1: { text: '维护', color: 'var(--warn)' },
  2: { text: '忙碌', color: 'var(--accent)' },
  3: { text: '离线', color: 'var(--err)' },
}
function nodeState(s) {
  return NODE_STATUS[s] || { text: '未知', color: 'var(--muted)' }
}
function svcState(s) {
  if (s === 'busy') return { text: '繁忙', color: 'var(--warn)' }
  if (s === 'offline') return { text: '离线', color: 'var(--err)' }
  return { text: '运行中', color: 'var(--ok)' }
}

// 队列水位，超阈值变色提醒。
const queueFull = computed(() => {
  const q = st.value?.queue
  if (!q || !q.capacity) return false
  return q.pending * 10 >= q.capacity * 9
})

// 心跳超过 5 分钟视为可能失联。
const nodeLost = (age) => age > 300

function fmtUptime(s) {
  if (!s && s !== 0) return '-'
  const d = Math.floor(s / 86400), h = Math.floor(s % 86400 / 3600), m = Math.floor(s % 3600 / 60)
  return `${d}天 ${h}小时 ${m}分`
}
function fmtAge(sec) {
  if (!sec) return '-'
  if (sec < 60) return sec + '秒前'
  if (sec < 3600) return Math.floor(sec / 60) + '分钟前'
  return Math.floor(sec / 3600) + '小时前'
}
function pct() {
  const q = st.value?.queue
  if (!q || !q.capacity) return 0
  return Math.round((q.running / q.capacity) * 100)
}

function beginEdit() {
  const j = st.value.judge
  draft.workers = j.workers
  draft.pool_size = j.pool_size
  draft.queue_size = j.queue_size
  draft.user_timeout_ms = j.user_timeout_ms
  draft.mem_limit_mb = j.mem_limit_mb
  draft.mem_extra_mb = j.mem_extra_mb
  editing.value = true
}
function cancelEdit() { editing.value = false }

async function save() {
  if (busy.value) return
  busy.value = true
  try {
    await api.put('/api/admin/judge/config', {
      workers: Number(draft.workers),
      pool_size: Number(draft.pool_size),
      queue_size: Number(draft.queue_size),
      user_timeout_ms: Number(draft.user_timeout_ms),
      mem_limit_mb: Number(draft.mem_limit_mb),
      mem_extra_mb: Number(draft.mem_extra_mb),
    })
    editing.value = false
    toast('测评参数已保存')
    await load()
  } catch (e) {
    toast(e.message || '保存失败', false)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="err" class="panel muted">测评机状态获取失败，服务可能未启动</div>

  <template v-else-if="st">
    <div class="grid grid--4" style="margin-bottom:16px">
      <div class="stat">
        <b>
          <span class="dot" :style="{ background: svcState(st.status).color }"></span>
          {{ svcState(st.status).text }}
        </b>
        <span>测评机状态</span>
      </div>
      <div class="stat"><b>{{ st.version }}</b><span>版本号</span></div>
      <div class="stat"><b>{{ fmtUptime(st.uptime_seconds) }}</b><span>已运行</span></div>
      <div class="stat">
        <b>
          <span v-if="queueFull" class="dot" style="background:var(--warn)"></span>
          {{ st.queue.running }}/{{ st.queue.capacity }}
        </b>
        <span>队列占用</span>
      </div>
    </div>

    <div class="grid grid--2">
      <div class="panel">
        <h3>测评队列</h3>
        <table class="tbl">
          <tbody>
            <tr><th>执行中</th><td>{{ st.queue.running }}</td></tr>
            <tr><th>等待中</th><td>{{ st.queue.pending }}</td></tr>
            <tr><th>队列容量</th><td>{{ st.queue.capacity }}</td></tr>
            <tr><th>累计处理</th><td>{{ st.queue.total }}</td></tr>
            <tr><th>已完成</th><td>{{ st.queue.done }}</td></tr>
            <tr><th>工作线程</th><td>{{ st.queue.workers }}</td></tr>
          </tbody>
        </table>
        <div class="gauge">
          <div class="gauge__bar" :style="{ width: pct() + '%', background: queueFull ? 'var(--warn)' : 'var(--accent)' }"></div>
        </div>
        <div class="small muted">
          当前利用率 {{ pct() }}%{{ queueFull ? '，队列已接近满载' : '' }}
        </div>
      </div>

      <div class="panel">
        <h3>测评节点</h3>
        <div v-if="!st.nodes.length" class="muted small">尚未配置测评节点</div>
        <div v-else class="nlist">
          <div v-for="n in st.nodes" :key="n.id" class="nrow">
            <div class="nrow__head">
              <span class="badge" :style="{ color: nodeState(n.status).color }">
                ● {{ nodeState(n.status).text }}
              </span>
              <span class="mono">{{ n.name }}</span>
              <span class="chip chip--sm">{{ n.judge_type }}</span>
            </div>
            <div class="nrow__meta small muted">
              <span>测评 {{ n.total_count }}</span>
              <span>通过 {{ n.accept_count }}</span>
              <span>运行 {{ n.running }}</span>
              <span :style="{ color: nodeLost(n.age_seconds) ? 'var(--warn)' : '' }">
                心跳 {{ fmtAge(n.age_seconds) }}
              </span>
            </div>
          </div>
        </div>
        <div class="small muted" style="margin-top:10px">
          在线 {{ st.nodes_online }} / 共 {{ st.nodes_total }} 台
          <span v-if="st.nodes_busy">，忙碌 {{ st.nodes_busy }}</span>
        </div>
      </div>
    </div>

    <div class="panel" style="margin-top:16px">
      <div class="phead">
        <h3>测评配置参数</h3>
        <template v-if="canEdit">
          <button v-if="!editing" class="btn btn--ghost btn--sm" @click="beginEdit">调整参数</button>
          <template v-else>
            <button class="btn btn--ghost btn--sm" :disabled="busy" @click="cancelEdit">取消</button>
            <button class="btn btn--primary btn--sm" :disabled="busy" @click="save">
              {{ busy ? '保存中…' : '保存' }}
            </button>
          </template>
        </template>
      </div>

      <div class="grid grid--3">
        <div class="field">
          <label>工作线程</label>
          <input v-if="editing" v-model.number="draft.workers" type="number" min="1" max="64">
          <b v-else>{{ st.judge.workers }}</b>
        </div>
        <div class="field">
          <label>进程池</label>
          <input v-if="editing" v-model.number="draft.pool_size" type="number" min="1" max="256">
          <b v-else>{{ st.judge.pool_size }}</b>
        </div>
        <div class="field">
          <label>队列容量</label>
          <input v-if="editing" v-model.number="draft.queue_size" type="number" min="1" max="100000">
          <b v-else>{{ st.judge.queue_size }}</b>
        </div>
        <div class="field">
          <label>单用例超时（毫秒）</label>
          <input v-if="editing" v-model.number="draft.user_timeout_ms" type="number" min="100" max="600000">
          <b v-else>{{ st.judge.user_timeout_ms }} ms</b>
        </div>
        <div class="field">
          <label>内存限制（MB）</label>
          <input v-if="editing" v-model.number="draft.mem_limit_mb" type="number" min="16" max="65536">
          <b v-else>{{ st.judge.mem_limit_mb }} MB</b>
        </div>
        <div class="field">
          <label>附加开销（MB）</label>
          <input v-if="editing" v-model.number="draft.mem_extra_mb" type="number" min="0" max="4096">
          <b v-else>{{ st.judge.mem_extra_mb }} MB</b>
        </div>
      </div>

      <div class="kv">
        <div><span>执行器</span><span class="mono">{{ st.judge.judge_bin || '内置执行器' }}</span></div>
        <div><span>编译器</span><span class="mono">{{ st.judge.compiler }}</span></div>
        <div><span>编译器路径</span><span class="mono">{{ st.judge.compiler_path || '未找到' }}</span></div>
        <div><span>编译器版本</span><span class="mono">{{ st.judge.compiler_version || '未知' }}</span></div>
        <div><span>支持语言</span><span class="mono">{{ st.judge.languages.join(', ') }}</span></div>
        <div><span>cgroup 挂载</span><span class="mono">{{ st.judge.cgroup_mount }}</span></div>
      </div>
      <div class="small muted">
        执行器与 cgroup 挂载路径需修改环境变量并重启服务生效；数值参数保存后立即生效。
      </div>
    </div>

    <div class="grid grid--2" style="margin-top:16px">
      <div class="panel">
        <h3>运行环境</h3>
        <table class="tbl">
          <tbody>
            <tr><th>Go 版本</th><td class="mono">{{ st.runtime.go_version }}</td></tr>
            <tr><th>平台</th><td class="mono">{{ st.runtime.goos }}/{{ st.runtime.goarch }}</td></tr>
            <tr><th>CPU 核数</th><td>{{ st.runtime.num_cpu }}</td></tr>
            <tr><th>最大并行</th><td>{{ st.runtime.gomaxprocs }}</td></tr>
            <tr><th>协程数</th><td>{{ st.runtime.num_goroutine }}</td></tr>
            <tr><th>已分配内存</th><td>{{ st.runtime.mem_alloc_mb }} MB</td></tr>
            <tr><th>GC 次数</th><td>{{ st.runtime.num_gc }}</td></tr>
            <tr><th>数据目录</th><td class="mono">{{ st.runtime.data_dir }}</td></tr>
          </tbody>
        </table>
      </div>

      <div class="panel">
        <h3>服务信息</h3>
        <table class="tbl">
          <tbody>
            <tr><th>启动时间</th><td class="small">{{ fmtDate(st.started_at) }}</td></tr>
            <tr><th>已运行</th><td>{{ fmtUptime(st.uptime_seconds) }}</td></tr>
            <tr><th>测评机状态</th>
              <td><span class="badge" :style="{ color: svcState(st.status).color }">{{ svcState(st.status).text }}</span></td>
            </tr>
            <tr><th>版本</th><td class="mono">{{ st.version }}</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </template>
</template>

<style scoped>
.dot { display: inline-block; width: 10px; height: 10px; border-radius: 50%; margin-right: 8px; vertical-align: 1px; }
.stat b { display: flex; align-items: center; }
.phead { display: flex; align-items: center; justify-content: space-between; margin: 0 0 14px; }
.phead h3 { margin: 0; }
.field { display: flex; flex-direction: column; gap: 6px; }
.field label { font-size: 12px; color: var(--muted); }
.field b { font-size: 15px; font-weight: 700; }
.field input {
  background: var(--panel2); border: 1px solid var(--border); color: var(--text);
  border-radius: 8px; padding: 7px 10px; font-size: 13px;
}
.gauge { height: 8px; border-radius: 4px; background: var(--panel2); overflow: hidden; margin-top: 12px; }
.gauge__bar { height: 100%; border-radius: 4px; transition: width .3s ease; }
.nlist { display: grid; gap: 10px; }
.nrow { padding: 10px 12px; background: var(--panel2); border: 1px solid var(--border); border-radius: 10px; }
.nrow__head { display: flex; align-items: center; gap: 10px; }
.nrow__head .mono { font-weight: 600; }
.chip--sm { font-size: 11px; padding: 1px 8px; }
.nrow__meta { display: flex; gap: 14px; flex-wrap: wrap; margin-top: 6px; }
.kv { display: grid; gap: 8px; margin: 14px 0 8px; }
.kv > div { display: flex; gap: 12px; font-size: 13px; }
.kv span:first-child { color: var(--muted); min-width: 96px; }
</style>
