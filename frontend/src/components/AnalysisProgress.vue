<script setup lang="ts">
import ProgressBar from 'primevue/progressbar'
import { ANALYSIS_STEPS, useAnalysis } from '@/composables/useAnalysis'

const { stepIndex, progressPct } = useAnalysis()
</script>

<template>
  <section class="card progress" aria-live="polite" aria-label="Progreso del análisis">
    <div class="head">
      <i class="pi pi-sparkles" aria-hidden="true" />
      <h2>Analizando los 12 medidores…</h2>
    </div>
    <ProgressBar :value="progressPct" :show-value="false" style="height: 6px" />
    <ol>
      <li v-for="(s, i) in ANALYSIS_STEPS" :key="s.key" :class="{ done: i < stepIndex, active: i === stepIndex }">
        <i class="pi" :class="i < stepIndex ? 'pi-check-circle' : i === stepIndex ? 'pi-spin pi-spinner' : 'pi-circle'" aria-hidden="true" />
        {{ s.label }}
      </li>
    </ol>
  </section>
</template>

<style scoped>
.card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); padding: 20px 24px; }
.head { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; }
.head .pi { color: var(--accent); }
ol { list-style: none; margin: 16px 0 0; padding: 0; display: grid; gap: 8px; grid-template-columns: repeat(auto-fit, minmax(210px, 1fr)); }
li { display: flex; align-items: center; gap: 8px; color: var(--muted); }
li.done { color: var(--text); }
li.done .pi { color: var(--ok); }
li.active { color: var(--accent); }
</style>
