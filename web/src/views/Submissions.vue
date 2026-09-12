<script setup>
import { onMounted, ref } from 'vue'
import { api, auth, fmtDate, toast } from '../api'
import { useRouterGuard } from '../composables'
import Pager from '../components/Pager.vue'

useRouterGuard()
const q = ref({ status: '', page: 1, size: 20 })
const list = ref([])
const total = ref(0)
const loading = ref(false)

onMounted(load)

async function load() {
  if (loading.value) return
  loading.value = true
  try {
    const r = await api.get('/api/submissions?' + new URLSearchParams({
      page: q.page, page_size: q.size,
    }))
    list.value = r.list || []
    total.value = r.total || 0
  } finally {
    loading.value = false
  }
}

const COLORS = ['#8a94a6', '#22c55e', '#ef4444', '#a855f7', '#f59e0b', '#3b82f6', '#ec4899', '#14b8a6', '#6b7280']
</script>

<template>
  <div class="panel">
    <div class="row">
      <h2 style="margin:0">测评记录</h2>
      <span class="spacer" />
      <span class="chip">共 {{ total }} 条</span>
    </div>
  </div>

  <div class="panel">
    <table class="tbl">
      <thead>
        <tr><th>ID</th><th>用户</th><th>题目</th><th>状态</th><th>用时</th><th>内存</th><th>时间</th></tr>
      </thead>
      <tbody>
        <tr v-for="s in list" :key="s.id">
          <td class="mono">{{ s.id }}</td>
          <td>{{ s.username }}</td>
          <td><router-link :to="'/problems/' + s.problem_id">{{ s.problem_name }}</router-link></td>
          <td><span class="badge" :style="{ color: COLORS[s.status] || '#8a94a6' }">{{ s.status_text }}</span></td>
          <td>{{ s.time_used }} ms</td>
          <td>{{ s.mem_used }} KB</td>
          <td class="muted small">{{ fmtDate(s.created_at) }}</td>
        </tr>
        <tr v-if="!list.length"><td colspan="7" class="muted">暂无提交记录</td></tr>
      </tbody>
    </table>
    <Pager :total="total" :page="q.page" :size="q.size" @go="p => { q.page = p; load() }" />
  </div>
</template>
