<script setup>
import { onMounted, ref } from 'vue'
import { api, fmtDate } from '../api'

const list = ref([])
const page = ref(1)
const size = 10
const total = ref(0)

onMounted(load)

async function load() {
  const r = await api.get(`/api/notices?page=${page.value}&size=${size}`).catch(() => ({}))
  list.value = r.list || []
  total.value = r.total || 0
}

const levelColor = { info: '#3b82f6', success: '#22c55e', warning: '#f59e0b', danger: '#ef4444' }
</script>

<template>
  <div class="panel">
    <h2>公告</h2>
  </div>

  <div class="panel">
    <div v-if="!list.length" class="muted">暂无公告</div>
    <div v-for="n in list" :key="n.id">
      <router-link :to="`/notices/${n.id}`" class="notice-row">
        <span class="badge" :style="{ color: levelColor[n.level] || '#8a94a6' }">{{ n.level }}</span>
        <span style="font-weight:600" :class="{ pinned: n.pinned }">
          {{ n.pinned ? '📌 ' : '' }}{{ n.title }}
        </span>
        <span class="spacer" />
        <span class="small muted">{{ fmtDate(n.updated_at) }}</span>
        <span class="small muted">👁 {{ n.views }}</span>
      </router-link>
    </div>
    <Pager :total="total" :page="page" :size="size" @go="p => (page = p, load())" />
  </div>
</template>

<style scoped>
.notice-row { display: flex; align-items: center; gap: 10px; padding: 12px 4px;
  border-bottom: 1px solid var(--border); text-decoration: none; color: var(--text); }
.notice-row:hover { color: var(--accent); }
.pinned { color: var(--warning); }
</style>
