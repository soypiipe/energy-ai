<script setup lang="ts">
import type { Severity } from '@/api/types'
import { SEVERITY } from '@/utils/labels'

defineProps<{ severity: Severity }>()
</script>

<template>
  <span class="pill" :style="{ '--c': SEVERITY[severity].color }">
    <span class="dot" aria-hidden="true" />{{ SEVERITY[severity].label }}
  </span>
</template>

<style>
/* Píldora compartida por SeverityTag, StatusTag y TypeTag: color = tinte del 14% + texto del color. */
.pill {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 2px 10px; border-radius: 999px;
  font-size: 12px; font-weight: 600; white-space: nowrap;
  color: var(--c);
  background: color-mix(in srgb, var(--c) 14%, transparent);
  border: 1px solid color-mix(in srgb, var(--c) 30%, transparent);
}
.pill .dot { width: 6px; height: 6px; border-radius: 50%; background: var(--c); }
</style>
