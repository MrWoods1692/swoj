<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api, auth, toast } from '../api'
import { i18n } from '../i18n'

const zh = computed(() => i18n.lang === 'zh')

const list = ref([])
const kinds = ref([])
const kindsCount = ref({})
const total = ref(0)
const page = ref(1)
const size = 20
const active = ref('')
const loading = ref(false)
const err = ref('')

const isAdmin = computed(() => auth.isAdmin)

const edit = ref(false)
const form = reactive({ id: 0, version: '', kind: 'feature', content: '', notes: '', released_at: '' })

async function load() {
  loading.value = true
  err.value = ''
  try {
    const params = new URLSearchParams({ page: String(page.value), size: String(size) })
    if (active.value) params.set('kind', active.value)
    const data = await api.get('/api/changelog?' + params)
    list.value = data.list || []
    total.value = data.total || 0
    kindsCount.value = data.kinds || {}
    const k = await api.get('/api/changelog/kinds').catch(() => ({ kinds: [] }))
    kinds.value = (k && k.kinds) || []
  } catch (e) {
    err.value = e?.msg || e?.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function filter(kind) {
  active.value = kind
  page.value = 1
  load()
}

function blankForm() {
  form.id = 0
  form.version = ''
  form.kind = 'feature'
  form.content = ''
  form.notes = ''
  form.released_at = new Date().toISOString().slice(0, 10)
}

function editRow(row) {
  blankForm()
  Object.assign(form, { id: row.id, version: row.version, kind: row.kind, content: row.content, notes: row.notes || '', released_at: row.released_at })
  edit.value = true
}

async function save() {
  const payload = {
    version: form.version,
    kind: form.kind,
    content: form.content,
    notes: form.notes,
    released_at: form.released_at,
  }
  try {
    if (form.id) await api.put('/api/admin/changelog/' + form.id, payload)
    else await api.post('/api/admin/changelog', payload)
    toast(form.id ? '已更新' : '已新增')
    edit.value = false
    blankForm()
    load()
  } catch (e) {
    toast(e?.msg || e?.message || '保存失败')
  }
}

async function del(row) {
  if (!confirm(`确认删除「${row.version} · ${row.content}」？`)) return
  try {
    await api.del('/api/admin/changelog/' + row.id)
    toast('已删除')
    load()
  } catch (e) {
    toast(e?.msg || e?.message || '删除失败')
  }
}

onMounted(load)
</script>

<template>
  <div class="panel">
    <div class="head-row">
      <div>
        <h2>更新日志</h2>
        <p class="muted small" style="margin-top:4px">
          {{ zh ? '按发布时间倒序的改动记录，共 ' + total + ' 条。' : 'Release notes, newest first. ' + total + ' entries.' }}
        </p>
      </div>
      <button v-if="isAdmin" class="btn btn--primary" @click="edit = !edit">
        {{ edit ? (zh ? '收起' : 'Close') : (zh ? '＋ 新增记录' : 'Add') }}
      </button>
    </div>

    <div class="chips" style="margin-top:12px">
      <span class="chip small" :class="{ active: !active }" @click="filter('')">
        {{ zh ? '全部' : 'All' }} <b>{{ total }}</b>
      </span>
      <span v-for="k in kinds" :key="k.kind" class="chip small" :class="{ active: active === k.kind }" @click="filter(k.kind)">
        {{ k.label }} <b>{{ kindsCount[k.kind] || 0 }}</b>
      </span>
    </div>

    <form v-if="edit" class="form" style="margin-top:12px">
      <div class="grid grid--2">
        <label class="field">
          <span>{{ zh ? '版本号' : 'Version' }} *</span>
          <input v-model="form.version" maxlength="32" placeholder="1.0.0" />
        </label>
        <label class="field">
          <span>{{ zh ? '改动分类' : 'Kind' }}</span>
          <select v-model="form.kind">
            <option v-for="k in kinds" :key="k.kind" :value="k.kind">{{ k.label }}</option>
          </select>
        </label>
      </div>
      <label class="field">
        <span>{{ zh ? '标题（≤120 字）' : 'Title (≤120)' }} *</span>
        <input v-model="form.content" maxlength="120" placeholder="新增资料模块：教师上传教学资料" />
      </label>
      <label class="field">
        <span>{{ zh ? '发布日期' : 'Released' }}</span>
        <input v-model="form.released_at" type="date" />
      </label>
      <label class="field">
        <span>{{ zh ? '详细说明（可选）' : 'Notes (optional)' }}</span>
        <textarea v-model="form.notes" rows="4" maxlength="4000" placeholder="改动详情、影响范围、已知问题"></textarea>
      </label>
      <div class="row" style="justify-content:flex-end">
        <button type="button" class="btn btn--ghost" @click="edit = false">{{ zh ? '取消' : 'Cancel' }}</button>
        <button type="button" class="btn btn--primary" @click="save">
          {{ form.id ? (zh ? '保存修改' : 'Save') : (zh ? '发布记录' : 'Publish') }}
        </button>
      </div>
    </form>

    <p v-if="err" class="muted" style="margin-top:12px;color:var(--err)">{{ err }}</p>
    <p v-else-if="loading && !list.length" class="muted" style="margin-top:12px">{{ zh ? '加载中…' : 'Loading…' }}</p>
    <p v-else-if="!list.length" class="muted" style="margin-top:12px">
      {{ zh ? '暂无更新日志。' : 'No entries yet.' }}
    </p>
  </div>

  <div v-if="list.length" class="tl">
    <div v-for="row in list" :key="row.id" class="tl__item">
      <div class="tl__dot" :class="'tl__dot--' + row.kind"></div>
      <div class="tl__body">
        <div class="tl__head">
          <span class="chip small">{{ row.version }}</span>
          <span class="chip small tl__tag" :class="'tl__tag--' + row.kind">{{ row.kind_label }}</span>
          <span class="muted small">{{ row.released_at }}</span>
        </div>
        <div class="tl__title">{{ row.content }}</div>
        <div v-if="row.notes" class="tl__notes muted small">{{ row.notes }}</div>
        <div class="tl__foot muted small">
          {{ zh ? '更新于' : 'updated' }} {{ row.updated_at }}
        </div>
        <div v-if="isAdmin" class="row" style="margin-top:8px;gap:8px">
          <button class="btn btn--ghost small" @click="editRow(row)">{{ zh ? '编辑' : 'Edit' }}</button>
          <button class="btn btn--ghost small btn--danger" @click="del(row)">{{ zh ? '删除' : 'Delete' }}</button>
        </div>
      </div>
    </div>
  </div>

  <div v-if="list.length" class="row" style="justify-content:center;margin-top:14px;gap:8px">
    <button class="btn btn--ghost" :disabled="page <= 1" @click="page--; load()">
      {{ zh ? '上一页' : 'Prev' }}
    </button>
    <span class="muted small">{{ page }}</span>
    <button class="btn btn--ghost" :disabled="page * size >= total" @click="page++; load()">
      {{ zh ? '下一页' : 'Next' }}
    </button>
  </div>
</template>

<style scoped>
.head-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.chip {
  cursor: pointer;
  transition: border-color .15s, color .15s;
}
.chip.active {
  border-color: var(--accent);
  color: var(--accent);
}
.tl {
  margin-top: 14px;
  position: relative;
}
.tl__item {
  display: flex;
  gap: 12px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--border);
}
.tl__item:last-child { border-bottom: none; }
.tl__dot {
  flex: 0 0 10px;
  margin-top: 8px;
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: var(--accent);
}
.tl__dot--fix { background: var(--err); }
.tl__dot--improve { background: var(--warn); }
.tl__dot--perf { background: var(--accent2); }
.tl__dot--security { background: var(--err); }
.tl__body { flex: 1; min-width: 0; }
.tl__head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.tl__title {
  font-weight: 700;
  margin-top: 6px;
  line-height: 1.5;
}
.tl__notes {
  margin-top: 4px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}
.tl__foot { margin-top: 6px; }
.tl__tag--feature { color: var(--accent); }
.tl__tag--fix { color: var(--err); }
.tl__tag--improve { color: var(--warn); }
.tl__tag--perf { color: var(--accent2); }
.tl__tag--security { color: var(--err); }
.tl__tag--refactor { color: var(--muted); }
.btn--danger { color: var(--err); }
</style>
