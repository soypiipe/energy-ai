<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Skeleton from 'primevue/skeleton'
import { api } from '@/api/client'
import type { Anomaly, Severity } from '@/api/types'
import RunAnalysisButton from '@/components/RunAnalysisButton.vue'
import SeverityTag from '@/components/SeverityTag.vue'
import TypeTag from '@/components/TypeTag.vue'
import { useAnalysis } from '@/composables/useAnalysis'
import { fmtLocal } from '@/utils/format'
import { ANOMALY_STATUS } from '@/utils/labels'

const router = useRouter()
const { version } = useAnalysis()

const anomalies = ref<Anomaly[]>([])
const loading = ref(true)
const error = ref('')
const severity = ref<'ALL' | Severity>('ALL')
const onlyOpen = ref(false)

async function load() {
  error.value = ''
  try {
    anomalies.value = (await api.anomalies()).data
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'No se pudieron cargar las anomalías.'
  } finally {
    loading.value = false
  }
}
onMounted(load)
watch(version, load)

const counts = computed(() => ({
  ALL: anomalies.value.length,
  HIGH: anomalies.value.filter((a) => a.severity === 'HIGH').length,
  MEDIUM: anomalies.value.filter((a) => a.severity === 'MEDIUM').length,
  LOW: anomalies.value.filter((a) => a.severity === 'LOW').length,
}))
const filters = [
  { key: 'ALL', label: 'Todas' },
  { key: 'HIGH', label: 'Alta' },
  { key: 'MEDIUM', label: 'Media' },
  { key: 'LOW', label: 'Baja' },
] as const

const visible = computed(() =>
  anomalies.value.filter((a) => (severity.value === 'ALL' || a.severity === severity.value) && (!onlyOpen.value || a.status === 'OPEN')),
)
const maxPriority = computed(() => Math.max(1, ...anomalies.value.map((a) => a.priority_score)))

function open(e: { data: Anomaly }) {
  router.push({ name: 'anomaly', params: { id: e.data.id } })
}
</script>

<template>
  <div class="page">
    <Message v-if="error" severity="error" :closable="false">{{ error }} <Button label="Reintentar" size="small" text @click="load" /></Message>

    <section v-if="!loading && !error && anomalies.length === 0" class="empty">
      <i class="pi pi-sparkles" aria-hidden="true" />
      <h2>Todavía no hay anomalías</h2>
      <p class="muted">Ejecuta el análisis para que la IA priorice qué medidor atender primero.</p>
      <RunAnalysisButton size="large" />
    </section>

    <template v-else>
      <div class="toolbar">
        <div class="chips" role="group" aria-label="Filtrar por severidad">
          <button v-for="f in filters" :key="f.key" type="button" class="chip" :class="{ on: severity === f.key }" :aria-pressed="severity === f.key" @click="severity = f.key">
            {{ f.label }} <span class="n mono">{{ counts[f.key] }}</span>
          </button>
        </div>
        <label class="toggle"><input v-model="onlyOpen" type="checkbox" /> Solo abiertas</label>
      </div>

      <div class="card">
        <Skeleton v-if="loading" height="320px" />
        <DataTable v-else :value="visible" data-key="id" selection-mode="single" row-hover @row-click="open" :pt="{ bodyRow: { style: 'cursor: pointer' } }">
          <template #empty><p class="muted empty-row">Ninguna anomalía coincide con el filtro.</p></template>
          <Column header="Prioridad" style="min-width: 130px">
            <template #body="{ data }">
              <div class="prio" :title="`Puntaje de prioridad ${data.priority_score}`">
                <span class="mono">{{ data.priority_score.toFixed(2) }}</span>
                <span class="bar"><i :style="{ width: (data.priority_score / maxPriority) * 100 + '%' }" /></span>
              </div>
            </template>
          </Column>
          <Column header="Medidor">
            <template #body="{ data }"><RouterLink :to="{ name: 'meter', params: { id: data.meter_id } }" @click.stop><b>{{ data.meter_id }}</b></RouterLink></template>
          </Column>
          <Column header="Tipo"><template #body="{ data }"><TypeTag :type="data.type" /></template></Column>
          <Column header="Severidad"><template #body="{ data }"><SeverityTag :severity="data.severity" /></template></Column>
          <Column header="Confianza"><template #body="{ data }"><span class="mono">{{ Math.round(data.confidence * 100) }}%</span></template></Column>
          <Column header="Detectada"><template #body="{ data }"><span class="mono muted nowrap">{{ fmtLocal(data.detected_at) }}</span></template></Column>
          <Column header="Estado">
            <template #body="{ data }"><span class="st" :class="data.status.toLowerCase()">{{ ANOMALY_STATUS[data.status as keyof typeof ANOMALY_STATUS] }}</span></template>
          </Column>
          <Column header="Acción" style="min-width: 260px">
            <template #body="{ data }"><span class="why">{{ data.recommended_action }}</span></template>
          </Column>
          <Column header="Por qué" style="min-width: 260px">
            <template #body="{ data }"><span class="why muted">{{ data.reason }}</span></template>
          </Column>
        </DataTable>
      </div>
    </template>
  </div>
</template>

<style scoped>
.page { display: grid; gap: 16px; }
.toolbar { display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap; }
.chips { display: flex; gap: 8px; flex-wrap: wrap; }
.chip {
  display: inline-flex; gap: 8px; align-items: center; cursor: pointer;
  background: var(--surface); color: var(--muted); border: 1px solid var(--border);
  border-radius: 999px; padding: 6px 14px; font: inherit; font-weight: 500;
}
.chip:hover { color: var(--text); background: var(--surface-2); }
.chip.on { color: var(--accent); border-color: color-mix(in srgb, var(--accent) 50%, transparent); background: var(--accent-soft); }
.n { font-size: 12px; opacity: .85; }
.toggle { display: inline-flex; gap: 8px; align-items: center; color: var(--muted); cursor: pointer; }
.toggle input { accent-color: var(--accent); width: 16px; height: 16px; }
.card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); padding: 4px; overflow-x: auto; }
.prio { display: grid; gap: 4px; font-weight: 700; }
.bar { height: 4px; border-radius: 4px; background: var(--surface-2); overflow: hidden; }
.bar i { display: block; height: 100%; background: var(--accent); border-radius: 4px; }
.st { font-size: 12px; font-weight: 600; padding: 2px 8px; border-radius: 6px; background: var(--surface-2); color: var(--text); }
.st.acknowledged { color: var(--info); }
.st.resolved { color: var(--ok); }
.why { display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; max-width: 420px; }
.empty { display: grid; justify-items: center; gap: 10px; text-align: center; padding: 64px 16px; border: 1px dashed var(--border); border-radius: 16px; background: var(--surface); }
.empty .pi { font-size: 32px; color: var(--accent); }
.empty p { margin: 0 0 8px; }
.nowrap { white-space: nowrap; }
.empty-row { text-align: center; padding: 28px 0; }
</style>
