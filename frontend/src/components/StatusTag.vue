<script setup lang="ts">
import type { MeterStatus } from '@/api/types'
import { METER_STATUS } from '@/utils/labels'

defineProps<{ status: MeterStatus }>()
</script>

<template>
  <span class="pill" :style="{ '--c': METER_STATUS[status].color }">
    <span class="dot" aria-hidden="true" />{{ METER_STATUS[status].label }}
  </span>
</template>
