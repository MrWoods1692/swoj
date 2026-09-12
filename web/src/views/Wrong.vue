<script setup>
import { onMounted, ref } from 'vue'
import { api, fmtDate, toast } from '../api'
import { useRouterGuard } from '../composables'

useRouterGuard()
const list = ref([])

onMounted(async () => {
  list.value = await api.get('/api/wrong-questions').catch(() => [])
})

async function remove(w) {
  if (!confirm('确认从错题本移除这道题？')) return
  try {
    await api.del('/api/wrong-questions/' + w.id)
    list.value = list.value.filter(x => x.id !== w.id)
    toast('已移除')
  } catch (e) { toast(e.message, false) }
}
</script>

<template>
  <div class="panel">
    <div class="row">
      <h2 style="margin:0">错题本</h2>
      <span class="spacer" />
      <span class="chip">共 {{ list.length }} 题</span>
    </div>
    <p class="muted small" style="margin:10px 0 0">提交非 AC 状态时自动收录，AC 后自动清除。</p>
  </div>

  <div class="panel">
    <table class="tbl">
      <thead>
        <tr><th>题目</th><th>尝试次数</th><th>最近尝试</th><th style="width:110px">操作</th></tr>
      </thead>
      <tbody>
        <tr v-for="w in list" :key="w.id">
          <td><router-link :to="'/problems/' + w.problem_id">{{ w.problem_name }}</router-link></td>
          <td class="muted">{{ w.times }}</td>
          <td class="muted small">{{ fmtDate(w.last_try_at) }}</td>
          <td>
            <div class="row">
              <router-link :to="'/problems/' + w.problem_id" class="btn btn--sm">再试</router-link>
              <button class="btn btn--sm" @click="remove(w)">移除</button>
            </div>
          </td>
        </tr>
        <tr v-if="!list.length"><td colspan="4" class="muted">错题本是空的，继续保持 AC</td></tr>
      </tbody>
    </table>
  </div>
</template>
