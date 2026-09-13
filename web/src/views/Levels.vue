<script setup>
import { onMounted, ref } from 'vue'
import { api, toast } from '../api'
import { useRouterGuard } from '../composables'

useRouterGuard()
const me = ref({ points: 0, level: null, cost_to_next: 0, achievements: 0 })
const tiers = ref([])
const ach = ref({ list: [], unlocked: 0, total: 0 })
const buying = ref(false)

onMounted(async () => {
  me.value = await api.get('/api/levels/me').catch(() => me.value)
  tiers.value = (await api.get('/api/levels').catch(() => ({ tiers: [] }))).tiers || []
  ach.value = await api.get('/api/achievements').catch(() => ach.value)
})

async function buy(steps) {
  if (!confirm(`确认消耗积分升 ${steps} 级？`)) return
  buying.value = true
  try {
    await api.post('/api/levels/buy', { levels: steps })
    me.value = await api.get('/api/levels/me')
    toast(`升级成功，当前 ${me.value.level?.name ?? 'Lv'}`)
  } catch (e) { toast(e.message, false) }
  finally { buying.value = false }
}
</script>

<template>
  <div class="panel">
    <div class="row">
      <div>
        <h2 style="margin:0">{{ me.level?.name ?? 'Lv 0' }}</h2>
        <div class="muted small">
          第 {{ me.level?.level ?? 0 }} 级 · 积分 {{ me.points }} · 已解锁成就 {{ me.achievements }}
        </div>
      </div>
      <span class="spacer" />
      <div class="row">
        <button class="btn" :disabled="buying || me.cost_to_next === 0" @click="buy(1)">
          升 1 级
        </button>
        <button class="btn" :disabled="buying || me.cost_to_next === 0" @click="buy(3)">升 3 级</button>
      </div>
    </div>
    <div class="small muted" style="margin-top:8px">
      下一级还需 {{ me.cost_to_next }} 积分
    </div>
  </div>

  <div class="panel">
    <h3>等级体系</h3>
    <table class="tbl">
      <thead><tr><th>等级</th><th>名称</th><th>积分门槛</th></tr></thead>
      <tbody>
        <tr v-for="t in tiers" :key="t.level">
          <td>{{ t.level }}</td>
          <td>{{ t.name }}</td>
          <td class="muted">{{ t.min }}</td>
        </tr>
        <tr v-if="!tiers.length"><td colspan="3" class="muted">暂无等级配置</td></tr>
      </tbody>
    </table>
  </div>

  <div class="panel">
    <h3>成就 {{ ach.unlocked }}/{{ ach.total }}</h3>
    <div class="grid grid--3">
      <div v-for="a in ach.list" :key="a.code" class="card"
           :style="{ opacity: a.unlocked ? 1 : 0.45 }">
        <div class="row">
          <span style="font-size:20px">{{ a.icon }}</span>
          <span style="flex:1;font-weight:600">{{ a.name }}</span>
          <span class="badge badge--ok" v-if="a.unlocked">已解锁</span>
        </div>
        <div class="small muted" style="margin-top:6px">{{ a.desc }}</div>
        <div v-if="a.unlocked_at" class="small muted" style="margin-top:4px">{{ a.unlocked_at }}</div>
      </div>
    </div>
  </div>
</template>
