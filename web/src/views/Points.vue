<script setup>
import { onMounted, ref } from 'vue'
import { api, fmtDate, toast } from '../api'
import { useRouterGuard } from '../composables'

useRouterGuard()
const data = ref({ balance: 0, streak: 0, log: [], rules: {} })
const rank = ref([])
const online = ref({ hours: 0, points_awarded: 0, balance: 0 })
const busy = ref(false)

onMounted(async () => {
  data.value = await api.get('/api/points').catch(() => data.value)
  rank.value = await api.get('/api/points/rank?limit=30').catch(() => [])
  online.value = await api.get('/api/points/online').catch(() => ({ hours: 0 }))
})

async function checkin() {
  busy.value = true
  try {
    const r = await api.post('/api/points/checkin')
    toast(`签到成功 +${r.points} 积分，连续 ${r.streak} 天`)
    data.value.balance = r.balance ?? data.value.balance
    data.value.streak = r.streak
    online.value.balance = r.balance ?? online.value.balance
  } catch (e) { toast(e.message, false) }
  finally { busy.value = false }
}

const day = new Date().toISOString().slice(0, 10)
const checkedToday = () => data.value.checked_today || data.value.checkedIn
</script>

<template>
  <div class="grid grid--4" style="margin-bottom:16px">
    <div class="stat"><b>{{ data.balance }}</b><span>当前积分</span></div>
    <div class="stat"><b>{{ data.streak }}</b><span>连续签到</span></div>
    <div class="stat"><b>{{ online.hours }}</b><span>在线小时</span></div>
    <div class="stat"><b>{{ online.points_awarded }}</b><span>在线积分</span></div>
  </div>

  <div class="grid grid--2">
    <div class="panel">
      <div class="row">
        <h3 style="margin:0">每日签到</h3>
        <span class="spacer" />
        <button class="btn btn--primary" :disabled="busy" @click="checkin">立即签到</button>
      </div>
      <div class="small muted" style="margin-top:8px">
        基础 {{ data.rules?.sign_base ?? '-' }} 分，连续 +{{ data.rules?.sign_bonus ?? '-' }}/天，
        最高 {{ data.rules?.sign_max ?? '-' }} 分
      </div>
    </div>

    <div class="panel">
      <h3>积分规则</h3>
      <table class="tbl">
        <tbody>
          <tr><th>AC 一题</th><td>+{{ data.rules?.ac ?? data.rules?.ac_default ?? '-' }}</td></tr>
          <tr><th>在线 {{ data.rules?.online_hours ?? '-' }} 小时</th><td>+{{ data.rules?.online_points ?? '-' }}</td></tr>
          <tr><th>比赛名次</th><td>管理员按场设置</td></tr>
        </tbody>
      </table>
    </div>
  </div>

  <div class="panel">
    <h3>积分流水</h3>
    <table class="tbl">
      <thead><tr><th>时间</th><th>类别</th><th>变动</th><th>说明</th></tr></thead>
      <tbody>
        <tr v-for="l in data.log" :key="l.id">
          <td class="muted small">{{ fmtDate(l.created_at) }}</td>
          <td class="muted">{{ l.category }}</td>
          <td :style="{ color: l.delta >= 0 ? 'var(--ok)' : 'var(--err)' }">
            {{ l.delta >= 0 ? '+' : '' }}{{ l.delta }}
          </td>
          <td class="muted small">{{ l.remark }}</td>
        </tr>
        <tr v-if="!data.log?.length"><td colspan="4" class="muted">暂无流水</td></tr>
      </tbody>
    </table>
  </div>

  <div class="panel">
    <h3>积分排行榜</h3>
    <table class="tbl">
      <tbody>
        <tr v-for="(u, i) in rank" :key="u.rank || i">
          <td>{{ u.rank || i + 1 }}</td>
          <td>{{ u.username }}</td>
          <td style="color:var(--accent)">{{ u.points }}</td>
        </tr>
        <tr v-if="!rank.length"><td colspan="3" class="muted">暂无数据</td></tr>
      </tbody>
    </table>
  </div>
</template>
