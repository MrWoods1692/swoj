<script setup>
import { onMounted, ref } from 'vue'
import { auth, toast } from './api'
import { i18n } from './i18n'
import TermsGate from './components/TermsGate.vue'
import RestReminder from './components/RestReminder.vue'

const collapsed = ref(true)

function setTheme(theme) {
  document.documentElement.setAttribute('data-theme', theme)
  localStorage.setItem('swoj_theme', theme)
  themeLabel.value = theme === 'dark' ? '☀️' : '🌙'
}

const themeLabel = ref('🌙')

function toggleTheme() {
  const next = document.documentElement.getAttribute('data-theme') === 'dark' ? 'light' : 'dark'
  setTheme(next)
}

async function doLogout() {
  await auth.logout()
  toast('已退出登录')
  location.href = '/'
}

onMounted(() => {
  auth.me()
  const saved = localStorage.getItem('swoj_theme')
  setTheme(saved === 'dark' ? 'dark' : 'light')
})
</script>

<template>
  <header class="topbar">
    <div class="topbar__inner">
      <router-link to="/" class="brand" title="School Wall Online Judge"><img src="/logo.webp" class="brand__logo" alt="Swoj" />Swoj<span>OJ</span></router-link>
      <nav class="nav" :class="{ 'nav--open': !collapsed }">
        <router-link to="/problems">{{ i18n.t('problems') }}</router-link>
        <router-link to="/training">{{ i18n.t('training') }}</router-link>
        <router-link to="/contests">{{ i18n.t('contests') }}</router-link>
        <router-link to="/assignments">{{ i18n.t('assignments') }}</router-link>
        <router-link to="/discussions">{{ i18n.t('discussions') }}</router-link>
        <router-link to="/submissions">{{ i18n.t('submissions') }}</router-link>
        <router-link to="/leaderboard">{{ i18n.t('leaderboard') }}</router-link>
        <router-link to="/levels">{{ i18n.t('levels') }}</router-link>
        <router-link to="/points">{{ i18n.t('points') }}</router-link>
        <router-link to="/notices">{{ i18n.t('notices') }}</router-link>
        <router-link to="/materials">{{ i18n.t('materials') }}</router-link>
        <router-link to="/changelog">{{ i18n.t('changelog') }}</router-link>
        <router-link to="/status">{{ i18n.t('status') }}</router-link>
        <router-link to="/about">{{ i18n.t('about') }}</router-link>
      </nav>
      <div class="topbar__right">
        <button class="btn btn--ghost" @click="i18n.toggle()">{{ i18n.lang === 'zh' ? 'EN' : '中' }}</button>
        <button class="btn btn--ghost" @click="toggleTheme" title="切换亮/暗主题">{{ themeLabel }}</button>
        <template v-if="auth.user">
          <router-link v-if="auth.isAdmin" to="/admin" class="btn btn--ghost">{{ i18n.t('admin') }}</router-link>
          <router-link to="/notes" class="chip">{{ i18n.t('notes') }}</router-link>
          <router-link to="/proposals" class="chip">{{ i18n.t('proposals') }}</router-link>
      <router-link to="/logs" class="chip">{{ i18n.t('logs') }}</router-link>
          <router-link to="/me" class="chip">
            <img v-if="auth.user.avatar" :src="auth.user.avatar" class="chip__ava" alt="" />
            <span v-else class="chip__ava chip__ava--def">{{ (auth.user.realname || auth.user.username)[0] }}</span>
            {{ auth.user.realname || auth.user.username }}
          </router-link>
          <button class="btn btn--ghost" @click="doLogout">{{ i18n.t('logout') }}</button>
        </template>
        <button v-else class="btn btn--primary" @click="auth.login()">{{ i18n.t('login') }}</button>
      </div>
    </div>
  </header>
  <main class="page"><router-view /></main>
  <TermsGate v-if="auth.user" />
  <RestReminder />
  <footer class="foot">
    <router-link to="/terms">{{ i18n.t('terms') }}</router-link>
    <span> · </span>
    <router-link to="/privacy">{{ i18n.t('privacy') }}</router-link>
    <span> · Swoj · School Wall Online Judge · Go + SQLite + Vue3 · 校园墙授权登录</span>
  </footer>
</template>
