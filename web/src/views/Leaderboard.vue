<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api'
import { i18n } from '../i18n'

const q = ref({ problem_id: '', contest_id: '', username: '', page: 1, size: 20 })
const list = ref([])
const loading = ref(false)

onMounted(load)

async function load() {
  if (loading.value) return
  loading.value = true
  try {
    const r = await api.get('/api/leaderboard?' + new URLSearchParams({
      problem_id: q.value.problem_id,
      contest_id: q.value.contest_id,
      username: q.value.username,
      page: q.value.page,
      page_size: q.value.size,
    }))
    list.value = r.list || []
  } finally {
    loading.value = false
  }
}

const medal = (n) => ['#fbbf24', '#cbd5e1', '#f97316'][n - 1] || ''
</script>

<template>
  <div class="panel">
    <h2>{{ i18n.t('leaderboard') }}</h2>
    <form class="row" @submit.prevent="q.page = 1; load()">
      <input v-model="q.username" placeholder="用户名筛选" />
      <input v-model="q.problem_id" placeholder="限定题号" style="width:110px" />
      <input v-model="q.contest_id" placeholder="限定比赛" style="width:110px" />
      <button class="btn btn--primary">查询</button>
    </form>
  </div>

  <div class="panel">
    <table class="tbl">
      <thead>
        <tr>
          <th style="width:60px">排名</th><th>用户</th><th>AC 题数</th><th>总用时</th>
          <th>提交数</th><th>通过率</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="u in list" :key="u.username">
          <td>
            <span v-if="u.rank <= 3" class="medal" :style="{ color: medal(u.rank) }">{{ u.rank }}</span>
            <span v-else>{{ u.rank }}</span>
          </td>
          <td>
            <router-link :to="`/profile/${u.user_id}`">{{ u.username }}</router-link>
          </td>
          <td>{{ u.accepted }}</td>
          <td class="mono">{{ u.time_used }} ms</td>
          <td class="muted">{{ u.submitted }}</td>
          <td>{{ u.rate }}</td>
        </tr>
        <tr v-if="!list.length"><td colspan="6" class="muted">暂无数据</td></tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.medal { font-size: 18px; font-weight: 800; }
</style>
