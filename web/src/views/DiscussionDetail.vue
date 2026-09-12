<script setup>
import { onMounted, ref } from 'vue'
import { api, auth, fmtDate, toast } from '../api'
import { useRouterGuard } from '../composables'

const props = defineProps({ id: String })
const d = ref(null)
const replies = ref([])
const reply = ref('')

onMounted(async () => {
  try {
    const r = await api.get('/api/discussions/' + props.id)
    d.value = r.d || r
    replies.value = r.replies || []
  } catch (e) {
    d.value = { error: e.message || '讨论加载失败' }
  }
})

async function like() {
  try {
    await api.post('/api/discussions/' + props.id + '/like')
    d.value.liked = !d.value.liked
    d.value.like_count += d.value.liked ? 1 : -1
  } catch (e) { toast(e.message, false) }
}

async function send() {
  if (!reply.value.trim()) return
  try {
    await api.post('/api/discussions/' + props.id + '/reply', { content: reply.value })
    reply.value = ''
    const r = await api.get('/api/discussions/' + props.id)
    replies.value = r.replies || []
    d.value.replies = (d.value.replies || 0) + 1
  } catch (e) { toast(e.message, false) }
}
</script>

<template>
  <div v-if="d && d.error" class="panel muted">{{ d.error }}</div>

  <template v-else-if="d">
    <div class="panel">
      <div class="row">
        <h2 style="margin:0;flex:1">{{ d.title }}</h2>
        <button class="btn" @click="like">{{ d.liked ? '❤' : '🤍' }} {{ d.like_count || 0 }}</button>
      </div>
      <div class="small muted" style="margin-top:6px">
        <router-link :to="'/profile/' + d.user_id">{{ d.username }}</router-link>
        · {{ fmtDate(d.created_at) }}
        <span v-if="d.problem_name"> · <router-link :to="'/problems/' + d.problem_id">{{ d.problem_name }}</router-link></span>
      </div>
      <div class="mono" style="margin-top:12px;white-space:pre-wrap">{{ d.content }}</div>
    </div>

    <div class="panel">
      <h3>回复 {{ replies.length }}</h3>
      <div v-if="!replies.length" class="muted">暂无回复</div>
      <div v-for="r in replies" :key="r.id" class="reply">
        <div class="small muted">
          <router-link :to="'/profile/' + r.user_id">{{ r.username }}</router-link>
          · {{ fmtDate(r.created_at) }}
        </div>
        <div class="mono">{{ r.content }}</div>
      </div>

      <form class="row" style="margin-top:14px" @submit.prevent="send">
        <input v-model="reply" placeholder="写下你的回复" style="flex:1;min-width:200px" />
        <button class="btn btn--primary">回复</button>
      </form>
    </div>
  </template>

  <div v-else class="panel muted">加载中…</div>
</template>

<style scoped>
.reply { padding: 10px 0; border-bottom: 1px solid var(--border); }
</style>
