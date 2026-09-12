<script setup>
// 资料页：全站只读展示学习资料（外部链接 + 图标），由老师与管理员通过管理入口发布。
// 分类与图标白名单从 /api/materials/meta 取，前端不重复定义，避免两端漂移。
import { computed, onMounted, reactive, ref } from 'vue'
import { api, auth, fmtDate, toast } from '../api'
import { i18n } from '../i18n'

const list = ref([])
const cats = ref([])      // 后端白名单，含空串（全部）
const icons = ref(['book'])
const counts = ref({})    // category -> 数量
const active = ref('')    // 当前分类筛选，'' 为全部
const kw = ref('')

// 图标 -> emoji，纯展示层映射，缺失回落 book。
const ICON_EMOJI = {
  book: '📚', doc: '📄', video: '🎬', code: '💻',
  link: '🔗', note: '📝', image: '🖼️', question: '❓',
}
const iconOf = c => ICON_EMOJI[c] || ICON_EMOJI.book

onMounted(load)

async function load() {
  const meta = await api.get('/api/materials/meta').catch(() => ({}))
  cats.value = meta.categories || ['']
  icons.value = meta.icons || ['book']

  const r = await api.get(`/api/materials?category=${encodeURIComponent(active.value)}`).catch(() => ({}))
  list.value = r.list || []
  counts.value = r.categories || {}
}

const shown = computed(() => {
  const q = kw.value.trim().toLowerCase()
  if (!q) return list.value
  return list.value.filter(m =>
    m.title.toLowerCase().includes(q) ||
    (m.desc || '').toLowerCase().includes(q))
})

// 打开外部链接前先记一次点击量，失败不影响跳转。
async function open(m) {
  api.get(`/api/materials/${m.id}`).catch(() => {})
  window.open(m.url, '_blank', 'noopener,noreferrer')
}

// 录入与编辑共用同一份表单状态。
const blank = () => ({ title: '', category: '', url: '', icon: 'book', desc: '', pinned: false })
const editing = ref(false)
const draft = reactive(blank())
const formErr = ref('')

function openAdd() { Object.assign(draft, blank()); editing.value = true }
function editOne(m) {
  Object.assign(draft, blank(), { id: m.id, title: m.title, category: m.category || '', url: m.url, icon: m.icon, desc: m.desc || '', pinned: !!m.pinned })
  editing.value = true
}
async function save() {
  formErr.value = ''
  const body = { title: draft.title, category: draft.category, url: draft.url, icon: draft.icon, desc: draft.desc, pinned: draft.pinned }
  if (draft.id) {
    await api.put(`/api/admin/materials/${draft.id}`, body).catch(e => { formErr.value = e.message; return })
    toast('资料已更新')
  } else {
    await api.post('/api/admin/materials', body).catch(e => { formErr.value = e.message; return })
    toast('资料已发布')
  }
  editing.value = false
  await load()
}
async function remove(m) {
  if (!window.confirm(`删除《${m.title}》？`)) return
  await api.del(`/api/admin/materials/${m.id}`).catch(e => { formErr.value = e.message; return })
  toast('已删除')
  await load()
}

const host = u => { try { return new URL(u).host } catch { return u } }
</script>

<template>
  <div class="grid grid--2 mat-layout">
    <!-- 左侧：分类与搜索 -->
    <div>
      <div class="panel">
        <div class="row" style="justify-content: space-between; align-items: center">
          <h2 style="margin:0">{{ i18n.t('materials') }}</h2>
          <button v-if="auth.isEditor" class="btn btn--primary" @click="openAdd">＋ 添加资料</button>
        </div>
        <input v-model="kw" class="input" placeholder="搜索标题或说明…" style="margin-top: 12px" />
      </div>

      <div class="panel">
        <div v-for="c in cats" :key="c || 'all'" class="cat"
          :class="{ on: active === c }" @click="active = c; load()">
          <span>{{ c || '全部资料' }}</span>
          <span class="muted small">{{ counts[c] || 0 }}</span>
        </div>
      </div>
    </div>

    <!-- 右侧：资料列表 -->
    <div>
      <div v-if="!shown.length" class="panel muted">暂无资料</div>
      <div v-for="m in shown" :key="m.id" class="panel mat">
        <div class="mat__ic" :title="m.icon">{{ iconOf(m.icon) }}</div>
        <div class="mat__body">
          <div class="row" style="gap: 8px">
            <a :href="m.url" target="_blank" rel="noopener noreferrer" class="mat__title"
              @click="open(m)">{{ m.pinned ? '📌 ' : '' }}{{ m.title }}</a>
            <span v-if="m.category" class="chip small">{{ m.category }}</span>
          </div>
          <div v-if="m.desc" class="small muted" style="margin-top: 4px">{{ m.desc }}</div>
          <div class="row small muted" style="margin-top: 8px; gap: 12px">
            <span class="mat__host">{{ host(m.url) }}</span>
            <span>👤 {{ m.creator || '未知' }}</span>
            <span>👁 {{ m.clicks }}</span>
            <span>{{ fmtDate(m.updated_at) }}</span>
            <span v-if="auth.isEditor" style="margin-left: auto">
              <button class="btn btn--ghost small" @click="editOne(m)">编辑</button>
              <button class="btn btn--ghost small" @click="remove(m)">删除</button>
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- 录入 / 编辑弹窗 -->
  <div v-if="editing" class="mask" @click.self="editing = false">
    <div class="modal mat__modal">
      <h2 style="margin-top:0">{{ draft.id ? '编辑资料' : '添加资料' }}</h2>
      <label class="fld">标题 <span class="req">*</span>
        <input v-model="draft.title" class="input" maxlength="80" placeholder="如：洛谷入门算法讲义" />
      </label>
      <div class="row" style="gap:12px">
        <label class="fld" style="flex:1">分类
          <select v-model="draft.category" class="input">
            <option v-for="c in cats" :key="c || 'all'" :value="c">{{ c || '未分类' }}</option>
          </select>
        </label>
        <label class="fld" style="flex:0 0 120px">置顶
          <select v-model="draft.pinned" class="input">
            <option :value="false">否</option>
            <option :value="true">是</option>
          </select>
        </label>
      </div>
      <label class="fld">链接 <span class="req">*</span>
        <input v-model="draft.url" class="input" placeholder="https://…" />
      </label>
      <label class="fld">图标
        <div class="ic-pick">
          <button v-for="i in icons" :key="i" class="ic"
            :class="{ on: draft.icon === i }" :title="i" @click="draft.icon = i">{{ iconOf(i) }}</button>
        </div>
      </label>
      <label class="fld">说明
        <textarea v-model="draft.desc" class="input" rows="3" maxlength="300" placeholder="可选，一句话说明这份资料适合谁看"></textarea>
      </label>
      <div v-if="formErr" class="fld-err">{{ formErr }}</div>
      <div class="row" style="justify-content: flex-end; gap: 10px">
        <button class="btn btn--ghost" @click="editing = false">取消</button>
        <button class="btn btn--primary" @click="save">{{ draft.id ? '保存修改' : '发布资料' }}</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.cat {
  display: flex; justify-content: space-between; align-items: center;
  padding: 10px 12px; margin: 0 -12px; border-radius: 8px; cursor: pointer;
  color: var(--text);
}
.cat:hover { background: var(--panel2); }
.cat.on { background: var(--panel2); font-weight: 700; color: var(--accent); }

.mat-layout { grid-template-columns: 220px 1fr; align-items: start; }
@media (max-width: 900px) { .mat-layout { grid-template-columns: 1fr; } }
.mat { display: flex; gap: 14px; align-items: flex-start; }
.mat__ic {
  flex: 0 0 44px; height: 44px; border-radius: 10px;
  background: var(--panel2); display: grid; place-items: center; font-size: 22px;
}
.mat__body { flex: 1; min-width: 0; }
.mat__title { font-weight: 600; color: var(--text); text-decoration: none; }
.mat__title:hover { color: var(--accent); text-decoration: underline; }
.mat__host {
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 240px;
  color: var(--accent);
}

.ic-pick { display: flex; flex-wrap: wrap; gap: 8px }
.ic {
  width: 40px; height: 40px; border-radius: 8px; font-size: 18px;
  background: var(--panel2); color: var(--text);
  border: 1px solid var(--border); cursor: pointer;
}
.ic:hover { border-color: var(--accent); }
.ic.on { border-color: var(--accent); background: color-mix(in srgb, var(--accent) 14%, transparent); }

.fld { display: block; margin-bottom: 12px; font-size: 13px; color: var(--text); }
.fld-err { color: var(--error); font-size: 13px; margin-bottom: 10px; }
.req { color: var(--error); }
.mat__modal { width: min(560px, 92vw); }
</style>
