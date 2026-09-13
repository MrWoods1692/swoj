<script setup>
import { computed, onMounted, ref } from 'vue'
import { api, auth, fmtDate, toast } from '../api'

const props = defineProps({ id: String })
const data = ref(null)
const missing = ref(false)
const saving = ref(false)
const form = ref({ realname: '', school: '', signature: '', website: '', background: '', qq: '' })

const isMe = computed(() => auth.user && (!props.id || String(auth.user.id) === String(props.id)))

const load = async () => {
  const uid = props.id || auth.user?.id
  if (!uid) { missing.value = true; return }
  data.value = await api.get('/api/users/' + uid + '/homepage').catch(() => null)
  missing.value = !data.value
  if (data.value) {
    form.value = {
      realname: data.value.user.realname || '', school: data.value.user.school || '',
      signature: data.value.user.signature || '',
      website: data.value.user.website || '', background: data.value.user.background || '',
      qq: data.value.user.qq || '',
    }
  }
}

onMounted(load)

async function save() {
  saving.value = true
  try {
    await api.put('/api/auth/me', form.value)
    await auth.me()
    await load()
    toast('资料已保存')
  } catch (e) {
    toast(e.message, false)
  } finally {
    saving.value = false
  }
}

const heat = computed(() => {
  const map = Object.fromEntries((data.value.heatmap || []).map(h => [h.date, h]))
  const days = []
  const today = new Date()
  for (let i = 179; i >= 0; i--) {
    const d = new Date(today); d.setDate(today.getDate() - i)
    const key = d.toISOString().slice(0, 10)
    const h = map[key] || { total: 0, accepted: 0 }
    days.push({ date: key, level: heatLevel(h), title: `${key}\n提交 ${h.total} · AC ${h.accepted}` })
  }
  return days
})

function heatLevel(h) {
  if (h.total === 0) return 0
  if (h.accepted === 0) return 1
  if (h.total <= 2) return 2
  return 3
}

const heatColor = ['var(--heat-0)', 'var(--heat-1)', 'var(--heat-2)', 'var(--heat-3)']
</script>

<template>
  <div v-if="missing && isMe" class="panel">
    <div class="muted">请先登录后查看个人主页。</div>
    <button class="btn btn--primary" style="margin-top:10px" @click="auth.login()">校园墙登录</button>
  </div>
  <div v-else-if="missing" class="panel muted">用户不存在</div>

  <template v-else-if="data">
    <div class="panel cover" :style="data.user.background ? { backgroundImage: 'url(' + data.user.background + ')' } : {}">
      <div class="cover__inner">
        <img v-if="data.user.avatar" :src="data.user.avatar" class="avatar-lg" alt="" />
        <div v-else class="avatar-lg avatar-lg--def">{{ (data.user.realname || data.user.username)[0] }}</div>
        <div class="cover__body">
          <h2 style="margin:0">{{ data.user.realname || data.user.username }}</h2>
          <div class="small muted" style="margin-top:4px">
            @{{ data.user.username }}
            <span v-if="data.user.school">· {{ data.user.school }}</span>
          </div>
          <div v-if="data.user.signature" class="small" style="margin-top:8px">{{ data.user.signature }}</div>
          <div class="row" style="margin-top:10px">
            <a v-if="data.user.website" :href="data.user.website" target="_blank" class="chip small">个人网站</a>
            <span v-if="data.user.qq" class="chip small">QQ {{ data.user.qq }}</span>
            <span class="chip small">等级 {{ data.stats.level?.name }}</span>
          </div>
        </div>
      </div>
    </div>

    <div class="grid grid--4" style="margin-bottom:16px">
      <div class="stat"><b>{{ data.stats.solved }}</b><span>已通过</span></div>
      <div class="stat"><b>{{ data.stats.submitted }}</b><span>总提交</span></div>
      <div class="stat"><b>{{ data.stats.pass_rate }}%</b><span>通过率</span></div>
      <div class="stat"><b>#{{ data.stats.rank }}</b><span>全站排名</span></div>
    </div>

    <div class="grid grid--2" style="margin-bottom:16px">
      <div class="panel">
        <h3>做题热力图（近 180 天）</h3>
        <div class="heat">
          <div v-for="d in heat" :key="d.date" class="cell" :style="{ background: heatColor[d.level] }" :title="d.title"></div>
        </div>
      </div>

      <div class="panel">
        <h3>活跃度与算力</h3>
        <table class="tbl">
          <tbody>
            <tr><th>近 30 天活跃</th><td>{{ data.stats.active_days_30 }} 天</td></tr>
            <tr><th>积分</th><td>{{ data.stats.points }}</td></tr>
            <tr><th>成就</th><td>{{ data.stats.achievement_core }} / {{ data.stats.achievement_total }}</td></tr>
            <tr><th>测评耗时</th><td>{{ data.compute.total_ms }} ms</td></tr>
            <tr><th>单次峰值耗时</th><td>{{ data.compute.max_ms }} ms</td></tr>
            <tr><th>平均耗时</th><td>{{ data.compute.avg_ms ? data.compute.avg_ms.toFixed(0) : 0 }} ms</td></tr>
            <tr><th>内存峰值</th><td>{{ data.compute.max_mem_kb }} KB</td></tr>
            <tr><th>注册时间</th><td class="small">{{ fmtDate(data.stats.registered_at) }}</td></tr>
            <tr><th>最近登录</th><td class="small">{{ fmtDate(data.stats.last_login_at) }}</td></tr>
          </tbody>
        </table>
        <div class="row" style="margin-top:12px">
          <button class="btn btn--sm" @click="navigator.clipboard.writeText(data.share_url); toast('分享链接已复制')">
            复制分享链接
          </button>
        </div>
      </div>
    </div>

    <div v-if="isMe" class="panel">
      <h3>编辑资料</h3>
      <div class="grid grid--2">
        <label class="fld"><span>真实姓名</span><input v-model="form.realname" /></label>
        <label class="fld"><span>QQ 号</span><input v-model="form.qq" /></label>
        <label class="fld"><span>学校 / 学院</span><input v-model="form.school" /></label>
        <label class="fld"><span>个人网站</span><input v-model="form.website" placeholder="https://" /></label>
        <label class="fld"><span>背景图 URL</span><input v-model="form.background" placeholder="https://" /></label>
        <div class="fld">
          <span>头像（按 QQ 自动生成，不可修改）</span>
          <div class="ava-prev">
            <img v-if="data.user.avatar" :src="data.user.avatar" class="ava-prev__img" alt=""
              @error="(e) => { e.target.style.display = 'none' }" />
            <span v-else class="ava-prev__def">{{ (data.user.realname || data.user.username)[0] }}</span>
          </div>
        </div>
      </div>
      <label class="fld"><span>个人简介</span><textarea v-model="form.signature" rows="2"></textarea></label>
      <div class="row">
        <button class="btn btn--primary" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存资料' }}</button>
      </div>
    </div>
  </template>
</template>

<style scoped>
.avatar-lg { width: 72px; height: 72px; border-radius: 14px; object-fit: cover; background: var(--panel2);
  display: flex; align-items: center; justify-content: center; font-size: 26px; color: var(--text); }
.cover { background-size: cover; background-position: center; position: relative; overflow: hidden; }
.cover__inner { display: flex; gap: 18px; align-items: center; }
.heat { display: grid; grid-template-columns: repeat(26, 1fr); gap: 3px; }
.cell { aspect-ratio: 1; border-radius: 3px; }
.ava-prev { height: 40px; display: flex; align-items: center; gap: 10px; background: var(--panel2);
  border: 1px solid var(--border); border-radius: 10px; padding: 0 10px; }
.ava-prev__img { width: 40px; height: 40px; border-radius: 8px; object-fit: cover; flex: none; }
.ava-prev__def { width: 40px; height: 40px; border-radius: 8px; background: var(--panel);
  display: flex; align-items: center; justify-content: center; font-size: 16px; color: var(--text); flex: none; }
</style>
