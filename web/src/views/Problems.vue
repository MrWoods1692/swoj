<script setup>
import { onMounted, ref } from 'vue'
import { api, DIFF_COLOR } from '../api'
import { useRouterGuard } from '../composables'

const q = ref({ keyword: '', difficulty: '', tag: '', page: 1, size: 20 })
const list = ref([])
const total = ref(0)
const loading = ref(false)

onMounted(useRouterGuard)

async function load() {
  if (loading.value) return
  loading.value = true
  try {
    const r = await api.get('/api/problems?' + new URLSearchParams({
      keyword: q.value.keyword,
      difficulty: q.value.difficulty,
      tag: q.value.tag,
      accepted: q.value.accepted,
      page: q.value.page,
      page_size: q.value.size,
    }))
    list.value = r.list || []
    total.value = r.total || 0
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="panel">
    <form class="row" @submit.prevent="q.page = 1; load()">
      <input v-model="q.keyword" placeholder="搜索题目名 / 题号" style="flex:1;min-width:200px" />
      <select v-model="q.difficulty">
        <option value="">全部难度</option>
        <option v-for="(c, k) in DIFF_COLOR" :key="k" :value="k">{{ k }}</option>
      </select>
      <select v-model="q.tag">
        <option value="">全部标签</option>
        <option>排序</option><option>二分</option><option>图论</option><option>动态规划</option>
        <option>字符串</option><option>模拟</option><option>数论</option><option>数学</option>
      </select>
      <select v-model="q.accepted">
        <option value="">不筛选</option>
        <option value="0">未通过</option>
        <option value="1">已通过</option>
      </select>
      <button class="btn btn--primary">搜索</button>
    </form>
  </div>

  <div class="panel">
    <table class="tbl">
      <thead>
        <tr><th style="width:70px">ID</th><th>题目</th><th>标签</th><th>难度</th><th>通过率</th><th>提交</th></tr>
      </thead>
      <tbody>
        <tr v-for="p in list" :key="p.id">
          <td class="mono">{{ p.id }}</td>
          <td><router-link :to="`/problems/${p.id}`">{{ p.name }}</router-link></td>
          <td>
            <span v-for="t in (p.tags || [])" :key="t" class="tag">{{ t }}</span>
          </td>
          <td>
            <span class="badge" :style="{ color: DIFF_COLOR[p.difficulty] || '#8a94a6' }">{{ p.difficulty }}</span>
          </td>
          <td>{{ p.accept_rate }}%</td>
          <td class="muted">{{ p.submissions }}</td>
        </tr>
        <tr v-if="!list.length"><td colspan="6" class="muted">没有匹配的题目</td></tr>
      </tbody>
    </table>
    <Pager :total="total" :page="q.page" :size="q.size" @go="p => (q.page = p, load())" />
  </div>
</template>

<style scoped>
.tag { display: inline-block; padding: 1px 7px; margin: 1px 2px 1px 0; border-radius: 999px;
  background: var(--panel2); border: 1px solid var(--border); font-size: 11px; color: var(--muted); }
</style>
