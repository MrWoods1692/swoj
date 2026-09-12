<script setup>
// 协议确认门禁：用户登录态已建立但尚未同意当前版本协议时弹出，
// 确认后写入服务端 terms_accepted_at，弹窗不再出现。
import { computed, ref } from 'vue'
import { api, auth, toast } from '../api'
import { TERMS_VERSION } from '../legals'

const saving = ref(false)
// 用户点「稍后处理」：本地不再弹，但服务端仍未同意，
// 下次访问若 terms_accepted_at 仍未写入则会重新弹出。
const postponed = ref(false)

// 未登录不弹；已同意当前版本或本轮已暂缓都不再弹。
const need = computed(() => !!auth.user && !postponed.value
  && auth.user.terms_accepted_at !== TERMS_VERSION)

async function accept() {
  if (saving.value) return
  saving.value = true
  try {
    await api.post('/api/terms/accept', { version: TERMS_VERSION })
    auth.user.terms_accepted_at = TERMS_VERSION
    toast('已同意用户协议与隐私政策')
  } catch (e) {
    toast(e.message || '确认失败，请稍后再试', false)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <teleport to="body">
    <div v-if="need" class="mask mask--terms" role="dialog" aria-modal="true" :aria-label="title">
      <div class="terms">
        <header class="terms__head">
          <h2>请先阅读并同意服务协议</h2>
          <p class="small muted">
            使用本站服务前，请您阅读并同意《用户协议》与《隐私政策》。
            您可以先阅读内容再决定是否同意。
          </p>
        </header>

        <div class="terms__list">
          <a class="terms__doc" href="/terms" target="_blank">
            <span class="terms__doc__k">用户协议</span>
            <span class="small muted">阅读完整条款</span>
          </a>
          <a class="terms__doc" href="/privacy" target="_blank">
            <span class="terms__doc__k">隐私政策</span>
            <span class="small muted">个人信息处理说明</span>
          </a>
        </div>

        <div class="terms__key small muted">
          <div>· 账号由校园墙授权创建，需补全真实姓名后方可提交代码。</div>
          <div>· 提交的代码会进入测评队列，并用于成绩、排名与个人主页统计。</div>
          <div>· AI 问答会把您提供的代码与问题提交给外部大模型服务。</div>
          <div>· 真实姓名、学校、成绩、排名与已解锁成就将在个人主页公开展示，可自行清空。</div>
        </div>

        <footer class="terms__foot">
          <button class="btn btn--primary" :disabled="saving" @click="accept">
            {{ saving ? '确认中…' : '我已阅读并同意' }}
          </button>
          <button class="btn btn--ghost" @click="postponed = true">
            稍后处理
          </button>
        </footer>
      </div>
    </div>
  </teleport>
</template>

<style scoped>
.mask--terms {
  position: fixed; inset: 0; z-index: 90;
  background: rgba(6, 10, 18, .72); backdrop-filter: blur(4px);
  display: flex; align-items: center; justify-content: center; padding: 24px;
}
.terms {
  width: min(560px, 100%); background: var(--panel); border: 1px solid var(--border);
  border-radius: 14px; padding: 22px 24px; box-shadow: 0 18px 50px rgba(0, 0, 0, .45);
}
.terms__head h2 { margin: 0 0 8px; font-size: 18px; }
.terms__list { display: grid; gap: 10px; margin: 18px 0; }
.terms__doc {
  display: flex; align-items: center; gap: 12px; padding: 12px 14px;
  background: var(--panel2); border: 1px solid var(--border); border-radius: 10px;
  color: var(--text); text-decoration: none;
}
.terms__doc:hover { border-color: var(--accent); }
.terms__doc__k { font-weight: 700; }
.terms__key { line-height: 1.9; padding: 12px 14px; background: var(--panel2); border-radius: 10px; }
.terms__foot { display: flex; gap: 10px; justify-content: flex-end; margin-top: 18px; }
</style>
