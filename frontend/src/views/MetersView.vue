<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import InputText from 'primevue/inputtext'
import IconField from 'primevue/iconfield'
import InputIcon from 'primevue/inputicon'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Skeleton from 'primevue/skeleton'
import { api } from '@/api/client'
import type { MeterStatus, MeterSummary } from '@/api/types'
import SeverityTag from '@/components/SeverityTag.vue'
import StatusTag from '@/components/StatusTag.vue'
import TypeTag from '@/components/TypeTag.vue'
import { useAnalysis } from '@/composables/useAnalysis'
import { fmtKwh, fmtLocal, fmtPct } from '@/utils/format'

const router = useRouter()
const { version } = useAnalysis()

const meters = ref<MeterSummary[]>([])
const loading = ref(true)
const error = ref('')
const filter = ref<'ALL' | MeterStatus>('ALL')
const query = ref('')

async function load() {
  error.value = ''
  try {
    meters.value = await api.meters()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'No se pudieron cargar los medidores.'
  } finally {
    loading.value = false
  }
}
onMounted(load)
watch(version, load)

// Orden de triage por defecto: primero lo más grave.
const RANK: Record<MeterStatus, number> = { CRITICAL: 3, ALERT: 2, OK: 1 }
const rows = computed(() =>
  meters.value.map((m) => ({ ...m, rank: RANK[m.status] * 1000 + Math.min(Math.abs(m.variation_pct), 999) })),
)

const counts = computed(() => ({
  ALL: meters.value.length,
  OK: meters.value.filter((m) => m.status === 'OK').length,
  ALERT: meters.value.filter((m) => m.status === 'ALERT').length,
  CRITICAL: meters.value.filter((m) => m.status === 'CRITICAL').length,
}))

const filters = [
  { key: 'ALL', label: 'Todos' },
  { key: 'OK', label: 'Normales' },
  { key: 'ALERT', label: 'Alertas' },
  { key: 'CRITICAL', label: 'Críticos' },
] as const

const visible = computed(() => {
  const q = query.value.trim().toLowerCase()
  return rows.value.filter(
    (m) => (filter.value === 'ALL' || m.status === filter.value) && (!q || `${m.meter_id} ${m.name} ${m.location ?? ''}`.toLowerCase().includes(q)),
  )
})

const varColor = (v: number) => (Math.abs(v) >= 50 ? 'var(--crit)' : Math.abs(v) >= 15 ? 'var(--warn)' : 'var(--muted)')

function open(e: { data: MeterSummary }) {
  router.push({ name: 'meter', params: { id: e.data.meter_id } })
}
</script>

<template>
  <div class="page">
    <Message v-if="error" severity="error" :closable="false">{{ error }} <Button label="Reintentar" size="small" text @click="load" /></Message>

    <div class="toolbar">
      <div class="chips" role="group" aria-label="Filtrar por estado">
        <button v-for="f in filters" :key="f.key" type="button" class="chip" :class="{ on: filter === f.key }" :aria-pressed="filter === f.key" @click="filter = f.key">
          {{ f.label }} <span class="n mono">{{ counts[f.key] }}</span>
        </button>
      </div>
      <IconField class="search">
        <InputIcon class="pi pi-search" />
        <InputText v-model="query" placeholder="Buscar medidor…" aria-label="Buscar medidor" fluid />
      </IconField>
    </div>

    <div class="card">
      <Skeleton v-if="loading" height="420px" />
      <DataTable
        v-else
        :value="visible"
        data-key="meter_id"
        sort-field="rank"
        :sort-order="-1"
        removable-sort
        selection-mode="single"
        row-hover
        @row-click="open"
        :pt="{ bodyRow: { style: 'cursor: pointer' } }"
      >
        <template #empty><p class="muted empty">Ningún medidor coincide con el filtro.</p></template>

        <Column field="meter_id" header="Medidor" sortable>
          <template #body="{ data }">
            <RouterLink :to="{ name: 'meter', params: { id: data.meter_id } }" class="id" @click.stop><b>{{ data.meter_id }}</b></RouterLink>
          </template>
        </Column>
        <Column field="rank" header="Estado" sortable>
          <template #body="{ data }"><StatusTag :status="data.status" /></template>
        </Column>
        <Column header="Anomalía IA">
          <template #body="{ data }">
            <div v-if="data.anomaly_type" class="an"><TypeTag :type="data.anomaly_type" /><SeverityTag v-if="data.severity" :severity="data.severity" /></div>
            <span v-else class="muted">—</span>
          </template>
        </Column>
        <Column field="current_kwh" header="Últimas 24 h" sortable>
          <template #body="{ data }"><span class="mono">{{ fmtKwh(data.current_kwh) }}</span></template>
        </Column>
        <Column field="baseline_kwh" header="Baseline / día" sortable>
          <template #body="{ data }"><span class="mono muted">{{ fmtKwh(data.baseline_kwh) }}</span></template>
        </Column>
        <Column field="variation_pct" header="Variación" sortable>
          <template #body="{ data }">
            <span class="mono var" :style="{ color: varColor(data.variation_pct) }">
              <i class="pi" :class="data.variation_pct >= 0 ? 'pi-arrow-up-right' : 'pi-arrow-down-right'" aria-hidden="true" />{{ fmtPct(data.variation_pct) }}
            </span>
          </template>
        </Column>
        <Column field="last_reading_at" header="Última lectura">
          <template #body="{ data }"><span class="muted mono">{{ fmtLocal(data.last_reading_at) }}</span></template>
        </Column>
      </DataTable>
    </div>
  </div>
</template>

<style scoped>
.page { display: grid; gap: 16px; }
.toolbar { display: flex; gap: 16px; justify-content: space-between; align-items: center; flex-wrap: wrap; }
.chips { display: flex; gap: 8px; flex-wrap: wrap; }
.chip {
  display: inline-flex; gap: 8px; align-items: center; cursor: pointer;
  background: var(--surface); color: var(--muted); border: 1px solid var(--border);
  border-radius: 999px; padding: 6px 14px; font: inherit; font-weight: 500;
  transition: background .15s, color .15s, border-color .15s;
}
.chip:hover { color: var(--text); background: var(--surface-2); }
.chip.on { color: var(--accent); border-color: color-mix(in srgb, var(--accent) 50%, transparent); background: var(--accent-soft); }
.n { font-size: 12px; opacity: .85; }
.search { width: min(320px, 100%); }
.card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); padding: 4px; overflow-x: auto; }
.id { color: var(--text); }
.an { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
.var { display: inline-flex; gap: 6px; align-items: center; font-weight: 600; }
.var .pi { font-size: 12px; }
.empty { text-align: center; padding: 32px 0; }
</style>
