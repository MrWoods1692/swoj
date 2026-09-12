<script setup>
import { onMounted, ref } from 'vue'
import { api, auth, fmtDate, toast } from '../api'
import { useRouterGuard } from '../composables'

useRouterGuard()
const list = ref([])
const showCreate = ref(false)
const form = ref({ name: '', info: '', deadline: '', visible: true, problem_ids: '' })

const isAdmin = () => auth.user && ['admin', 'super', 'superadmin'].includes(auth.user.role)

onMounted(async () => {
  list.value = await api.get('/api/assignments').catch(() => [])
})

async function create() {
  try {
    await api.post('/api/assignments', {
      name: form.name, info: form.info,
      deadline: form.deadline, visible: form.visible,
      problem_ids: form.problem_ids.split(/[,\s]+/).filter(Boolean).map(Number),
    })
    toast('作业已创建')
    showCreate.value = false
    form.value = { name: '', info: '', deadline: '', visible: true, problem_ids: '' }
    list.value = await api.get('/api/assignments').catch(() => [])
  } catch (e) { toast(e.message, false) }
}
</script>

<template>
  <div class="panel">
    <div class="row">
      <h2 style="margin:0">作业</h2>
      <span class="spacer" />
      <button v-if="isAdmin()" class="btn btn--primary" @click="showCreate = true">+ 创建作业</button>
    </div>
  </div>

  <div v-if="showCreate" class="panel">
    <h3>创建作业</h3>
    <label class="fld"><span>名称 *</span><input v-model="form.name" /></label>
    <label class="fld"><span>说明</span><textarea v-model="form.info" rows="2"></textarea></label>
    <label class="fld"><span>截止时间 *</span><input type="datetime-local" v-model="form.deadline" /></label>
    <label class="fld"><span>题目编号（逗号分隔）</span><input v-model="form.problem_ids" placeholder="1,2,3" /></label>
    <label class="fld"><span>是否可见</span><input type="checkbox" v-model="form.visible" /></label>
    <div class="row">
      <button class="btn btn--primary" @click="create">提交</button>
      <button class="btn" @click="showCreate = false">取消</button>
    </div>
  </div>

  <div class="grid grid--2">
    <router-link v-for="a in list" :key="a.id" :to="'/assignments/' + a.id" class="card">
      <div class="row">
        <h3 style="margin:0;flex:1">{{ a.name }}</h3>
        <span class="badge" :class="a.state === '进行中' ? 'badge--ok' : 'badge--muted'">{{ a.state }}</span>
      </div>
      <div class="muted small" style="margin-top:6px">截止 {{ fmtDate(a.deadline).slice(0, 16) }}</div>
      <div v-if="a.info" class="small" style="margin-top:6px">{{ a.info.slice(0, 80) }}</div>
      <div class="row small muted" style="margin-top:8px">
        <span>共 {{ (a.problems || []).length }} 题</span>
        <span v-if="a.submit">提交 {{ a.submit }}</span>
        <span v-if="a.accept">通过 {{ a.accept }}</span>
      </div>
    </router-link>
    <div v-if="!list.length" class="muted">暂无可见作业</div>
  </div>
</template>

<style scoped>
.badge--ok { background: rgba(34,197,94,.15); color: #86efac; }
.badge--muted { background: rgba(138,148,166,.15); color: #c7ccd6; }
</style>
