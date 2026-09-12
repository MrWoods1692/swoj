<script setup>
import { onMounted, ref } from 'vue'
import { api, auth, fmtDate, toast } from '../api'
import { useRouterGuard } from '../composables'

useRouterGuard()
const list = ref([])
const showCreate = ref(false)
const form = ref({ name: '', info: '', start: '', end: '', rank: false, problems: '' })

onMounted(async () => {
  list.value = await api.get('/api/contests').catch(() => [])
})

async function create() {
  try {
    await api.post('/api/contests', {
      name: form.name, info: form.info,
      start_time: form.start, end_time: form.end,
      rank: form.rank,
      problem_ids: form.problems.split(/[,\s]+/).filter(Boolean).map(Number),
      visible: true,
    })
    toast('比赛已创建')
    showCreate.value = false
    form.value = { name: '', info: '', start: '', end: '', rank: false, problems: '' }
    list.value = await api.get('/api/contests').catch(() => [])
  } catch (e) { toast(e.message, false) }
}
</script>

<template>
  <div class="panel">
    <div class="row">
      <h2 style="margin:0">比赛</h2>
      <span class="spacer" />
      <button v-if="auth.user && ['admin','super','superadmin'].includes(auth.user.role)"
              class="btn btn--primary" @click="showCreate = true">+ 创建比赛</button>
    </div>
  </div>

  <div v-if="showCreate" class="panel">
    <h3>创建比赛</h3>
    <label class="fld"><span>名称 *</span><input v-model="form.name" /></label>
    <label class="fld"><span>简介</span><textarea v-model="form.info" rows="2"></textarea></label>
    <div class="grid grid--2">
      <label class="fld"><span>开始时间 *</span><input type="datetime-local" v-model="form.start" /></label>
      <label class="fld"><span>结束时间 *</span><input type="datetime-local" v-model="form.end" /></label>
    </div>
    <label class="fld"><span>题目编号（逗号分隔）</span><input v-model="form.problems" placeholder="1,2,3" /></label>
    <label class="fld"><span>是否开放排行榜</span><input type="checkbox" v-model="form.rank" /></label>
    <div class="row">
      <button class="btn btn--primary" @click="create">提交</button>
      <button class="btn" @click="showCreate = false">取消</button>
    </div>
  </div>

  <div class="grid grid--2">
    <router-link v-for="c in list" :key="c.id" :to="'/contests/' + c.id" class="card">
      <div class="row">
        <h3 style="margin:0;flex:1">{{ c.name }}</h3>
        <span class="badge" :class="c.state === '进行中' ? 'badge--ok' : c.state === '已结束' ? 'badge--muted' : 'badge--warn'">
          {{ c.state }}
        </span>
      </div>
      <div class="muted small" style="margin-top:6px">
        {{ fmtDate(c.start_time).slice(0,16) }} ~ {{ fmtDate(c.end_time).slice(5,16) }}
      </div>
      <div v-if="c.info" class="small" style="margin-top:6px">{{ c.info.slice(0, 80) }}</div>
      <div class="row small muted" style="margin-top:8px">
        <span>共 {{ (c.problems || []).length }} 题</span>
        <span>提交 {{ c.submit }}</span>
        <span>通过 {{ c.accept }}</span>
      </div>
    </router-link>
    <div v-if="!list.length" class="muted">暂无可见比赛</div>
  </div>
</template>

<style scoped>
.badge--ok { background: rgba(34,197,94,.15); color: #86efac; }
.badge--muted { background: rgba(138,148,166,.15); color: #c7ccd6; }
.badge--warn { background: rgba(245,158,11,.15); color: #fcd34d; }
</style>
