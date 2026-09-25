<script setup lang="ts">
defineProps<{ size?: number }>()
</script>

<template>
  <span class="brand" :style="{ '--s': (size ?? 32) + 'px' }">
    <svg class="mark" viewBox="0 0 32 32" aria-hidden="true">
      <rect width="32" height="32" rx="8" fill="var(--surface-2)" />
      <path d="M18 4 8 18h7l-1 10 10-14h-7z" fill="var(--accent)" />
    </svg>
    <span class="word">Voltix</span>
  </span>
</template>

<style scoped>
.brand { display: inline-flex; align-items: center; gap: 10px; }
.mark { width: var(--s); height: var(--s); }
.word { font-size: calc(var(--s) * 0.62); font-weight: 700; letter-spacing: -0.02em; }
</style>
