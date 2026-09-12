<script setup>
import { onMounted, ref } from 'vue'
import { api, auth, fmtDate, toast, STATUS, STATUS_COLOR } from '../api'

const question = ref('')
const code = ref('')
const problemId = ref('')
const status = ref(0)
const sending = ref(false)
const answer = ref(null)
const history = ref([])
const disabled = ref(false)
const login = ref(false)

const ask = async () => {
  if (!question.value.trim()) { toast('请输入问题', false); return }
  sending.value = true
  try {
    const r = await api.post('/api/ai/ask', {
      problem_id: problemId.value ? Number(problemId.value) : 0,
      question: question.value,
      code: code.value,
      status: status.value,
    })
    answer.value = r
    history.value.unshift({ ...r, created_at: new Date().toISOString() })
    question.value = ''
  } catch (e) {
    if (/登录/.test(e.message)) login.value = true
    if (/未启用/.test(e.message)) disabled.value = true
    toast(e.message, false)
  } finally {
    sending.value = false
  }
}

onMounted(async () => {
  history.value = await api.get('/api/ai/history').catch(() => [])
  login.value = !auth.user
})
</script>

<template>
  <div class="grid grid--2">
    <div>
      <div class="panel">
        <h2>AI 问答</h2>
        <p class="small muted" style="line-height:1.7">
          贴入你的代码与当前判题状态，AI 会结合题目上下文给出分析与改进建议。
        </p>
        <button v-if="login" class="btn btn--primary" style="width:100%" @click="auth.login()">登录后使用</button>

        <template v-else>
          <label class="fld">
            <span>关联题目 ID（可选）</span>
            <input v-model="problemId" type="number" placeholder="如 12，不填则纯文本问答" />
          </label>
          <label class="fld">
            <span>当前判题状态（可选）</span>
            <select v-model="status">
              <option :value="0">— 不指定 —</option>
              <option v-for="(s, i) in STATUS" :key="i" :value="i">{{ s }}</option>
            </select>
          </label>
          <label class="fld">
            <span>代码（可选，最多 12000 字符）</span>
            <textarea v-model="code" rows="6" class="mono" placeholder="#include <bits/stdc++.h> ..."
                      :maxlength="20000"></textarea>
          </label>
          <label class="fld">
            <span>问题 *</span>
            <textarea v-model="question" rows="3"
                      placeholder="例如：这份代码为什么会 TLE？瓶颈在哪个环节？"></textarea>
          </label>
          <div class="row">
            <button class="btn btn--primary" :disabled="sending" @click="ask">
              {{ sending ? '思考中…' : '提问' }}
            </button>
            <span v-if="sending" class="small muted">首次回答可能需要 10–30 秒</span>
          </div>
          <div v-if="disabled" class="panel" style="margin-top:14px">
            <div class="small" style="color:var(--warning)">
              AI 服务未启用。请在服务端设置 SWOJ_AI_ENABLED 与 SWOJ_AI_KEY 后重启。
            </div>
          </div>
        </template>
      </div>

      <div v-if="answer" class="panel">
        <div class="row">
          <h3 style="margin:0">回答</h3>
          <span class="chip small" v-if="answer.source">{{ answer.source }}</span>
        </div>
        <div class="mono" style="white-space:pre-wrap;line-height:1.7">{{ answer.answer }}</div>
        <div class="small muted" style="margin-top:10px">
          记录 ID {{ answer.id }} · 已写入历史记录
        </div>
      </div>
    </div>

    <div class="panel">
      <h3>历史记录</h3>
      <div v-if="!history.length" class="muted">暂无提问记录</div>
      <div v-for="h in history" :key="h.id" class="qa">
        <div class="row">
          <span class="small" style="font-weight:700">Q</span>
          <span class="spacer" />
          <span class="small muted">{{ fmtDate(h.created_at) }}</span>
          <span v-if="h.source" class="chip small">{{ h.source }}</span>
        </div>
        <div class="small" style="margin-top:4px;white-space:pre-wrap">{{ h.question }}</div>
        <div class="small muted mono" style="margin-top:8px;white-space:pre-wrap;max-height:120px;overflow:auto">
          {{ h.answer }}
        </div>
        <div class="row" style="margin-top:8px">
          <router-link v-if="h.problem_id" :to="'/problems/' + h.problem_id" class="btn btn--sm">
            题目 #{{ h.problem_id }}
          </router-link>
          <button class="btn btn--sm" @click="question = h.question">再次提问</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.qa { padding: 12px 0; border-bottom: 1px solid var(--border); }
</style>
