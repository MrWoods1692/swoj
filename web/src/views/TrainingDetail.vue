<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api'

// 计划详情：后端返回单个 PlanResp，records 已按 sequence 排序。
const props = defineProps({ id: String })
const plan = ref(null)

onMounted(async () => {
  try {
    plan.value = await api.get('/api/training-plans/' + props.id)
  } catch (e) {
    plan.value = { error: e.message || '加载失败' }
  }
})
</script>

<template>
  <div v-if="plan && plan.error" class="panel muted">{{ plan.error }}</div>

  <template v-else-if="plan">
    <div class="panel">
      <h2>{{ plan.name }}</h2>
      <div class="muted" v-if="plan.info">{{ plan.info }}</div>
      <div class="row" style="margin-top:10px">
        <span class="chip">共 {{ plan.total }} 题</span>
        <span class="chip">已 AC {{ plan.accepted }}</span>
      </div>
    </div>

    <div class="panel">
      <table class="tbl">
        <thead><tr><th>顺序</th><th>题目</th><th>状态</th><th>尝试次数</th></tr></thead>
        <tbody>
          <tr v-for="r in plan.records" :key="r.problem_id">
            <td>{{ r.sequence }}</td>
            <td><router-link :to="'/problems/' + r.problem_id">{{ r.problem_name }}</router-link></td>
            <td><span class="badge" :class="r.accepted ? 'badge--ok' : 'badge--pending'">{{ r.accepted ? '已 AC' : '未 AC' }}</span></td>
            <td class="muted">{{ r.times }}</td>
          </tr>
          <tr v-if="!plan.records || !plan.records.length">
            <td colspan="4" class="muted">该计划暂未关联题目</td>
          </tr>
        </tbody>
      </table>
    </div>
  </template>
</template>
