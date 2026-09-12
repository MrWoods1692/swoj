<script setup>
import { onMounted, ref } from 'vue'
import { api, auth, fmtDate, toast } from '../api'
import { useRouterGuard } from '../composables'
import Pager from '../components/Pager.vue'

useRouterGuard()
const q = ref({ keyword: '', problem_id: '', page: 1, size: 20 })
const list = ref([])
const total = ref(0)
const showCreate = ref(false)
const form = ref({ title: '', content: '', problem_id: '' })

onMounted(load)

async function load() {
  const r = await api.get('/api/discussions?' + new URLSearchParams({
    keyword: q.keyword, problem_id: q.problem_id,
    page: q.page, page_size: q.size,
  })).catch(() => ({ list: [], total: 0 }))
  list.value = r.list || []
  total.value = r.total || 0
}

async function create() {
  if (!form.title.trim()) { toast('请填写标题', false); return }
  try {
    await api.post('/api/discussions', {
      title: form.title, content: form.content,
      problem_id: Number(form.problem_id) || 0,
    })
    toast('讨论已发布')
    showCreate.value = false
    form.value = { title: '', content: '', problem_id: '' }
    q.page = 1
    await load()
  } catch (e) { toast(e.message, false) }
}

async function like(d) {
  try {
    await api.post('/api/discussions/' + d.id + '/like')
    d.liked = !d.liked
    d.like_count += d.liked ? 1 : -1
  } catch (e) { toast(e.message, false) }
}
</script>

<template>
  <div class="panel">
    <div class="row">
      <h2 style="margin:0">讨论区</h2>
      <span class="spacer" />
      <button class="btn btn--primary" @click="showCreate = true">+ 发起讨论</button>
    </div>
    <form class="row" style="margin-top:12px" @submit.prevent="q.page = 1; load()">
      <input v-model="q.keyword" placeholder="搜索标题 / 内容" style="flex:1;min-width:200px" />
      <input v-model="q.problem_id" placeholder="限定题号" style="width:110px" />
      <button class="btn">搜索</button>
    </form>
  </div>

  <div v-if="showCreate" class="panel">
    <h3>发起讨论</h3>
    <label class="fld"><span>标题 *</span><input v-model="form.title" /></label>
    <label class="fld"><span>关联题目（可选）</span><input v-model="form.problem_id" placeholder="题号，留空表示通用讨论" /></label>
    <label class="fld"><span>内容</span><textarea v-model="form.content" rows="4"></textarea></label>
    <div class="row">
      <button class="btn btn--primary" @click="create">发布</button>
      <button class="btn" @click="showCreate = false">取消</button>
    </div>
  </div>

  <div class="panel">
    <div v-if="!list.length" class="muted">还没有讨论，来发起第一个吧</div>
    <router-link v-for="d in list" :key="d.id" :to="'/discussions/' + d.id" class="disc-row">
      <div class="row">
        <span class="disc-title">{{ d.title }}</span>
        <span v-if="d.problem_name" class="chip small">{{ d.problem_name }}</span>
        <span class="spacer" />
        <span class="small muted">💬 {{ d.replies || 0 }}</span>
        <button class="btn btn--sm" @click.stop="like(d)">{{ d.liked ? '❤' : '🤍' }} {{ d.like_count || 0 }}</button>
      </div>
      <div class="small muted">
        <router-link :to="'/profile/' + d.user_id">{{ d.username }}</router-link>
        · {{ fmtDate(d.created_at) }}
      </div>
    </router-link>
    <Pager v-if="total > q.size" :total="total" :page="q.page" :size="q.size" @go="p => { q.page = p; load() }" />
  </div>
</template>

<style scoped>
.disc-row { display: block; padding: 12px 0; border-bottom: 1px solid var(--border); }
.disc-row:hover .disc-title { color: var(--accent); text-decoration: none; }
.disc-title { font-weight: 600; }
</style>
