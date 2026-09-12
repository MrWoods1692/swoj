<script setup>
import { STATUS, STATUS_COLOR } from '../api'
defineProps({ status: Number, text: String })
</script>

<template>
  <span class="badge" :style="{ color: STATUS_COLOR[status] || '#8a94a6', borderColor: (STATUS_COLOR[status] || '#8a94a6') + '55' }">
    {{ text || STATUS[status] || status }}
  </span>
</template>
