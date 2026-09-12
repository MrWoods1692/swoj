<script setup>
// 协议 / 隐私政策共用渲染器。文档文本集中在 legals.js，避免弹窗与独立页面各写一份。
import { computed } from 'vue'
import { TERMS_DOC, PRIVACY_DOC, TERMS_META } from '../legals'

const props = defineProps({
  // 'terms' 用户协议 | 'privacy' 隐私政策
  doc: { type: String, required: true },
})

const isPrivacy = computed(() => props.doc === 'privacy')
const title = computed(() => (isPrivacy.value ? '隐私政策' : '用户协议'))
const doc = computed(() => (isPrivacy.value ? PRIVACY_DOC : TERMS_DOC))
const docVersion = computed(() => (isPrivacy.value ? 'v1.0' : TERMS_META.version))
</script>

<template>
  <article class="legal">
    <header class="legal__head">
      <h1>{{ title }}</h1>
      <div class="legal__meta">
        <span class="chip small">版本 {{ docVersion }}</span>
        <span class="chip small">生效日期 {{ TERMS_META.effective }}</span>
        <span class="chip small">更新日期 {{ TERMS_META.updated }}</span>
      </div>
      <p class="small muted" style="margin-top:10px">
        本政策由 Swoj Online Judge 提供，适用于本站全部服务。若您对本政策有任何疑问，
        请通过站内公告渠道或联系管理员。
      </p>
    </header>

    <section v-for="sec in doc" :key="sec.title" class="legal__sec">
      <h2>{{ sec.title }}</h2>
      <p v-for="(p, i) in sec.paras" :key="i">{{ p }}</p>
    </section>

    <footer class="legal__foot">
      <p class="small muted">
        Swoj Online Judge · Go + SQLite + Vue3 · 校园墙授权登录
      </p>
      <div class="row" style="margin-top:10px">
        <router-link :to="isPrivacy ? '/terms' : '/privacy'" class="btn btn--sm">
          {{ isPrivacy ? '查看用户协议' : '查看隐私政策' }}
        </router-link>
        <router-link to="/about" class="btn btn--sm btn--ghost">关于本站</router-link>
      </div>
    </footer>
  </article>
</template>

<style scoped>
.legal { max-width: 820px; margin: 0 auto; padding: 24px 28px; }
.legal__head { border-bottom: 1px solid var(--border); padding-bottom: 16px; margin-bottom: 22px; }
.legal__sec { margin-bottom: 22px; }
.legal__sec h2 { font-size: 16px; color: var(--accent); margin-bottom: 10px; }
.legal__sec p { line-height: 1.9; color: var(--text); margin-bottom: 8px; font-size: 14px; }
.legal__foot { border-top: 1px solid var(--border); margin-top: 30px; padding-top: 18px; }
</style>
