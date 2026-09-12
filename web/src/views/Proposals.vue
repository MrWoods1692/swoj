<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { api, auth } from '../api.js'

const isAdmin = computed(() => auth.isAdmin || auth.isEditor)

// 我提交的
const mine = ref([])
const mineTotal = ref(0)
const activeFilter = ref('all')
const filteredMine = computed(() => {
  if (activeFilter.value === 'all') return mine.value
  return mine.value.filter(n => n.status === activeFilter.value)
})
const statusLabel = (s) => ({ pending: '待审核', approved: '已通过', rejected: '已拒绝', withdrawn: '已撤回' })[s] || s

// 编辑
const editing = ref(false)
const editor = reactive({ id: null, name: '', background: '', description: '', input_format: '', output_format: '', hint: '' })
const cases = ref([{ input: '', output: '' }])
const saving = ref(false)

// 查看详情
const detail = ref(null)
const detailOpen = ref(false)

// 审核（老师/管理员）
const pendingList = ref([])
const reviewing = ref(null)
const reviewForm = reactive({ accepted: true, points: 10, comment: '' })

function resetEditor() {
  editor.id = null; editor.name = ''; editor.background = '';
  editor.description = ''; editor.input_format = ''; editor.output_format = ''; editor.hint = ''
  cases.value = [{ input: '', output: '' }]
  editing.value = false
}

function fmtTime(s) {
  if (!s) return ''
  return s.replace('T', ' ').slice(0, 16)
}

function statusTag(s) {
  return ({ pending: 'pending', approved: 'approved', rejected: 'rejected', withdrawn: 'withdrawn' })[s] || 'pending'
}

async function loadMine() {
  try {
    const d = await api.get('/api/proposals')
    mine.value = d.list || []
    mineTotal.value = d.total || 0
  } catch (e) { /* 静默 */ }
}

async function loadPending() {
  if (!isAdmin.value) return
  try {
    const d = await api.get('/api/admin/proposals')
    pendingList.value = d.list || []
  } catch (e) { /* 静默 */ }
}

async function openView(id) {
  try {
    detail.value = await api.get('/api/proposals/' + id)
    detailOpen.value = true
    window.scrollTo({ top: 0, behavior: 'smooth' })
  } catch (e) { alert(e.message || '加载失败') }
}

async function save() {
  if (saving.value) return
  saving.value = true
  try {
    const body = {
      name: editor.name, background: editor.background, description: editor.description,
      input_format: editor.input_format, output_format: editor.output_format,
      hint: editor.hint,
      cases: cases.value.filter(c => c.input.trim() || c.output.trim()),
    }
    if (editor.id) {
      alert('编辑已有提议暂不支持，请先撤回后删除并重新提交。')
      return
    }
    await api.post('/api/proposals', body)
    resetEditor()
    await loadMine()
  } catch (e) { alert(e.message || '保存失败') }
  finally { saving.value = false }
}

async function askWithdraw(n) {
  if (!confirm(`撤回提议「${n.name}」？`)) return
  try {
    await api.put('/api/proposals/' + n.id + '/withdraw')
    await loadMine()
  } catch (e) { alert(e.message || '撤回失败') }
}

async function askDelete(n) {
  if (!confirm(`删除提议「${n.name}」？此操作不可撤销。`)) return
  try {
    await api.del('/api/proposals/' + n.id)
    await loadMine()
  } catch (e) { alert(e.message || '删除失败') }
}

// 审核弹窗
function openReview(n) {
  reviewing.value = n
  reviewForm.accepted = true
  reviewForm.points = 10
  reviewForm.comment = ''
}

async function doReview() {
  const n = reviewing.value
  if (!n) return
  try {
    const body = { accepted: reviewForm.accepted, reward_points: reviewForm.points, comment: reviewForm.comment }
    const d = await api.post('/api/admin/proposals/' + n.id + '/review', body)
    alert(`审核完成：${d.status === 'approved' ? '已通过，已发放 ' + d.reward_points + ' 积分' : '已拒绝'}`)
    reviewing.value = null
    await Promise.all([loadPending(), loadMine()])
  } catch (e) { alert(e.message || '审核失败') }
}

onMounted(() => {
  loadMine()
  if (isAdmin.value) loadPending()
})
</script>

<template>
  <div class="proposals-page">
    <header class="page-head">
      <div>
        <h1>📝 学生出题</h1>
        <p class="sub">提交题目草稿，由老师或管理员审核通过后进入题库，并获得积分奖励。</p>
      </div>
      <button class="btn btn-primary" @click="resetEditor(); editing=true">＋ 新建提议</button>
    </header>

    <!-- 编辑器 -->
    <section v-if="editing" class="editor card">
      <div class="editor-head">
        <span>新建提议</span>
        <span class="hint-inline">通过后默认奖励 <strong>10</strong> 积分</span>
      </div>
      <input v-model="editor.name" class="title-input" placeholder="题目名称（1–80 字，必填）" maxlength="80" />
      <textarea v-model="editor.background" class="field" placeholder="题目背景（可选，≤ 1 万字）" maxlength="10000" rows="4"></textarea>
      <textarea v-model="editor.description" class="field mono" placeholder="题目描述（必填，≤ 5 万字）" maxlength="50000" rows="8"></textarea>
      <textarea v-model="editor.input_format" class="field mono" placeholder="输入格式（可选，≤ 5 千字）" maxlength="5000" rows="4"></textarea>
      <textarea v-model="editor.output_format" class="field mono" placeholder="输出格式（可选，≤ 5 千字）" maxlength="5000" rows="4"></textarea>
      <textarea v-model="editor.hint" class="field" placeholder="提示说明（可选，≤ 5 千字）" maxlength="5000" rows="3"></textarea>

      <div class="cases-section">
        <div class="cases-head">
          <span>样例（{{ cases.length }} 组，至少 1 组）</span>
          <button class="btn mini" @click="cases.push({input:'',output:''})">＋ 添加样例</button>
        </div>
        <div v-for="(c, i) in cases" :key="i" class="case-row">
          <span class="case-idx">#{{ i + 1 }}</span>
          <textarea v-model="c.input" placeholder="输入" rows="3"></textarea>
          <textarea v-model="c.output" placeholder="输出" rows="3"></textarea>
          <button class="btn mini danger" @click="cases.splice(i, 1)" :disabled="cases.length <= 1">删除</button>
        </div>
      </div>

      <div class="editor-actions">
        <button class="btn btn-primary" :disabled="saving" @click="save">{{ saving ? '提交中…' : '提交审核' }}</button>
        <button class="btn" @click="resetEditor()">取消</button>
      </div>
    </section>

    <!-- 审核列表（老师/管理员） -->
    <section v-if="isAdmin && pendingList.length" class="pending card">
      <h2>📥 待审核提议</h2>
      <table class="table">
        <thead><tr><th>ID</th><th>题目名称</th><th>作者</th><th>提交时间</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="n in pendingList" :key="n.id">
            <td>{{ n.id }}</td>
            <td><a href="#" @click.prevent="openView(n.id)">{{ n.name }}</a></td>
            <td>{{ n.author_name }}</td>
            <td>{{ fmtTime(n.created_at) }}</td>
            <td>
              <button class="btn mini" @click="openReview(n)">审核</button>
            </td>
          </tr>
        </tbody>
      </table>
    </section>

    <!-- 我的提议 -->
    <section class="list">
      <div class="toolbar">
        <div class="filter-group">
          <button v-for="s in [['all','全部'],['pending','待审核'],['approved','已通过'],['rejected','已拒绝'],['withdrawn','已撤回']]" :key="s[0]"
                  class="filter-chip" :class="{ active: activeFilter === s[0] }"
                  @click="activeFilter = s[0]">{{ s[1] }}</button>
        </div>
        <span class="count">共 {{ mineTotal }} 条 · 显示 {{ filteredMine.length }} 条</span>
        <button class="btn" @click="loadMine()">刷新</button>
      </div>
      <div v-if="!filteredMine.length" class="empty">还没有提议，点右上角「＋ 新建提议」开始提交题目。</div>
      <article v-for="n in filteredMine" :key="n.id" class="proposal-card card">
        <div class="proposal-head">
          <h3>
            <span v-if="n.status === 'approved' && n.approved_problem_id" class="tag-link">
              已入库 · <a href="/#/problems/{{ n.approved_problem_id }}">#{{ n.approved_problem_id }}</a>
            </span>
            {{ n.name }}
          </h3>
          <span class="status" :class="statusTag(n.status)">{{ statusLabel(n.status) }}</span>
        </div>
        <div class="meta">
          <span>提交：{{ fmtTime(n.created_at) }}</span>
          <span v-if="n.reviewed_at">审核：{{ fmtTime(n.reviewed_at) }}</span>
          <span v-if="n.reward_points && n.status === 'approved'">奖励 {{ n.reward_points }} 积分</span>
          <span v-if="n.review_comment" class="comment">「{{ n.review_comment }}」</span>
        </div>
        <div class="ops">
          <button class="mini" @click="openView(n.id)">查看</button>
          <button v-if="n.status === 'pending' || n.status === 'rejected'" class="mini" @click="askWithdraw(n)">撤回</button>
          <button v-if="n.status === 'withdrawn' || n.status === 'rejected'" class="mini danger" @click="askDelete(n)">删除</button>
        </div>
      </article>
    </section>

    <!-- 详情弹窗 -->
    <div v-if="detailOpen" class="modal-mask" @click.self="detailOpen=false">
      <div class="modal card">
        <div class="modal-head">
          <h2>{{ detail.name }}</h2>
          <button class="btn mini" @click="detailOpen=false">关闭</button>
        </div>
        <div v-if="detail.background" class="block">
          <h4>题目背景</h4>
          <pre>{{ detail.background }}</pre>
        </div>
        <div class="block">
          <h4>题目描述</h4>
          <pre>{{ detail.description }}</pre>
        </div>
        <div v-if="detail.input_format" class="block">
          <h4>输入格式</h4>
          <pre>{{ detail.input_format }}</pre>
        </div>
        <div v-if="detail.output_format" class="block">
          <h4>输出格式</h4>
          <pre>{{ detail.output_format }}</pre>
        </div>
        <div v-if="detail.hint" class="block">
          <h4>提示</h4>
          <pre>{{ detail.hint }}</pre>
        </div>
        <div class="block">
          <h4>样例（{{ (detail.cases || []).length }} 组）</h4>
          <div v-for="(c, i) in (detail.cases || [])" :key="i" class="case-block">
            <div class="case-idx">#{{ i + 1 }}</div>
            <div><b>输入：</b><pre>{{ c.input }}</pre></div>
            <div><b>输出：</b><pre>{{ c.output }}</pre></div>
          </div>
        </div>
      </div>
    </div>

    <!-- 审核弹窗 -->
    <div v-if="reviewing" class="modal-mask" @click.self="reviewing=null">
      <div class="modal card">
        <div class="modal-head">
          <h2>审核提议 #{{ reviewing.id }}</h2>
          <button class="btn mini" @click="reviewing=null">关闭</button>
        </div>
        <div class="block">
          <h4>题目：{{ reviewing.name }}</h4>
          <div class="meta">作者：{{ reviewing.author_name }} · 提交于 {{ fmtTime(reviewing.created_at) }}</div>
        </div>
        <div class="block">
          <h4>审核动作</h4>
          <label class="radio"><input type="radio" v-model="reviewForm.accepted" :value="true" /> 通过（进入题库并颁发积分）</label>
          <label class="radio"><input type="radio" v-model="reviewForm.accepted" :value="false" /> 拒绝（可附审核意见）</label>
        </div>
        <div v-if="reviewForm.accepted" class="block">
          <label class="label">奖励积分</label>
          <input v-model.number="reviewForm.points" type="number" min="1" max="100" class="small" />
          <span class="hint-inline">默认 10；范围 1–100</span>
        </div>
        <div class="block">
          <label class="label">审核意见</label>
          <textarea v-model="reviewForm.comment" rows="3" placeholder="可选"></textarea>
        </div>
        <div class="modal-actions">
          <button class="btn" @click="reviewing=null">取消</button>
          <button class="btn" :class="reviewForm.accepted ? 'btn-primary' : 'btn-danger'" @click="doReview">
            {{ reviewForm.accepted ? '确认通过' : '确认拒绝' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.proposals-page { padding: 16px; max-width: 960px; margin: 0 auto; }
.page-head { display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:16px; gap:16px; }
.page-head h1 { margin: 0 0 4px; font-size: 22px; }
.page-head .sub { color: var(--text-secondary); margin: 0; font-size: 13px; }
.btn { padding: 6px 14px; border-radius: 6px; border: 1px solid var(--border); background: var(--bg-secondary); cursor: pointer; font-size: 14px; }
.btn-primary { background: var(--accent); color: #fff; border-color: var(--accent); }
.btn-danger { background: var(--error); color: #fff; border-color: var(--error); }
.btn:disabled { opacity: .6; cursor: not-allowed; }
.btn.mini { padding: 3px 8px; font-size: 12px; }
.mini { padding: 3px 8px; border-radius: 4px; border: 1px solid var(--border); background: transparent; cursor: pointer; font-size: 12px; color: var(--text-primary); }
.mini:hover { background: var(--bg-secondary); }
.mini.danger { color: var(--error); border-color: var(--error); }
.card { border: 1px solid var(--border); border-radius: 8px; padding: 14px; background: var(--bg-primary); margin-bottom: 12px; }
.editor-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:8px; font-weight:600; }
.hint-inline { color: var(--text-secondary); font-weight: 400; font-size: 12px; }
.title-input, .field, .small {
  width: 100%; box-sizing: border-box; padding: 8px 10px; margin-bottom: 8px;
  border: 1px solid var(--border); border-radius: 6px; background: var(--bg-primary); color: var(--text-primary);
  font-size: 14px;
}
.mono { font-family: ui-monospace, Menlo, Consolas, monospace; }
.field { resize: vertical; line-height: 1.6; }
.small { max-width: 100px; display: inline-block; }
.editor-actions { display:flex; gap:8px; }
.cases-section { margin-top: 12px; }
.cases-head { display:flex; justify-content:space-between; align-items:center; margin-bottom: 6px; font-weight: 600; font-size: 14px; }
.case-row { display:grid; grid-template-columns: 30px 1fr 1fr 60px; gap: 6px; margin-bottom: 6px; align-items: flex-start; }
.case-row textarea { min-height: 60px; resize: vertical; border: 1px solid var(--border); border-radius: 4px; padding: 6px; font-family: ui-monospace, Menlo, Consolas, monospace; font-size: 13px; }
.case-idx { display: inline-block; padding: 2px 6px; border-radius: 4px; background: var(--bg-secondary); font-size: 12px; font-weight: 600; }
.toolbar { display:flex; gap:8px; align-items:center; margin: 16px 0; flex-wrap: wrap; }
.filter-group { display:flex; gap: 4px; }
.filter-chip { padding: 4px 10px; border-radius: 999px; border: 1px solid var(--border); background: transparent; cursor: pointer; font-size: 13px; color: var(--text-primary); }
.filter-chip.active { background: var(--accent); color: #fff; border-color: var(--accent); }
.count { color: var(--text-secondary); font-size: 13px; margin-left: auto; }
.empty { text-align: center; padding: 40px; color: var(--text-secondary); }
.proposal-card { display: flex; flex-direction: column; gap: 8px; }
.proposal-head { display:flex; justify-content:space-between; align-items:baseline; gap: 8px; }
.proposal-head h3 { margin: 0; font-size: 16px; }
.tag-link { display: inline-block; padding: 1px 6px; margin-right: 6px; border-radius: 4px; background: var(--bg-secondary); font-size: 11px; font-weight: 600; }
.status { padding: 2px 8px; border-radius: 999px; font-size: 12px; font-weight: 600; }
.status.pending { background: #fef3c7; color: #92400e; }
.status.approved { background: #d1fae5; color: #065f46; }
.status.rejected { background: #fee2e2; color: #991b1b; }
.status.withdrawn { background: var(--bg-secondary); color: var(--text-secondary); }
.meta { display: flex; gap: 12px; flex-wrap: wrap; color: var(--text-secondary); font-size: 12px; }
.comment { font-style: italic; }
.ops { display:flex; gap: 6px; }
.table { width: 100%; border-collapse: collapse; margin-top: 8px; }
.table th, .table td { padding: 6px 8px; border-bottom: 1px solid var(--border); text-align: left; font-size: 13px; }
.modal-mask { position: fixed; inset: 0; background: rgba(0,0,0,.4); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { max-width: 640px; width: 92%; max-height: 86vh; overflow-y: auto; }
.modal-head { display:flex; justify-content:space-between; align-items:center; margin-bottom: 10px; }
.modal-head h2 { margin: 0; font-size: 18px; }
.modal-actions { display:flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
.block { margin-bottom: 10px; }
.block h4 { margin: 0 0 4px; font-size: 13px; color: var(--text-secondary); }
.block pre { margin: 0; padding: 8px; border-radius: 6px; background: var(--bg-secondary); font-family: ui-monospace, Menlo, Consolas, monospace; font-size: 13px; white-space: pre-wrap; word-break: break-word; }
.case-block { border: 1px solid var(--border); border-radius: 6px; padding: 6px 10px; margin-bottom: 6px; }
.label { display: block; margin-bottom: 4px; font-size: 13px; color: var(--text-secondary); }
.radio { display: flex; align-items: center; gap: 6px; margin-bottom: 6px; cursor: pointer; }
.radio input { margin: 0; }
textarea.small { min-height: 60px; }
</style>
