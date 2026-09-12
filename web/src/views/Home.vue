<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api'
import { useRouterGuard } from '../composables'

useRouterGuard()
const site = ref({})
const notices = ref([])
const top = ref([])
const trends = ref([])

onMounted(async () => {
  site.value = await api.get('/api/stats/site').catch(() => ({}))
  notices.value = (await api.get('/api/notices?size=3').catch(() => ({ list: [] }))).list || []
  const lb = await api.get('/api/leaderboard?size=5').catch(() => ({ list: [] }))
  top.value = lb.list || []
  // 首页小趋势复用公开数据：拉个人提交分布不必要，这里只展示概览数字。
})

function barH(t) {
  const max = Math.max(...trends.value.map(x => x.submissions), 1)
  return Math.round((t.submissions / max) * 100) + '%'
}
</script>

<template>
  <div class="hero panel">
    <div class="hero__inner">
      <div>
        <h1>Swoj <span>Online Judge</span></h1>
        <p class="muted">校园编程在线评测 · Go + SQLite + 并发测评队列 · 仅 C++</p>
        <div class="row">
          <router-link class="btn btn--primary" to="/problems">开始刷题</router-link>
          <router-link class="btn" to="/training">训练计划</router-link>
          <router-link class="btn" to="/contests">参加比赛</router-link>
        </div>
      </div>
    </div>
  </div>

  <div class="grid grid--4" style="margin-bottom:16px">
    <div class="stat"><b>{{ site.users ?? 0 }}</b><span>注册用户</span></div>
    <div class="stat"><b>{{ site.problems ?? 0 }}</b><span>公开题目</span></div>
    <div class="stat"><b>{{ site.submissions ?? 0 }}</b><span>累计提交</span></div>
    <div class="stat"><b>{{ site.accept_rate ?? '0.00%' }}</b><span>全站通过率</span></div>
  </div>

  <div class="grid grid--2">
    <div class="panel">
      <div class="row"><h3>最新公告</h3><span class="spacer" /><router-link class="small" to="/notices">全部 →</router-link></div>
      <div v-if="!notices.length" class="muted">暂无公告</div>
      <router-link v-for="n in notices" :key="n.id" :to="`/notices/${n.id}`" class="notice-line">
        <span class="badge" :class="'lv--' + n.level">{{ n.pinned ? '置顶 · ' : '' }}{{ n.title }}</span>
        <span class="muted small">{{ (n.created_at || '').slice(0, 10) }}</span>
      </router-link>
    </div>
    <div class="panel">
      <div class="row"><h3>排行榜</h3><span class="spacer" /><router-link class="small" to="/leaderboard">完整榜单 →</router-link></div>
      <table class="tbl" v-if="top.length">
        <thead><tr><th>#</th><th>用户</th><th>AC</th><th>通过率</th></tr></thead>
        <tbody>
          <tr v-for="u in top" :key="u.rank">
            <td>{{ u.rank }}</td>
            <td><router-link :to="`/profile/${u.rank}`">{{ u.username }}</router-link></td>
            <td>{{ u.accepted }}</td><td>{{ u.rate }}</td>
          </tr>
        </tbody>
      </table>
      <div v-else class="muted">还没有提交数据</div>
    </div>
  </div>

  <div class="panel">
    <h3>今日概览</h3>
    <div class="row small">
      <span class="chip">今日提交 {{ site.today_submissions ?? 0 }}</span>
      <span class="chip">今日 AC {{ site.today_accepted ?? 0 }}</span>
      <span class="chip">活跃用户 {{ site.today_active_users ?? 0 }}</span>
      <span class="chip">在线用户 {{ site.today_online_users ?? 0 }}</span>
      <span class="chip">讨论 {{ site.discussions ?? 0 }}</span>
      <span class="chip">比赛 {{ site.contests ?? 0 }}</span>
      <span class="chip">计划 {{ site.plans ?? 0 }}</span>
    </div>
  </div>
</template>

<style scoped>
.hero__inner h1 { margin: 0 0 6px; font-size: 30px; }
.hero__inner h1 span { background: linear-gradient(135deg, var(--accent), var(--accent2)); -webkit-background-clip: text; background-clip: text; color: transparent; }
.notice-line { display: flex; justify-content: space-between; align-items: center; padding: 7px 0; border-bottom: 1px dashed var(--border); color: var(--text); }
.lv--warning { background: rgba(245,158,11,.15); color: #fcd34d; }
.lv--danger { background: rgba(239,68,68,.15); color: #fca5a5; }
.lv--success { background: rgba(34,197,94,.15); color: #86efac; }
.lv--info { background: rgba(79,140,255,.15); color: #93c5fd; }
</style>
