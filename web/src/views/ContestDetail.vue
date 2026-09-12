<script setup>
import { computed, onMounted, ref } from 'vue'
import { api, fmtDate, toast } from '../api'
import { useRouterGuard } from '../composables'

const props = defineProps({ id: String })
const contest = ref(null)
const rank = ref([])
const enrolled = ref(false)
const submitting = ref(false)
const editor = ref({ code: '', problem: '' })

onMounted(async () => {
  try {
    const c = await api.get('/api/contests/' + props.id)
    contest.value = c
    enrolled.value = c.enrolled
  } catch (e) {
    contest.value = { error: e.message || '比赛加载失败' }
    return
  }
  if (c.rank) rank.value = await api.get('/api/contests/' + props.id + '/rank').catch(() => [])
})

async function enroll() {
  try {
    await api.post('/api/contests/' + props.id + '/enroll')
    enrolled.value = true
    toast('报名成功')
  } catch (e) { toast(e.message, false) }
}

async function exportRank(format) {
  try {
    await api.download('/api/contests/' + props.id + '/rank/export?format=' + format, 'contest-rank.' + (format === 'markdown' ? 'md' : format))
    toast('导出成功')
  } catch (e) { toast(e.message, false) }
}

const exportFormats = [
  { key: 'csv', label: 'CSV' },
  { key: 'tsv', label: 'TSV' },
  { key: 'xlsx', label: 'Excel' },
  { key: 'json', label: 'JSON' },
  { key: 'markdown', label: 'Markdown' },
  { key: 'html', label: 'HTML' },
]

async function submitCurrent() {
  if (!editor.problem) { toast('请选择一道题目', false); return }
  if (!editor.code.trim()) { toast('请先填写代码', false); return }
  submitting.value = true
  try {
    const r = await api.post('/api/submissions', {
      problem_id: Number(editor.problem), code: editor.code,
      mode: 'cpp', contest_id: Number(props.id),
    })
    toast('提交成功，正在测评')
    if (contest.value.rank) {
      setTimeout(async () => {
        rank.value = await api.get('/api/contests/' + props.id + '/rank').catch(() => rank.value)
      }, 1500)
    }
  } catch (e) {
    toast(e.message, false)
  } finally {
    submitting.value = false
  }
}

const problemName = computed(() => {
  const p = (contest.value?.problems || []).find(x => String(x.id) === String(editor.problem))
  return p ? p.name : ''
})
</script>

<template>
  <div v-if="contest && contest.error" class="panel muted">{{ contest.error }}</div>

  <template v-else-if="contest">
    <div class="panel">
      <div class="row">
        <h2 style="margin:0;flex:1">{{ contest.name }}</h2>
        <span class="badge" :class="contest.state === '进行中' ? 'badge--ok' : 'badge--muted'">{{ contest.state }}</span>
        <button v-if="!enrolled" class="btn btn--primary" @click="enroll">报名</button>
        <span v-else class="chip">已报名</span>
      </div>
      <div class="row small muted" style="margin-top:8px">
        <span class="chip">{{ fmtDate(contest.start_time).slice(0, 16) }}</span>
        <span class="chip">{{ fmtDate(contest.end_time).slice(0, 16) }}</span>
        <span class="chip">提交 {{ contest.submit }}</span>
        <span class="chip">通过 {{ contest.accept }}</span>
      </div>
      <div v-if="contest.info" class="small" style="margin-top:8px">{{ contest.info }}</div>
    </div>

    <div class="editor">
      <div class="panel" style="margin-bottom:0">
        <h3>题目</h3>
        <select v-model="editor.problem" style="width:100%;margin-bottom:10px">
          <option value="">选择题目…</option>
          <option v-for="p in contest.problems" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>
        <textarea v-model="editor.code" class="codearea" placeholder="// 在此填写代码" spellcheck="false"></textarea>
        <div class="row" style="margin-top:10px">
          <button class="btn btn--primary" :disabled="submitting || contest.state !== '进行中'" @click="submitCurrent">
            {{ submitting ? '提交中…' : '提交' }}
          </button>
          <router-link v-if="editor.problem" :to="'/problems/' + editor.problem" class="btn">查看题面</router-link>
          <span class="small muted">{{ problemName }}</span>
        </div>
      </div>

      <div class="panel" style="margin-bottom:0">
        <div class="rank-head">
          <h3>排行榜</h3>
          <div class="export-group" v-if="contest.rank">
            <span class="muted small">导出：</span>
            <button v-for="f in exportFormats" :key="f.key" class="btn btn--ghost btn--sm" @click="exportRank(f.key)">{{ f.label }}</button>
          </div>
        </div>
        <div v-if="!contest.rank" class="muted small">该比赛未开放排行榜</div>
        <table class="tbl" v-else-if="rank.length">
          <thead><tr><th>#</th><th>用户</th><th>AC</th><th>首次通过</th></tr></thead>
          <tbody>
            <tr v-for="(u, i) in rank" :key="u.username">
              <td>{{ i + 1 }}</td>
              <td>{{ u.username }}</td>
              <td>{{ u.accepted }}</td>
              <td class="muted small">{{ fmtDate(u.first_at) }}</td>
            </tr>
          </tbody>
        </table>
        <div v-else class="muted small">暂无上榜记录</div>
      </div>
    </div>
  </template>

  <div v-else class="panel muted">加载中…</div>
</template>

<style scoped>
.rank-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; }
.rank-head h3 { margin: 0; }
.export-group { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.export-group .btn { font-size: 12px; padding: 4px 10px; }
.badge--ok { background: rgba(34,197,94,.15); color: #86efac; }
.badge--muted { background: rgba(138,148,166,.15); color: #c7ccd6; }
</style>
