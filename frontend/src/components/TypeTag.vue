<script setup lang="ts">
import type { AnomalyType } from '@/api/types'
import { ANOMALY_TYPE } from '@/utils/labels'

defineProps<{ type: AnomalyType }>()
</script>

<template>
  <span class="type" :title="ANOMALY_TYPE[type].hint">
    <i class="pi" :class="ANOMALY_TYPE[type].icon" aria-hidden="true" />{{ ANOMALY_TYPE[type].label }}
  </span>
</template>

<style scoped>
.type { display: inline-flex; align-items: center; gap: 6px; font-size: 13px; color: var(--text); white-space: nowrap; }
.pi { color: var(--muted); font-size: 13px; }
</style>
