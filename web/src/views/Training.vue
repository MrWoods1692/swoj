<script setup>
import { onMounted, ref } from 'vue'
import { api, auth, toast } from '../api'
import { useRouterGuard } from '../composables'

// 训练计划列表：后端一次返回全部可见计划，无分页无搜索。
useRouterGuard()
const list = ref([])
const showCreate = ref(false)
const form = ref({ name: '', info: '', problem_id: '', visible: true })

const isAdmin = () => auth.user && ['admin', 'super', 'superadmin'].includes(auth.user.role)

async function refresh() {
  try { list.value = await api.get('/api/training-plans') } catch { list.value = [] }
}
onMounted(refresh)

async function submitCreate() {
  try {
    await api.post('/api/training-plans', {
      name: form.name, info: form.info,
      problem_id: Number(form.problem_id) || 0, visible: form.visible,
    })
    toast('计划已创建')
    showCreate.value = false
    form.value = { name: '', info: '', problem_id: '', visible: true }
    await refresh()
  } catch (e) { toast(e.message || '创建失败', false) }
}

function pct(p) {
  if (!p.total) return 0
  return Math.round((p.accepted / p.total) * 100)
}
</script>

<template>
  <div class="panel">
    <div class="row">
      <h2 style="margin:0">训练计划</h2>
      <span class="spacer" />
      <button v-if="isAdmin()" class="btn btn--primary" @click="showCreate = true">+ 创建计划</button>
    </div>
  </div>

  <div v-if="showCreate" class="panel">
    <h3>创建训练计划</h3>
    <label class="fld"><span>计划名称 *</span><input v-model="form.name" /></label>
    <label class="fld"><span>说明</span><textarea v-model="form.info" rows="3" placeholder="训练目标、难度走向…"></textarea></label>
    <label class="fld"><span>题目编号 *</span><input v-model="form.problem_id" placeholder="例如 12" /></label>
    <label class="fld"><span>是否可见</span><input type="checkbox" v-model="form.visible" /></label>
    <div class="row">
      <button class="btn btn--primary" @click="submitCreate">提交</button>
      <button class="btn" @click="showCreate = false">取消</button>
    </div>
  </div>

  <div class="grid grid--2">
    <router-link v-for="p in list" :key="p.id" :to="'/training/' + p.id" class="card">
      <div class="row">
        <h3 style="margin:0">{{ p.name }}</h3>
        <span class="spacer" />
        <span class="badge">{{ p.accepted }}/{{ p.total }}</span>
      </div>
      <div class="muted small" v-if="p.info">{{ p.info }}</div>
      <div class="bar"><div class="bar__in" :style="{ width: pct(p) + '%' }"></div></div>
    </router-link>
    <div v-if="!list.length" class="muted">暂无可见训练计划</div>
  </div>
</template>

<style scoped>
.bar { height: 6px; margin-top: 8px; background: var(--panel2); border-radius: 999px; overflow: hidden; }
.bar__in { height: 100%; background: linear-gradient(90deg, var(--accent), var(--accent2)); border-radius: 999px; }
</style>
