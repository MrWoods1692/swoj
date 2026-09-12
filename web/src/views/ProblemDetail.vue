<script setup>
import { computed, onMounted, ref } from 'vue'
import { api, auth, DIFF_COLOR, fmtDate, toast } from '../api'
import { useRouterGuard } from '../composables'

const props = defineProps({ id: String })
const problem = ref(null)
const tags = ref([])
const recent = ref([])
const fav = ref(false)
const code = ref('#include <bits/stdc++.h>\nusing namespace std;\n\nint main() {\n    // TODO: 在这里写你的解法\n    return 0;\n}\n')
const submitting = ref(false)

onMounted(async () => {
  try {
    const r = await api.get('/api/problems/' + props.id)
    problem.value = r.problem
    tags.value = r.tags || []
  } catch (e) {
    problem.value = { error: e.message || '题目加载失败' }
    return
  }
  recent.value = (await api.get('/api/submissions?problem_id=' + props.id + '&size=10')
    .catch(() => ({ list: [] }))).list || []
  if (auth.user) {
    const fav = (await api.get('/api/favorites/problem?size=200').catch(() => ({ list: [] }))).list || []
    fav.value = fav.some(x => String(x.id) === String(props.id))
  }
})

const rate = computed(() => problem.value?.accept_rate != null
  ? problem.value.accept_rate.toFixed(2) + '%' : '-')

async function toggleFav() {
  try {
    await api.post('/api/favorites/problem/' + props.id)
    fav.value = !fav.value
    toast(fav.value ? '已加入收藏' : '已取消收藏')
  } catch (e) { toast(e.message, false) }
}

async function submit() {
  if (!code.value.trim()) { toast('请先填写代码', false); return }
  if (!auth.user) { auth.login(); return }
  if (!auth.user.can_submit) { toast('请先完善资料（昵称/签名）后再提交', false); return }
  submitting.value = true
  try {
    await api.post('/api/submissions', { problem_id: Number(props.id), code: code.value, mode: 'cpp' })
    toast('提交成功，正在测评')
    setTimeout(async () => {
      recent.value = (await api.get('/api/submissions?problem_id=' + props.id + '&size=10')
        .catch(() => ({ list: [] }))).list || []
    }, 1500)
  } catch (e) {
    toast(e.message, false)
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div v-if="problem && problem.error" class="panel muted">{{ problem.error }}</div>

  <template v-else-if="problem">
    <div class="panel">
      <div class="row">
        <h2 style="margin:0;flex:1">{{ problem.name }}</h2>
        <span class="badge" :style="{ color: DIFF_COLOR[problem.difficulty] || '#8a94a6' }">{{ problem.difficulty }}</span>
        <button class="btn btn--sm" @click="toggleFav">{{ fav ? '★ 已收藏' : '☆ 收藏' }}</button>
      </div>
      <div class="row small muted" style="margin-top:8px">
        <span class="chip">时间 {{ problem.time_limit }} ms</span>
        <span class="chip">内存 {{ problem.mem_limit }} MB</span>
        <span class="chip">通过 {{ problem.accept }}</span>
        <span class="chip">提交 {{ problem.submit }}</span>
        <span class="chip">通过率 {{ rate }}</span>
        <span v-if="problem.created_at" class="chip">收录 {{ fmtDate(problem.created_at).slice(0,10) }}</span>
      </div>
      <div class="muted small" style="margin-top:6px">
        <template v-for="t in tags" :key="t">#{{ t }} </template>
      </div>
    </div>

    <div class="panel">
      <div class="row"><h3 style="margin:0">题面</h3></div>
      <div class="mono">{{ problem.content }}</div>
      <div v-if="problem.hint" class="row" style="margin-top:14px">
        <span class="badge" style="background:rgba(245,158,11,.15);color:#fcd34d">提示</span>
        <span class="small">{{ problem.hint }}</span>
      </div>
    </div>

    <div class="panel">
      <div class="row">
        <h3 style="margin:0">编辑器 <span class="small muted">C++</span></h3>
        <span class="spacer" />
        <button class="btn btn--primary" :disabled="submitting" @click="submit">
          {{ submitting ? '提交中…' : '提交评测' }}
        </button>
      </div>
      <textarea v-model="code" class="codearea" spellcheck="false"></textarea>
    </div>

    <div class="panel">
      <h3>最近提交</h3>
      <table class="tbl" v-if="recent.length">
        <thead><tr><th>ID</th><th>用户</th><th>状态</th><th>用时</th><th>时间</th></tr></thead>
        <tbody>
          <tr v-for="s in recent" :key="s.id">
            <td class="mono">{{ s.id }}</td>
            <td>{{ s.username }}</td>
            <td><span class="badge" :style="{ color: ['#8a94a6','#22c55e','#ef4444','#a855f7','#f59e0b','#3b82f6','#ec4899','#14b8a6','#6b7280'][s.status] || '#8a94a6' }">{{ s.status_text }}</span></td>
            <td>{{ s.time_used }} ms</td>
            <td class="muted small">{{ fmtDate(s.created_at) }}</td>
          </tr>
        </tbody>
      </table>
      <div v-else class="muted">暂无提交</div>
    </div>
  </template>

  <div v-else class="panel muted">加载中…</div>
</template>
