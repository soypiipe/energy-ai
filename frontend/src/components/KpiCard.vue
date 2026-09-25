<script setup lang="ts">
defineProps<{ label: string; value: string; icon: string; tone?: 'ok' | 'warn' | 'crit' | 'accent'; compact?: boolean }>()
</script>

<template>
  <article class="kpi" :class="tone">
    <div class="top"><span class="label">{{ label }}</span><i class="pi" :class="icon" aria-hidden="true" /></div>
    <div class="value mono" :class="{ compact }">{{ value }}</div>
    <div class="sub"><slot /></div>
  </article>
</template>

<style scoped>
.kpi { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); padding: 18px 20px; display: grid; gap: 6px; }
.top { display: flex; justify-content: space-between; align-items: center; color: var(--muted); }
.label { font-size: 13px; font-weight: 500; }
.value { font-size: 30px; font-weight: 700; letter-spacing: -0.02em; line-height: 1.1; }
.sub { font-size: 12px; color: var(--muted); min-height: 18px; display: flex; gap: 10px; flex-wrap: wrap; align-items: center; }
.value.compact { font-size: 20px; line-height: 1.6; }
.accent .pi { color: var(--accent); }
.ok .value { color: var(--ok); }
.warn .value { color: var(--warn); }
.crit .value { color: var(--crit); }
</style>
