<script setup>
const props = defineProps({ total: Number, page: Number, size: Number })
const emit = defineEmits(['go'])
const pages = () => Math.max(1, Math.ceil((props.total || 0) / (props.size || 20)))
</script>

<template>
  <div class="pager" v-if="total > size">
    <button class="btn btn--sm" :disabled="page <= 1" @click="emit('go', page - 1)">←</button>
    <span>{{ page }} / {{ pages() }}（共 {{ total }}）</span>
    <button class="btn btn--sm" :disabled="page >= pages()" @click="emit('go', page + 1)">→</button>
  </div>
</template>
