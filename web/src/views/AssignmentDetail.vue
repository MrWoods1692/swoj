<script setup>
import { onMounted, ref } from 'vue'
import { api, fmtDate, toast } from '../api'
import { useRouterGuard } from '../composables'

const props = defineProps({ id: String })
const assignment = ref(null)

onMounted(async () => {
  try {
    assignment.value = await api.get('/api/assignments/' + props.id)
  } catch (e) {
    assignment.value = { error: e.message || '作业加载失败' }
  }
})
</script>

<template>
  <div v-if="assignment && assignment.error" class="panel muted">{{ assignment.error }}</div>

  <template v-else-if="assignment">
    <div class="panel">
      <div class="row">
        <h2 style="margin:0;flex:1">{{ assignment.name }}</h2>
        <span class="badge" :class="assignment.state === '进行中' ? 'badge--ok' : 'badge--muted'">{{ assignment.state }}</span>
      </div>
      <div class="row small muted" style="margin-top:8px">
        <span class="chip">截止 {{ fmtDate(assignment.deadline).slice(0, 16) }}</span>
        <span class="chip">共 {{ (assignment.problems || []).length }} 题</span>
      </div>
      <div v-if="assignment.info" class="small" style="margin-top:8px">{{ assignment.info }}</div>
    </div>

    <div class="panel">
      <h3>题目列表</h3>
      <table class="tbl">
        <thead><tr><th>ID</th><th>题目</th><th>难度</th><th>通过</th><th>提交</th></tr></thead>
        <tbody>
          <tr v-for="p in assignment.problems" :key="p.id">
            <td class="mono">{{ p.id }}</td>
            <td><router-link :to="'/problems/' + p.id">{{ p.name }}</router-link></td>
            <td>{{ p.difficulty }}</td>
            <td>{{ p.accept }}</td>
            <td class="muted">{{ p.submit }}</td>
          </tr>
          <tr v-if="!(assignment.problems || []).length">
            <td colspan="5" class="muted">该作业暂未关联题目</td>
          </tr>
        </tbody>
      </table>
    </div>
  </template>

  <div v-else class="panel muted">加载中…</div>
</template>

<style scoped>
.badge--ok { background: rgba(34,197,94,.15); color: #86efac; }
.badge--muted { background: rgba(138,148,166,.15); color: #c7ccd6; }
</style>
