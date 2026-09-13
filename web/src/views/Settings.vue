<script setup>
import { onMounted, ref } from 'vue'
import { api, auth, toast } from '../api'

const loading = ref(true)
const missing = ref(false)
const saving = ref(false)
const form = ref({ realname: '', website: '', background: '', signature: '' })

onMounted(async () => {
  if (!auth.user) { missing.value = true; loading.value = false; return }
  const d = await api.get('/api/users/' + auth.user.id + '/homepage').catch(() => null)
  if (!d) { missing.value = true; loading.value = false; return }
  const u = d.user
  form.value = {
    realname: u.realname || '',
    website: u.website || '',
    background: u.background || '',
    signature: u.signature || '',
  }
  loading.value = false
})

async function save() {
  saving.value = true
  try {
    await api.put('/api/auth/me', form.value)
    await auth.me()
    toast('资料已保存')
  } catch (e) {
    toast(e.message, false)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div v-if="missing" class="panel">
    <div class="muted">请先登录后查看个人资料。</div>
    <button class="btn btn--primary" style="margin-top:10px" @click="auth.login()">校园墙登录</button>
  </div>

  <div v-else-if="!loading" class="panel">
    <h3>个人资料</h3>
    <p class="small muted" style="margin-top:-6px">头像与 QQ 号均由校园墙授权写入，不在本页修改。</p>
    <div class="grid grid--2">
      <div class="fld">
        <span>真实姓名（首次登录后不可修改）</span>
        <template v-if="form.realname">
          <input class="fld--ro" :value="form.realname" disabled />
          <div class="fld--hint">已填写，如需修改请联系管理员</div>
        </template>
        <input v-else v-model="form.realname" placeholder="3-4 个汉字" />
      </div>
      <div class="fld">
        <span>QQ 号（授权写入，不可修改）</span>
        <input class="fld--ro" :value="auth.user.qq || '未绑定'" disabled />
      </div>
      <label class="fld"><span>个人网站</span><input v-model="form.website" placeholder="https://" /></label>
      <label class="fld"><span>背景图 URL</span><input v-model="form.background" placeholder="https://" /></label>
      <div class="fld">
        <span>头像预览（不可修改）</span>
        <div class="ava-prev">
          <img v-if="auth.user.avatar" :src="auth.user.avatar" class="ava-prev__img" alt=""
            @error="e => { e.target.style.display = 'none' }" />
          <span v-else class="ava-prev__def">{{ (auth.user.realname || auth.user.username)[0] }}</span>
        </div>
      </div>
    </div>
    <label class="fld"><span>个人简介</span><textarea v-model="form.signature" rows="2" maxlength="50" placeholder="选填，≤ 50 字"></textarea></label>
    <div class="row">
      <button class="btn btn--primary" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存资料' }}</button>
    </div>
  </div>
</template>

<style scoped>
.ava-prev { height: 40px; display: flex; align-items: center; gap: 10px; background: var(--panel2);
  border: 1px solid var(--border); border-radius: 10px; padding: 0 10px; }
.ava-prev__img { width: 40px; height: 40px; border-radius: 8px; object-fit: cover; flex: none; }
.ava-prev__def { width: 40px; height: 40px; border-radius: 8px; background: var(--panel);
  display: flex; align-items: center; justify-content: center; font-size: 16px; color: var(--text); flex: none; }
/* 只读展示列：保留表单栅格对位，但不可编辑 */
.fld--ro { background: var(--panel2); color: var(--muted); font-variant-numeric: tabular-nums; }
/* 只读列下的提示行：保持可编辑输入框的高度基线 */
.fld--hint { font-size: 12px; color: var(--muted); margin-top: 6px; }
</style>
