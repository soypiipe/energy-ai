<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Skeleton from 'primevue/skeleton'
import { api, ApiError } from '@/api/client'
import type { Anomaly, MeterDetail, Reading } from '@/api/types'
import EChart from '@/components/EChart.vue'
import KpiCard from '@/components/KpiCard.vue'
import SeverityTag from '@/components/SeverityTag.vue'
import StatusTag from '@/components/StatusTag.vue'
import TypeTag from '@/components/TypeTag.vue'
import { useAnalysis } from '@/composables/useAnalysis'
import { fmtKwh, fmtLocal, fmtPct } from '@/utils/format'
import { EVENT_LABEL } from '@/utils/labels'
import { lineOption } from '@/utils/meterCharts'

const route = useRoute()
const router = useRouter()
const { version } = useAnalysis()

const meter = ref<MeterDetail>()
const readings = ref<Reading[]>([])
const anomaly = ref<Anomaly>()
const loading = ref(true)
const error = ref('')
const notFound = ref(false)

const id = computed(() => String(route.params.id))

async function load() {
  error.value = ''
  notFound.value = false
  try {
    const [m, r, a] = await Promise.all([api.meter(id.value), api.readings(id.value), api.anomalies({ meter_id: id.value })])
    meter.value = m
    readings.value = r
    anomaly.value = a.data[0]
  } catch (e) {
    if (e instanceof ApiError && (e.status === 404 || e.status === 400)) notFound.value = true
    else error.value = e instanceof Error ? e.message : 'No se pudo cargar el medidor.'
  } finally {
    loading.value = false
  }
}
onMounted(load)
watch(id, () => { loading.value = true; load() })
watch(version, load)

const events = computed(() => meter.value?.events ?? [])
const common = computed(() => ({ readings: readings.value, events: events.value, anomaly: anomaly.value }))

const consumption = computed(() => lineOption({ ...common.value, pick: (r) => r.consumption_kwh, name: 'Consumo', unit: 'kWh', baseline: true }))
const voltage = computed(() => lineOption({ ...common.value, pick: (r) => r.voltage_v, name: 'Voltaje', unit: 'V', height: 'small', decimals: 1 }))
const current = computed(() => lineOption({ ...common.value, pick: (r) => r.current_a, name: 'Corriente', unit: 'A', height: 'small', decimals: 1 }))
const pf = computed(() => lineOption({ ...common.value, pick: (r) => r.power_factor, name: 'Factor de potencia', unit: '', height: 'small', decimals: 3 }))

const varTone = computed(() => {
  const v = Math.abs(meter.value?.variation_pct ?? 0)
  return v >= 50 ? 'crit' : v >= 15 ? 'warn' : undefined
})
</script>

<template>
  <div class="page">
    <RouterLink to="/meters" class="back"><i class="pi pi-arrow-left" aria-hidden="true" /> Medidores</RouterLink>

    <Message v-if="notFound" severity="warn" :closable="false">Ese medidor no existe. <RouterLink to="/meters">Volver a la lista</RouterLink></Message>
    <Message v-else-if="error" severity="error" :closable="false">{{ error }} <Button label="Reintentar" size="small" text @click="load" /></Message>

    <template v-else>
      <header class="head" v-if="meter">
        <div class="title">
          <h2>{{ meter.meter_id }}</h2>
          <StatusTag :status="meter.status" />
        </div>
        <div v-if="anomaly" class="an">
          <TypeTag :type="anomaly.type" /><SeverityTag :severity="anomaly.severity" />
          <Button label="Investigar anomalía" icon="pi pi-search" size="small" @click="router.push({ name: 'anomaly', params: { id: anomaly.id } })" />
        </div>
        <span v-else class="muted">Sin anomalías en el último análisis</span>
      </header>
      <Skeleton v-else height="40px" width="260px" />

      <div class="kpis" v-if="meter">
        <KpiCard label="Últimas 24 h" :value="fmtKwh(meter.current_kwh)" icon="pi-chart-line" />
        <KpiCard label="Baseline diario" :value="fmtKwh(meter.baseline_kwh)" icon="pi-minus">Primeros 7 días de datos</KpiCard>
        <KpiCard label="Variación" :value="fmtPct(meter.variation_pct)" icon="pi-percentage" :tone="varTone">vs. su baseline</KpiCard>
        <KpiCard label="Última lectura" :value="fmtLocal(meter.last_reading_at)" icon="pi-clock" compact>Total del periodo: {{ fmtKwh(meter.total_kwh) }}</KpiCard>
      </div>
      <div class="kpis" v-else><Skeleton v-for="i in 4" :key="i" height="112px" border-radius="12px" /></div>

      <section class="card">
        <header class="card-head">
          <h2>Consumo contra baseline</h2>
          <span class="muted small">kWh por hora · <span class="mark">■</span> tramo anómalo · líneas punteadas: eventos</span>
        </header>
        <Skeleton v-if="loading" height="360px" />
        <EChart v-else :option="consumption" :height="360" label="Consumo horario del medidor comparado con su baseline, con eventos y tramo anómalo marcados" />
      </section>

      <div class="grid3">
        <section class="card"><header class="card-head"><h2>Voltaje</h2><span class="muted small">V</span></header>
          <Skeleton v-if="loading" height="200px" /><EChart v-else :option="voltage" :height="200" label="Voltaje del medidor en el tiempo" /></section>
        <section class="card"><header class="card-head"><h2>Corriente</h2><span class="muted small">A</span></header>
          <Skeleton v-if="loading" height="200px" /><EChart v-else :option="current" :height="200" label="Corriente del medidor en el tiempo" /></section>
        <section class="card"><header class="card-head"><h2>Factor de potencia</h2><span class="muted small">FP</span></header>
          <Skeleton v-if="loading" height="200px" /><EChart v-else :option="pf" :height="200" label="Factor de potencia del medidor en el tiempo" /></section>
      </div>

      <section class="card" v-if="meter">
        <header class="card-head"><h2>Eventos reportados</h2></header>
        <ul v-if="events.length" class="events">
          <li v-for="e in events" :key="e.timestamp + e.type">
            <span class="dot" :class="{ info: e.type === 'OPERATIONAL_CHANGE' || e.type === 'SCHEDULED_OUTAGE', warn: e.type === 'DATA_QUALITY' }" aria-hidden="true" />
            <div>
              <div><b>{{ EVENT_LABEL[e.type] ?? e.type }}</b> <span class="muted mono">· {{ fmtLocal(e.timestamp) }}</span></div>
              <div class="muted">{{ e.description }}</div>
            </div>
          </li>
        </ul>
        <p v-else class="muted">Este medidor no tiene eventos reportados.</p>
      </section>
    </template>
  </div>
</template>

<style scoped>
.page { display: grid; gap: 16px; }
.back { color: var(--muted); font-size: 13px; display: inline-flex; gap: 8px; align-items: center; width: fit-content; }
.back:hover { color: var(--text); text-decoration: none; }
.head { display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap; }
.title { display: flex; align-items: center; gap: 14px; }
.title h2 { font-size: 26px; }
.an { display: flex; gap: 14px; align-items: center; flex-wrap: wrap; }
.kpis { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px; }
.grid3 { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
.card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); padding: 18px 20px; }
.card-head { display: flex; justify-content: space-between; align-items: baseline; gap: 12px; margin-bottom: 6px; }
.small { font-size: 12px; }
.mark { color: var(--crit); opacity: .5; }
.events { list-style: none; margin: 8px 0 0; padding: 0; display: grid; gap: 14px; }
.events li { display: flex; gap: 12px; }
.dot { width: 10px; height: 10px; border-radius: 50%; background: var(--muted); margin-top: 6px; flex: none; }
.dot.info { background: var(--info); }
.dot.warn { background: var(--warn); }
@media (max-width: 1100px) { .kpis { grid-template-columns: repeat(2, minmax(0, 1fr)); } .grid3 { grid-template-columns: 1fr; } }
@media (max-width: 560px) { .kpis { grid-template-columns: 1fr; } }
</style>
