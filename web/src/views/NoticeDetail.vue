<script setup>
import { onMounted, ref } from 'vue'
import { api, fmtDate } from '../api'

const props = defineProps({ id: String })
const notice = ref(null)
const failed = ref(false)

onMounted(async () => {
  notice.value = await api.get('/api/notices/' + props.id).catch(() => null)
  failed.value = !notice.value
})

const levelColor = { info: '#3b82f6', success: '#22c55e', warning: '#f59e0b', danger: '#ef4444' }
</script>

<template>
  <div v-if="failed" class="panel muted">公告不存在或已删除</div>

  <div v-else-if="notice" class="panel">
    <div class="row">
      <span class="badge" :style="{ color: levelColor[notice.level] || '#8a94a6' }">{{ notice.level }}</span>
      <h2 style="margin:0;flex:1">{{ notice.pinned ? '📌 ' : '' }}{{ notice.title }}</h2>
    </div>
    <div class="small muted" style="margin-top:8px">
      {{ fmtDate(notice.updated_at) }} · 阅读 {{ notice.views }} 次
    </div>
    <div class="mono" style="margin-top:16px;white-space:pre-wrap;line-height:1.7">{{ notice.content }}</div>
    <div class="row" style="margin-top:24px">
      <router-link to="/notices" class="btn btn--sm">返回公告列表</router-link>
    </div>
  </div>
</template>
