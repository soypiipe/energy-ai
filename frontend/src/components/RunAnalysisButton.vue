<script setup lang="ts">
import Button from 'primevue/button'
import { useToast } from 'primevue/usetoast'
import { useAnalysis } from '@/composables/useAnalysis'

defineProps<{ size?: 'small' | 'large' }>()

const { running, start, currentLabel, error } = useAnalysis()
const toast = useToast()

async function run() {
  const result = await start()
  if (result) {
    const n = result.summary?.anomalies ?? 0
    toast.add({
      severity: 'success',
      summary: 'Análisis completado',
      detail: n === 1 ? 'Se encontró 1 anomalía. Ábrela para ver por qué.' : `Se encontraron ${n} anomalías, ordenadas por prioridad.`,
      life: 6000,
    })
  } else {
    toast.add({ severity: 'error', summary: 'El análisis falló', detail: error.value || 'Inténtalo de nuevo.', life: 7000 })
  }
}
</script>

<template>
  <Button
    :label="running ? currentLabel + '…' : 'Run AI Analysis'"
    :icon="running ? 'pi pi-spin pi-spinner' : 'pi pi-sparkles'"
    :size="size"
    :disabled="running"
    :aria-busy="running"
    @click="run"
  />
</template>
