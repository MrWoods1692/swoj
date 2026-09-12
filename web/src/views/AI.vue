<script setup>
import { onMounted, ref, computed } from 'vue'
import { api, auth, fmtDate, toast, STATUS } from '../api'

const question = ref('')
const code = ref('')
const problemId = ref('')
const status = ref(0)
const files = ref([])
const sending = ref(false)
const answer = ref(null)
const history = ref([])
const disabled = ref(false)
const login = ref(false)

const isAdmin = computed(() => auth.isAdmin)

const ask = async () => {
  if (!question.value.trim() && !files.value.length && !code.value.trim()) {
    toast('请输入问题或上传文件', false); return
  }
  sending.value = true
  answer.value = null
  const fd = new FormData()
  if (problemId.value) fd.append('problem_id', problemId.value)
  if (status.value) fd.append('status', status.value)
  if (question.value) fd.append('question', question.value)
  if (code.value) fd.append('code', code.value)
  for (const f of files.value) fd.append('files[]', f, f.name)
  try {
    const r = await api.upload('/api/ai/ask', fd)
    answer.value = { answer: r.answer, source: r.source, id: r.id }
    history.value.unshift({
      id: r.id, answer: r.answer, source: r.source,
      question: question.value || '(文件附件) ' + files.value.map(f => f.name).join(', '),
      problem_id: problemId.value ? Number(problemId.value) : 0,
      created_at: new Date().toISOString(),
    })
    question.value = ''
    code.value = ''
    files.value = []
  } catch (e) {
    if (/登录/.test(e.message)) login.value = true
    if (/未启用|Token/.test(e.message)) disabled.value = true
    toast(e.message, false)
  } finally {
    sending.value = false
  }
}

const onFileChange = (ev) => {
  const list = ev.target.files
  const allowed = ['.cpp','.cxx','.cc','.c','.h','.hpp','.txt','.in','.out','.md','.py','.java','.js','.ts','.go','.rs','.sh','.sql','.json','.xml','.yml','.yaml','.csv','.log']
  for (const f of list) {
    const ext = '.' + (f.name.split('.').pop() || '').toLowerCase()
    if (!allowed.includes(ext)) { toast('不支持的文件类型：' + f.name, false); continue }
    if (f.size > 1024 * 1024) { toast('文件超过 1MB：' + f.name, false); continue }
    if (files.value.length >= 5) { toast('最多上传 5 个文件', false); break }
    files.value.push(f)
  }
  ev.target.value = ''
}

onMounted(async () => {
  login.value = !auth.user
  if (auth.user) {
    history.value = await api.get('/api/ai/history').catch(() => [])
  }
})

// AI 提示词（仅管理员可见）
const myPrompt = ref('')
const myToken = ref('')
const clearToken = ref(false)
const myTokenMask = ref('')
const defaultPrompt = ref('')
const promptLoaded = ref(false)
const savingPrompt = ref(false)

onMounted(async () => {
  if (!isAdmin.value) return
  const r = await api.get('/api/admin/ai-config').catch(() => null)
  if (!r) return
  myPrompt.value = r.system_prompt || ''
  myTokenMask.value = r.token_masked || ''
  defaultPrompt.value = r.default_prompt || ''
  promptLoaded.value = true
})

const saveCfg = async () => {
  savingPrompt.value = true
  try {
    await api.put('/api/admin/ai-config', {
      token: myToken.value, provider: 'yunzhi', system_prompt: myPrompt.value,
      clear_token: clearToken.value,
    })
    toast('AI 配置已保存')
    const r = await api.get('/api/admin/ai-config').catch(() => null)
    if (r) myTokenMask.value = r.token_masked || ''
    myToken.value = ''
    clearToken.value = false
  } catch (e) { toast(e.message, false) }
  finally { savingPrompt.value = false }
}
</script>

<template>
  <div class="grid grid--2">
    <div>
      <div class="panel">
        <h2>AI 编程问答</h2>
        <p class="small muted" style="line-height:1.7">
          支持纯文本提问与文件上传（.cpp / .txt / .in / .out 等），AI 会结合代码与题目上下文给出分析与建议。对话记录仅保留 7 天。
        </p>
        <button v-if="login" class="btn btn--primary" style="width:100%" @click="auth.login()">登录后使用</button>

        <template v-else>
          <div class="fld">
            <span class="small">附件上传（.cpp / .txt / .in / .out 等，单文件 ≤ 1MB，最多 5 个）</span>
            <input type="file" multiple
                   accept=".cpp,.cxx,.cc,.c,.h,.hpp,.txt,.in,.out,.md,.py,.java,.js,.ts,.go,.rs,.sh,.sql,.json,.xml,.yml,.yaml,.csv,.log"
                   @change="onFileChange" />
            <div v-if="files.length" class="row small" style="margin-top:6px">
              <span v-for="f in files" :key="f.name" class="chip">
                {{ f.name }} · {{ (f.size/1024).toFixed(1) }}KB
                <button class="btn btn--sm" @click="files.splice(files.indexOf(f), 1)">移除</button>
              </span>
            </div>
          </div>

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
            <span>代码（可选，也可通过文件上传）</span>
            <textarea v-model="code" rows="6" class="mono" placeholder="#include <bits/stdc++.h> ..." :maxlength="20000"></textarea>
          </label>

          <label class="fld">
            <span>问题 <span class="small muted">（不填时默认请 AI 分析所附文件）</span></span>
            <textarea v-model="question" rows="3"
                      placeholder="例如：这份代码为什么会 TLE？瓶颈在哪个环节？"></textarea>
          </label>

          <div class="row">
            <button class="btn btn--primary" :disabled="sending" @click="ask">
              {{ sending ? '思考中…' : '提问' }}
            </button>
            <span v-if="sending" class="small muted">首次回答可能需要 10–60 秒</span>
          </div>

          <div v-if="disabled" class="panel" style="margin-top:14px">
            <div class="small" style="color:var(--warning)">
              AI 服务未启用或未配置 Token。请管理员进入管理后台 → AI 配置，填入 yunzhiapi.cn 颁发的 Token 后重试。
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
          记录 ID {{ answer.id }} · 已写入历史记录（保留 7 天）
        </div>
      </div>

      <div v-if="isAdmin && promptLoaded" class="panel">
        <h3>AI 配置（仅管理员可见）</h3>
        <p class="small muted">
          对话记录仅保留 7 天。留空提示词则使用内置默认提示词。Token 留空表示不修改。
        </p>
        <label class="fld">
          <span>Token
            <span class="small muted">（当前掩码：{{ myTokenMask || '未配置' }}）</span>
          </span>
          <input v-model="myToken" type="text"
                 placeholder="粘贴 yunzhiapi.cn 账号里的 Token（留空则不修改）" />
        </label>
        <label class="fld" style="display:flex;align-items:center;gap:8px">
          <input type="checkbox" v-model="clearToken" />
          <span class="small">清除已保存的 Token（保存时执行）</span>
        </label>
        <label class="fld">
          <span>模型提示词</span>
          <textarea v-model="myPrompt" rows="5" class="mono"
                    placeholder="留空使用默认提示词"
                    style="width:100%"></textarea>
        </label>
        <div class="row">
          <button class="btn btn--primary" @click="saveCfg" :disabled="savingPrompt">
            {{ savingPrompt ? '保存中…' : '保存 AI 配置' }}
          </button>
          <button class="btn" @click="myToken = ''">清空 Token 输入</button>
          <button class="btn btn--sm" @click="myPrompt = defaultPrompt">提示词使用默认</button>
        </div>
      </div>
    </div>

    <div class="panel">
      <div class="row">
        <h3 style="margin:0">历史记录</h3>
        <span class="small muted">保留 7 天</span>
      </div>
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
