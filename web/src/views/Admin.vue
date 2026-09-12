<script setup>
// 管理后台首页：概览 + 各管理域入口。
// 各域的具体增删改查直接调用 /api/admin/* 与 /api/stats/admin，
// 在此页只做导航与顶部指标展示，避免单文件膨胀。
import { computed, onMounted, ref } from 'vue'
import { api, auth, fmtDate } from '../api'

const stats = ref(null)
const srv = ref(null)
const err = ref('')

onMounted(async () => {
  if (!auth.isAdmin) return
  stats.value = await api.get('/api/stats/admin').catch(e => { err.value = e.message; return null })
  srv.value = await api.get('/api/status').catch(() => null)
})

const ok = computed(() => auth.isAdmin)
const ov = computed(() => stats.value?.overview || {})
const dist = computed(() => stats.value?.status_dist || [])
const trends = computed(() => stats.value?.trends || [])
const maxTrend = computed(() => Math.max(1, ...trends.value.map(t => t.submissions)))

const entries = [
  { to: '/notices', label: '公告', desc: '创建与上下架公告' },
  { to: '/admin', label: '题目', desc: '题目与测试用例' },
  { to: '/training', label: '训练计划', desc: '训练计划与题目编排' },
  { to: '/contests', label: '比赛', desc: '比赛与积分规则' },
  { to: '/assignments', label: '作业', desc: '作业与截止时间' },
  { to: '/wrong', label: '错题', desc: '错题数据核查' },
  { to: '/queue', label: '测评队列', desc: '队列与节点监控' },
  { to: '/shop', label: '积分商城', desc: '商品与订单' },
  { to: '/points', label: '积分', desc: '积分调整与发放' },
  { to: '/leaderboard', label: '用户', desc: '用户与角色' },
  { to: '/status', label: '服务状态', desc: '节点与运行指标' },
  { to: '/ai', label: 'AI 问答', desc: '问答记录与配置' },
  { to: '/admin/logs', label: '全局日志', desc: '全站操作日志审计' },
]

const kvs = [
  ['注册用户', ov.value.users], ['题目总数', ov.value.problems],
  ['提交总数', ov.value.submissions], ['AC 总数', ov.value.accepted],
  ['AC 率', ov.value.accept_rate], ['讨论帖', ov.value.discussions],
  ['今日提交', ov.value.today_submissions], ['今日 AC', ov.value.today_accepted],
  ['今日活跃', ov.value.today_active_users],
]
</script>

<template>
  <div v-if="!ok" class="panel muted">只有管理员可以访问管理后台。</div>

  <template v-else>
    <div class="panel">
      <div class="row" style="justify-content: space-between">
        <h2 style="margin:0">管理后台</h2>
        <span class="chip small">
          {{ auth.user?.realname || auth.user?.username }} · {{ auth.user?.role }}
        </span>
      </div>
    </div>

    <div v-if="err" class="panel" style="color: var(--error)">统计加载失败：{{ err }}</div>

    <template v-else-if="stats">
      <div class="grid grid--4">
        <div v-for="[k, v] in kvs" :key="k" class="stat">
          <b>{{ v ?? '-' }}</b><span>{{ k }}</span>
        </div>
      </div>

      <div class="grid grid--2" style="margin-bottom:16px">
        <div class="panel">
          <h3>近 30 天提交趋势</h3>
          <div class="bars">
            <div v-for="t in trends" :key="t.date" class="bar" :title="`${t.date}  提交 ${t.submissions} · AC ${t.accepted}`">
              <div style="background:#f59e0b" :style="{ height: (t.submissions / maxTrend) * 100 + '%' }"></div>
              <div style="background:#22c55e" :style="{ height: (t.accepted / maxTrend) * 100 + '%', position:'absolute', bottom:0, left:0, width:'100%' }"></div>
            </div>
          </div>
        </div>
        <div class="panel">
          <h3>判题状态分布</h3>
          <table class="tbl">
            <thead><tr><th>状态</th><th>数量</th></tr></thead>
            <tbody>
              <tr v-for="d in dist" :key="d.status"><td>{{ d.text }}</td><td>{{ d.count }}</td></tr>
              <tr v-if="!dist.length"><td colspan="2" class="muted small">暂无数据</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>

    <div class="panel">
      <h3>管理入口</h3>
      <div class="grid grid--4">
        <router-link v-for="e in entries" :key="e.to + e.label" :to="e.to" class="adm">
          <b>{{ e.label }}</b>
          <span class="small muted">{{ e.desc }}</span>
        </router-link>
      </div>
    </div>

    <div class="panel">
      <h3>运行状态</h3>
      <p class="small muted">
        <template v-if="srv">
          服务 {{ srv.status }} · 版本 {{ srv.version }} · 启动于 {{ fmtDate(srv.started_at) }} ·
          队列 {{ srv.queue?.pending }} 待处理 / 容量 {{ srv.queue?.capacity }} · 工作进程 {{ srv.queue?.workers }}
        </template>
        <span v-else>状态不可用</span>
      </p>
    </div>
  </template>
</template>

<style scoped>
.adm {
  display: block; padding: 12px 14px; margin-bottom: 10px;
  background: var(--panel2); border: 1px solid var(--border); border-radius: 10px;
  color: var(--text); text-decoration: none;
}
.adm:hover { border-color: var(--accent); }
.adm b { display: block; margin-bottom: 4px; }
</style>
