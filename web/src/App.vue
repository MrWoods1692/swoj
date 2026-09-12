<script setup>
import { onMounted, ref } from 'vue'
import { auth, toast } from './api'
import { i18n } from './i18n'
import TermsGate from './components/TermsGate.vue'

const collapsed = ref(true)

async function doLogout() {
  await auth.logout()
  toast('已退出登录')
  location.href = '/'
}

onMounted(() => auth.me())
</script>

<template>
  <header class="topbar">
    <div class="topbar__inner">
      <router-link to="/" class="brand">Swoj<span>OJ</span></router-link>
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
        <template v-if="auth.user">
          <router-link v-if="auth.isAdmin" to="/admin" class="btn btn--ghost">{{ i18n.t('admin') }}</router-link>
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
  <footer class="foot">
    <router-link to="/terms">{{ i18n.t('terms') }}</router-link>
    <span> · </span>
    <router-link to="/privacy">{{ i18n.t('privacy') }}</router-link>
    <span> · Swoj Online Judge · Go + SQLite + Vue3 · 校园墙授权登录</span>
  </footer>
</template>
