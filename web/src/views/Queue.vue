<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { api, auth, fmtDate, toast } from '../api'

const data = ref(null)
const denied = ref(false)
const timer = ref(null)

const load = async () => {
  try {
    data.value = await api.get('/api/admin/queue')
    denied.value = false
  } catch (e) {
    denied.value = !auth.user || !auth.isAdmin
    if (!denied.value) toast(e.message, false)
  }
}

onMounted(() => {
  load()
  timer.value = setInterval(load, 5000)
})
onUnmounted(() => timer.value && clearInterval(timer.value))
</script>

<template>
  <div v-if="denied" class="panel muted">
    测评队列监控需要管理员权限。
    <button class="btn btn--primary" style="margin-left:10px" @click="auth.login()">登录</button>
  </div>

  <template v-else-if="data">
    <div class="grid grid--4" style="margin-bottom:16px">
      <div class="stat"><b>{{ data.stats.running }}</b><span>执行中</span></div>
      <div class="stat"><b>{{ data.stats.pending }}</b><span>等待中</span></div>
      <div class="stat"><b>{{ data.pending_submissions }}</b><span>待测评提交</span></div>
      <div class="stat"><b>{{ data.stats.total }}</b><span>累计测评</span></div>
    </div>

    <div class="panel">
      <div class="row">
        <h3 style="margin:0">最近测评记录</h3>
        <span class="spacer" />
        <span class="chip small">容量 {{ data.stats.capacity }} · 线程 {{ data.stats.workers }}</span>
      </div>
      <table class="tbl">
        <thead>
          <tr><th>ID</th><th>用户</th><th>题目</th><th>状态</th><th>用时</th><th>内存</th><th>提交时间</th></tr>
        </thead>
        <tbody>
          <tr v-for="it in data.recent" :key="it.id">
            <td class="mono">{{ it.id }}</td>
            <td>{{ it.username }}</td>
            <td>{{ it.problem_name }}</td>
            <td>
              <span class="badge" :style="{ color: ['#8a94a6','#22c55e','#ef4444','#a855f7','#f59e0b','#3b82f6','#ec4899','#14b8a6','#6b7280'][it.status] || '#8a94a6' }">
                {{ it.status_text }}
              </span>
            </td>
            <td>{{ it.time_used }} ms</td>
            <td>{{ it.mem_used }} KB</td>
            <td class="small muted">{{ fmtDate(it.created_at) }}</td>
          </tr>
          <tr v-if="!data.recent.length"><td colspan="7" class="muted">暂无测评记录</td></tr>
        </tbody>
      </table>
    </div>
  </template>
</template>
