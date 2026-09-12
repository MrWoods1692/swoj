<script setup>
import { ref } from 'vue'
import { i18n } from '../i18n'

const go = (p) => (location.href = p)
</script>

<template>
  <div class="panel" style="text-align:center;padding:56px 16px">
    <div style="font-size:64px;font-weight:800;background:linear-gradient(135deg,var(--accent),var(--accent2));-webkit-background-clip:text;background-clip:text;color:transparent">404</div>
    <p class="muted">页面不存在或已被移除</p>
    <div class="row" style="justify-content:center">
      <button class="btn btn--primary" @click="go('/')">{{ i18n.t('home') }}</button>
      <button class="btn" @click="go('/problems')">{{ i18n.t('problems') }}</button>
      <button class="btn" @click="go('/about')">{{ i18n.t('about') }}</button>
    </div>
  </div>
</template>
