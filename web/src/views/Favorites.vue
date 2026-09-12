<script setup>
import { onMounted, ref } from 'vue'
import { api, toast } from '../api'
import { useRouterGuard } from '../composables'
import Pager from '../components/Pager.vue'

useRouterGuard()
const tab = ref('problem')
const list = ref([])
const total = ref(0)
const page = ref(1)
const size = 20

const label = { problem: '题目收藏', plan: '计划收藏' }

onMounted(load)

async function load() {
  const r = await api.get('/api/favorites/' + tab.value + '?page=' + page.value + '&size=' + size)
    .catch(() => ({ list: [], total: 0 }))
  list.value = r.list || []
  total.value = r.total || 0
}

function switchTab(t) {
  tab.value = t
  page.value = 1
  load()
}

async function remove(it) {
  try {
    await api.post('/api/favorites/' + tab.value + '/' + it.id)
    await load()
    toast('已取消收藏')
  } catch (e) { toast(e.message, false) }
}

const to = (it) => it.type === 'plan' ? '/training/' + it.id : '/problems/' + it.id
</script>

<template>
  <div class="panel">
    <div class="row">
      <h2 style="margin:0">我的收藏</h2>
      <span class="spacer" />
      <div class="row">
        <button class="btn" :class="{ 'btn--primary': tab === 'problem' }" @click="switchTab('problem')">题目</button>
        <button class="btn" :class="{ 'btn--primary': tab === 'plan' }" @click="switchTab('plan')">训练计划</button>
      </div>
    </div>
  </div>

  <div class="panel">
    <table class="tbl">
      <thead>
        <tr><th>名称</th><th v-if="tab === 'problem'">难度</th><th v-if="tab === 'problem'">通过率</th><th>操作</th></tr>
      </thead>
      <tbody>
        <tr v-for="it in list" :key="it.id">
          <td><router-link :to="to(it)">{{ it.name }}</router-link></td>
          <td v-if="tab === 'problem'">{{ it.difficulty }}</td>
          <td v-if="tab === 'problem'">{{ it.submit ? (it.accept / it.submit * 100).toFixed(1) + '%' : '-' }}</td>
          <td><button class="btn btn--sm" @click="remove(it)">取消收藏</button></td>
        </tr>
        <tr v-if="!list.length"><td colspan="4" class="muted">{{ label[tab] }}为空</td></tr>
      </tbody>
    </table>
    <Pager :total="total" :page="page" :size="size" @go="p => { page = p; load() }" />
  </div>
</template>
