<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { api } from '../api.js'
import { useRouter } from 'vue-router'

const router = useRouter()
const list = ref([])
const total = ref(0)
const loading = ref(false)
const filterTag = ref('')
const search = ref('')
const allTags = computed(() => {
  const s = new Set()
  for (const n of list.value) for (const t of n.tags || []) s.add(t)
  return [...s]
})
const filtered = computed(() => {
  let l = list.value
  if (filterTag.value) l = l.filter(n => (n.tags || []).includes(filterTag.value))
  if (search.value) {
    const q = search.value.toLowerCase()
    l = l.filter(n => (n.title || '').toLowerCase().includes(q) || (n.snippet || '').toLowerCase().includes(q))
  }
  return l
})

const editing = ref(false)
const form = reactive({ id: null, title: '', content: '', tags: '', pinned: false })
const saving = ref(false)

const showConfirm = ref(false)
const deleting = ref(null)
const confirmMsg = ref('')

function resetForm() {
  form.id = null; form.title = ''; form.content = ''; form.tags = ''; form.pinned = false
  editing.value = false
}

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/notes')
    list.value = data.list || []
    total.value = data.total || 0
  } catch (e) { /* 静默 */ }
  finally { loading.value = false }
}

async function open(id) {
  try {
    const n = await api.get('/api/notes/' + id)
    form.id = n.id
    form.title = n.title
    form.content = n.content
    form.tags = (n.tags || []).join(',')
    form.pinned = !!n.pinned
    editing.value = true
    window.scrollTo({ top: 0, behavior: 'smooth' })
  } catch (e) { alert(e.message || '加载失败') }
}

async function save() {
  if (saving.value) return
  saving.value = true
  try {
    const body = { title: form.title, content: form.content, tags: form.tags, pinned: form.pinned }
    if (form.id) await api.put('/api/notes/' + form.id, body)
    else await api.post('/api/notes', body)
    resetForm()
    await load()
  } catch (e) { alert(e.message || '保存失败') }
  finally { saving.value = false }
}

async function togglePin(n) {
  try {
    const full = await api.get('/api/notes/' + n.id)
    const body = {
      title: full.title,
      content: full.content,
      tags: (full.tags || []).join(','),
      pinned: !n.pinned,
    }
    await api.put('/api/notes/' + n.id, body)
    await load()
  } catch (e) { alert(e.message || '操作失败') }
}

async function askDelete(n) {
  deleting.value = n
  confirmMsg.value = `删除笔记「${n.title}」？此操作不可撤销。`
  showConfirm.value = true
}

async function doDelete() {
  const n = deleting.value
  showConfirm.value = false
  if (!n) return
  try {
    await api.del('/api/notes/' + n.id)
    if (form.id === n.id) resetForm()
    await load()
  } catch (e) { alert(e.message || '删除失败') }
  finally { deleting.value = null }
}

function fmtTime(s) {
  if (!s) return ''
  return s.replace('T', ' ').slice(0, 16)
}

onMounted(load)
</script>

<template>
  <div class="notes-page">
    <header class="page-head">
      <div>
        <h1>📓 个人笔记</h1>
        <p class="sub">仅自己可见的私密备忘；建议记录题目思路、复盘、错题笔记。</p>
      </div>
      <button class="btn btn-primary" @click="resetForm(); editing=true">＋ 新建笔记</button>
    </header>

    <!-- 编辑器 -->
    <section v-if="editing" class="editor card">
      <div class="editor-head">
        <span>{{ form.id ? '编辑笔记' : '新建笔记' }}</span>
        <label class="pin-toggle">
          <input type="checkbox" v-model="form.pinned" />
          <span>置顶</span>
        </label>
      </div>
      <input v-model="form.title" class="title-input" placeholder="标题（1–80 字，必填）" maxlength="80" @keyup.enter.exact.prevent="save" />
      <textarea v-model="form.content" class="content-input" placeholder="正文（支持 Markdown 纯文本；上限 10 万字）" maxlength="100000" rows="12"></textarea>
      <input v-model="form.tags" class="tags-input" placeholder="标签，用逗号/空格分隔，最多 8 个，每个 ≤ 12 字" />
      <div class="editor-actions">
        <button class="btn btn-primary" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存' }}</button>
        <button class="btn" @click="resetForm()">取消</button>
      </div>
    </section>

    <!-- 搜索与筛选 -->
    <section class="toolbar">
      <input v-model="search" class="search-input" placeholder="按标题或正文搜索…" />
      <select v-model="filterTag" class="filter-select">
        <option value="">全部标签</option>
        <option v-for="t in allTags" :key="t" :value="t">#{{ t }}</option>
      </select>
      <span class="count">共 {{ total }} 条 · 显示 {{ filtered.length }} 条</span>
      <button class="btn" @click="load">刷新</button>
    </section>

    <!-- 列表 -->
    <section class="list">
      <div v-if="loading" class="empty">加载中…</div>
      <div v-else-if="!filtered.length" class="empty">
        <template v-if="list.length">没有匹配的笔记</template>
        <template v-else>还没有笔记，点右上角「＋ 新建笔记」开始记录。</template>
      </div>
      <article v-for="n in filtered" :key="n.id" class="note-card card">
        <div class="note-head">
          <h3 :class="{ pinned: n.pinned }">
            <span v-if="n.pinned" class="pin">📌</span>
            <a href="#" @click.prevent="open(n.id)">{{ n.title }}</a>
          </h3>
          <time class="updated">{{ fmtTime(n.updated_at) }}</time>
        </div>
        <p class="snippet">{{ n.snippet || '（无正文）' }}</p>
        <div class="note-foot">
          <div class="tags">
            <span v-for="t in n.tags" :key="t" class="tag" @click="filterTag = t">#{{ t }}</span>
          </div>
          <div class="ops">
            <button class="mini" @click="togglePin(n)">{{ n.pinned ? '取消置顶' : '置顶' }}</button>
            <button class="mini" @click="open(n.id)">编辑</button>
            <button class="mini danger" @click="askDelete(n)">删除</button>
          </div>
        </div>
      </article>
    </section>

    <!-- 删除确认 -->
    <div v-if="showConfirm" class="modal-mask" @click.self="showConfirm=false">
      <div class="modal card">
        <p class="modal-msg">{{ confirmMsg }}</p>
        <div class="modal-actions">
          <button class="btn" @click="showConfirm=false">取消</button>
          <button class="btn btn-danger" @click="doDelete">确认删除</button>
        </div>
      </div>
    </div>

    <p class="hint">
      提示：笔记仅本人在 <router-link to="/notes">/notes</router-link> 可见；
      与「错题本」不同，笔记完全由用户手动维护，不参与积分统计。
    </p>
  </div>
</template>

<style scoped>
.notes-page { padding: 16px; max-width: 900px; margin: 0 auto; }
.page-head { display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:16px; gap:16px; }
.page-head h1 { margin: 0 0 4px; font-size: 22px; }
.page-head .sub { color: var(--text-secondary); margin: 0; font-size: 13px; }
.btn { padding: 6px 14px; border-radius: 6px; border: 1px solid var(--border); background: var(--bg-secondary); cursor: pointer; font-size: 14px; }
.btn-primary { background: var(--accent); color: #fff; border-color: var(--accent); }
.btn-danger { background: var(--error); color: #fff; border-color: var(--error); }
.btn:disabled { opacity: .6; cursor: not-allowed; }
.card { border: 1px solid var(--border); border-radius: 8px; padding: 14px; background: var(--bg-primary); margin-bottom: 12px; }
.editor-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:8px; font-weight:600; }
.pin-toggle { display:inline-flex; align-items:center; gap:6px; font-weight: 400; font-size: 13px; cursor: pointer; }
.title-input, .content-input, .tags-input, .search-input, .filter-select {
  width: 100%; box-sizing: border-box; padding: 8px 10px; margin-bottom: 8px;
  border: 1px solid var(--border); border-radius: 6px; background: var(--bg-primary); color: var(--text-primary);
  font-size: 14px;
}
.content-input { font-family: ui-monospace, Menlo, Consolas, monospace; line-height: 1.6; resize: vertical; }
.editor-actions { display:flex; gap:8px; }
.toolbar { display:flex; gap:8px; align-items:center; margin: 16px 0; flex-wrap: wrap; }
.toolbar .search-input { margin-bottom: 0; flex: 1; min-width: 200px; }
.toolbar .filter-select { margin-bottom: 0; }
.toolbar .count { color: var(--text-secondary); font-size: 13px; margin-left: auto; }
.toolbar .btn { margin-bottom: 0; }
.note-card { display: flex; flex-direction: column; gap: 8px; }
.note-head { display:flex; justify-content:space-between; align-items:baseline; }
.note-head h3 { margin: 0; font-size: 16px; }
.note-head h3.pinned { color: var(--accent); }
.note-head .pin { margin-right: 4px; }
.note-head time { color: var(--text-secondary); font-size: 12px; }
.snippet { margin: 0; color: var(--text-primary); font-size: 14px; line-height: 1.6; word-break: break-word; }
.note-foot { display:flex; justify-content:space-between; align-items:center; gap:8px; flex-wrap: wrap; }
.tags { display:flex; gap:4px; flex-wrap: wrap; }
.tag { display:inline-block; padding: 2px 8px; border-radius: 999px; background: var(--bg-secondary); color: var(--text-secondary); font-size: 12px; cursor: pointer; }
.tag:hover { background: var(--accent); color: #fff; }
.ops { display:flex; gap:6px; }
.mini { padding: 3px 8px; border-radius: 4px; border: 1px solid var(--border); background: transparent; cursor: pointer; font-size: 12px; color: var(--text-primary); }
.mini:hover { background: var(--bg-secondary); }
.mini.danger { color: var(--error); border-color: var(--error); }
.empty { text-align: center; padding: 40px; color: var(--text-secondary); }
.modal-mask { position: fixed; inset: 0; background: rgba(0,0,0,.4); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { max-width: 380px; width: 90%; }
.modal-msg { margin: 0 0 12px; }
.modal-actions { display:flex; justify-content: flex-end; gap: 8px; }
.hint { color: var(--text-secondary); font-size: 12px; margin-top: 20px; }
</style>
