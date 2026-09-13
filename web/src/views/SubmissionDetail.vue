<script setup>
import { computed, onMounted, ref } from 'vue'
import { api, fmtDate, toast } from '../api'
import { useRouterGuard } from '../composables'

const props = defineProps({ id: String })
const data = ref(null)
const poll = ref(null)

const statusColor = (s) =>
  ['#8a94a6', '#22c55e', '#ef4444', '#a855f7', '#f59e0b', '#3b82f6', '#ec4899', '#14b8a6', '#6b7280'][s] || '#8a94a6'

const stillRunning = computed(() => data.value && data.value.submission.status <= 0)

onMounted(async () => {
  await load()
  if (stillRunning.value) poll.value = setInterval(load, 2000)
})

async function load() {
  try {
    data.value = await api.get('/api/submissions/' + props.id)
    if (!stillRunning.value && poll.value) {
      clearInterval(poll.value)
      poll.value = null
    }
  } catch (e) {
    data.value = { error: e.message || '提交加载失败' }
  }
}

function copy(text) {
  if (!text) return
  navigator.clipboard.writeText(text).then(() => toast('已复制'), () => toast('复制失败', false))
}
</script>

<template>
  <div v-if="data && data.error" class="panel muted">{{ data.error }}</div>

  <template v-else-if="data">
    <div class="panel">
      <div class="row">
        <h2 style="margin:0;flex:1">提交 #{{ data.submission.id }}</h2>
        <span class="badge" :style="{ color: statusColor(data.submission.status) }">
          {{ data.submission.status_text }}
          <span v-if="stillRunning" class="muted small"> · 测评中…</span>
        </span>
        <button v-if="data.code" class="btn btn--sm" @click="copy(data.code)">复制代码</button>
      </div>
      <table class="tbl" style="margin-top:12px">
        <tbody>
          <tr><th>用户</th><td>{{ data.submission.username }}</td></tr>
          <tr><th>题目</th><td><router-link :to="'/problems/' + data.submission.problem_id">{{ data.submission.problem_name }}</router-link></td></tr>
          <tr><th>用时</th><td>{{ data.submission.time_used }} ms</td></tr>
          <tr><th>内存</th><td>{{ data.submission.mem_used }} KB</td></tr>
          <tr><th>时间</th><td class="muted">{{ fmtDate(data.submission.created_at) }}</td></tr>
        </tbody>
      </table>
    </div>

    <div class="panel" v-if="data.error">
      <h3>错误信息</h3>
      <div class="mono" style="color:var(--err)">{{ data.error }}</div>
    </div>

    <div class="panel" v-if="data.code">
      <h3>提交的代码</h3>
      <textarea class="codearea" :value="data.code" readonly rows="24" spellcheck="false"></textarea>
    </div>
  </template>

  <div v-else class="panel muted">加载中…</div>
</template>
