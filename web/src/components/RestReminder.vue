<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api, auth } from '../api'

// 休息提醒：累计在线满 1 小时提示离屏护眼，每 10 分钟自动消失一次。
// 阈值 1 小时是硬编码的护眼建议，与后端「专注六小时」成就互不干扰——
// 前者是提醒、后者是行为统计，两者不做合并。

const HOUR = 60 * 60
const TICK = 120        // 单次心跳上报秒数，远小于后端 600 秒截断
const SNOOZE = 5 * 60   // 关闭后下一次提醒的顺延间隔
const AUTO = 10 * 60    // 提醒自动消失时长

const visible = ref(false)
const countdown = ref(AUTO)
let due = HOUR          // 下一次应在累计时长达到此处时提醒
let sinceStart = null   // 本次心跳计时起点
let tickTimer = null
let countTimer = null

// 累计在线秒数：必须用 ref，模板里 fmtOnline(lastSeen) 才能随心跳刷新。
const lastSeen = ref(0)

const shown = computed(() => visible.value && countdown.value > 0)

function fmtClock(s) {
  return `${Math.floor(s / 60)} 分 ${String(s % 60).padStart(2, '0')} 秒`
}
function fmtOnline(s) {
  const h = Math.floor(s / HOUR)
  const m = Math.floor((s % HOUR) / 60)
  return h > 0 ? `${h} 小时 ${m} 分钟` : `${m} 分钟`
}

async function tick() {
  if (!auth.user || !sinceStart) return
  const now = Date.now()
  const secs = Math.max(1, Math.round((now - sinceStart) / 1000))
  sinceStart = now
  try {
    const d = await api.post('/api/points/online', { seconds: secs })
    if (typeof d?.online_seconds !== 'number') return
    lastSeen.value = d.online_seconds
    if (!visible.value && lastSeen.value >= due) fire()
  } catch {
    // 心跳失败不提示：在线时长是辅助统计，失败不打断做题。
  }
}

function fire() {
  visible.value = true
  countdown.value = AUTO
  if (countTimer) clearInterval(countTimer)
  countTimer = setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0) dismiss()
  }, 1000)
}

// 自动消失与「我已休息」共用：都推迟一个 SNOOZE 再提醒。
function dismiss() {
  if (countTimer) { clearInterval(countTimer); countTimer = null }
  visible.value = false
  due = Math.max(due, lastSeen.value) + SNOOZE
}

onMounted(() => {
  // App 挂载时 auth.me() 是异步的，此处可能尚未登录；watch 会补上。
  watch(() => auth.user, async (u, old) => {
    if (u === old) return
    if (!u) {
      // 登出：停止计时并清空。
      if (tickTimer) { clearInterval(tickTimer); tickTimer = null }
      if (countTimer) { clearInterval(countTimer); countTimer = null }
      visible.value = false
      lastSeen.value = 0
      due = HOUR
      sinceStart = null
      return
    }
    try {
      const d = await api.get('/api/points/online')
      if (typeof d?.online_seconds === 'number') lastSeen.value = d.online_seconds
    } catch { /* 忽略，心跳会再同步 */ }
    sinceStart = Date.now()
    if (lastSeen.value >= HOUR) fire()
    if (tickTimer) clearInterval(tickTimer)
    tickTimer = setInterval(tick, TICK * 1000)
  }, { immediate: true })
})

onBeforeUnmount(() => {
  if (tickTimer) clearInterval(tickTimer)
  if (countTimer) clearInterval(countTimer)
})
</script>

<template>
  <div v-if="shown" class="rest-tip">
    <div class="rest-tip__head">
      <span class="rest-tip__ico">👁</span>
      <span class="rest-tip__title">已连续在线满 1 小时</span>
    </div>
    <p class="rest-tip__body">
      离开屏幕休息 <b>10–20 分钟</b>，看看 6 米外的物体，让眼睛放松。
    </p>
    <div class="rest-tip__row">
      <span class="rest-tip__cd">已提醒 <b>{{ fmtClock(countdown) }}</b></span>
      <span class="rest-tip__prog">累计在线 <b>{{ fmtOnline(lastSeen) }}</b></span>
    </div>
    <div class="rest-tip__act">
      <span class="rest-tip__hint">10 分钟后自动消失</span>
      <button class="btn btn--primary small" @click="dismiss">我已休息</button>
    </div>
  </div>
</template>

<style scoped>
.rest-tip {
  position: fixed;
  right: 20px;
  bottom: 74px;
  z-index: 80;
  width: 320px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-left: 3px solid var(--warn);
  border-radius: 12px;
  padding: 14px 16px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, .35);
  animation: rest-in .28s ease-out;
}
@keyframes rest-in {
  from { opacity: 0; transform: translateY(12px); }
  to { opacity: 1; transform: translateY(0); }
}
.rest-tip__head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.rest-tip__ico { font-size: 18px; }
.rest-tip__title {
  font-weight: 700;
  color: var(--warn);
}
.rest-tip__body {
  margin: 8px 0 10px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--text);
}
.rest-tip__body b { color: var(--warn); font-weight: 700; }
.rest-tip__row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
  color: var(--muted);
  margin-bottom: 10px;
}
.rest-tip__row b { color: var(--text); font-weight: 600; }
.rest-tip__act {
  display: flex;
  gap: 8px;
  justify-content: space-between;
  align-items: center;
}
.rest-tip__hint { font-size: 11px; color: var(--muted); }
</style>
