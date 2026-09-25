<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Skeleton from 'primevue/skeleton'
import Message from 'primevue/message'
import { api } from '@/api/client'
import type { Anomaly, DashboardSummary, MeterSummary } from '@/api/types'
import AnalysisProgress from '@/components/AnalysisProgress.vue'
import EChart from '@/components/EChart.vue'
import KpiCard from '@/components/KpiCard.vue'
import RunAnalysisButton from '@/components/RunAnalysisButton.vue'
import SeverityTag from '@/components/SeverityTag.vue'
import TypeTag from '@/components/TypeTag.vue'
import { useAnalysis } from '@/composables/useAnalysis'
import { axisStyle, C, tooltipStyle } from '@/utils/chart'
import { fmtAgo, fmtInt, fmtKwh, fmtLocal, fmtPct, meterTitle } from '@/utils/format'
import { METER_STATUS } from '@/utils/labels'

const router = useRouter()
const { running, version } = useAnalysis()

const summary = ref<DashboardSummary>()
const meters = ref<MeterSummary[]>([])
const anomalies = ref<Anomaly[]>([])
const loading = ref(true)
const error = ref('')

async function load() {
  error.value = ''
  try {
    const [s, m, a] = await Promise.all([api.dashboard(), api.meters(), api.anomalies()])
    summary.value = s
    meters.value = m
    anomalies.value = a.data
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'No se pudieron cargar los datos.'
  } finally {
    loading.value = false
  }
}
onMounted(load)
watch(version, load)

const top = computed(() => anomalies.value[0])
const queue = computed(() => anomalies.value.slice(0, 5))
const variationTone = computed(() => {
  const v = summary.value?.consumption.variation_pct ?? 0
  return v >= 15 ? 'warn' : undefined
})

// Variación de las últimas 24 h contra el baseline, por medidor: el desvío salta a la vista.
const chartOption = computed(() => {
  const sorted = [...meters.value].sort((a, b) => a.meter_id.localeCompare(b.meter_id))
  return {
    grid: { left: 8, right: 8, top: 24, bottom: 8, containLabel: true },
    tooltip: {
      ...tooltipStyle,
      trigger: 'axis',
      axisPointer: { type: 'shadow', shadowStyle: { color: 'rgba(255,255,255,.04)' } },
      formatter: (p: { dataIndex: number }[]) => {
        const m = sorted[p[0].dataIndex]
        return `<b>${m.meter_id}</b> · ${m.name}<br/>Últimas 24 h: ${fmtKwh(m.current_kwh)}<br/>Baseline diario: ${fmtKwh(m.baseline_kwh)}<br/>Variación: <b>${fmtPct(m.variation_pct)}</b>`
      },
    },
    xAxis: { type: 'category', data: sorted.map((m) => m.meter_id), ...axisStyle, axisLabel: { color: C.muted, fontSize: 11 } },
    yAxis: { type: 'value', ...axisStyle, axisLabel: { color: C.muted, formatter: '{value}%' } },
    series: [{
      type: 'bar',
      barMaxWidth: 28,
      data: sorted.map((m) => ({
        value: m.variation_pct,
        itemStyle: {
          color: m.status === 'OK' ? C.accent : m.status === 'ALERT' ? C.warn : C.crit,
          opacity: m.status === 'OK' ? 0.55 : 1,
          borderRadius: m.variation_pct >= 0 ? [4, 4, 0, 0] : [0, 0, 4, 4],
        },
      })),
      markLine: { silent: true, symbol: 'none', label: { show: false }, lineStyle: { color: C.muted, type: 'solid' }, data: [{ yAxis: 0 }] },
    }],
  }
})

function openMeter(p: { dataIndex: number }) {
  const sorted = [...meters.value].sort((a, b) => a.meter_id.localeCompare(b.meter_id))
  const m = sorted[p.dataIndex]
  if (m) router.push({ name: 'meter', params: { id: m.meter_id } })
}
</script>

<template>
  <div class="page">
    <Message v-if="error" severity="error" :closable="false">
      {{ error }} <Button label="Reintentar" size="small" text @click="load" />
    </Message>

    <!-- Estado 1: analizando -->
    <AnalysisProgress v-if="running" />

    <!-- Estado 2: hay algo que atender primero -->
    <section v-else-if="top" class="hero" aria-labelledby="hero-title">
      <div class="hero-main">
        <div class="eyebrow"><i class="pi pi-flag-fill" aria-hidden="true" /> Atiende primero</div>
        <h2 id="hero-title">{{ meterTitle(top.meter_id, top.meter_name) }}</h2>
        <div class="tags"><TypeTag :type="top.type" /><SeverityTag :severity="top.severity" />
          <span class="muted">Confianza {{ Math.round(top.confidence * 100) }}%</span></div>
        <p class="reason">{{ top.reason }}</p>
        <p class="action"><i class="pi pi-arrow-right-arrow-left" aria-hidden="true" /> <span><b>Acción:</b> {{ top.recommended_action }}</span></p>
      </div>
      <div class="hero-cta">
        <Button label="Investigar" icon="pi pi-search" @click="router.push({ name: 'anomaly', params: { id: top.id } })" />
        <Button label="Ver medidor" severity="secondary" outlined @click="router.push({ name: 'meter', params: { id: top.meter_id } })" />
      </div>
    </section>

    <!-- Estado 3: todavía no hay análisis -->
    <section v-else-if="!loading && !error" class="hero empty">
      <div>
        <div class="eyebrow"><i class="pi pi-sparkles" aria-hidden="true" /> Listo para analizar</div>
        <h2>Aún no hay anomalías priorizadas</h2>
        <p class="reason muted">
          Ejecuta el análisis y en segundos sabrás qué medidor atender primero, por qué y qué hacer.
        </p>
      </div>
      <RunAnalysisButton size="large" />
    </section>

    <!-- KPIs -->
    <div class="kpis" v-if="summary">
      <KpiCard label="Medidores" :value="fmtInt(summary.meters.total)" icon="pi-bolt" tone="accent">
        <span style="color: var(--ok)">● {{ summary.meters.ok }} normales</span>
        <span style="color: var(--warn)">● {{ summary.meters.alert }} alerta</span>
        <span style="color: var(--crit)">● {{ summary.meters.critical }} crítico</span>
      </KpiCard>
      <KpiCard label="Consumo últimas 24 h" :value="fmtKwh(summary.consumption.current_kwh)" icon="pi-chart-line" :tone="variationTone">
        {{ fmtPct(summary.consumption.variation_pct) }} vs. baseline ({{ fmtKwh(summary.consumption.baseline_kwh) }}/día)
      </KpiCard>
      <KpiCard label="Anomalías abiertas" :value="fmtInt(summary.anomalies.open)" icon="pi-exclamation-triangle"
        :tone="summary.anomalies.open > 0 ? 'crit' : 'ok'">
        <template v-if="summary.anomalies.total > 0">
          {{ summary.anomalies.by_severity.HIGH ?? 0 }} alta · {{ summary.anomalies.by_severity.MEDIUM ?? 0 }} media · {{ summary.anomalies.by_severity.LOW ?? 0 }} baja
        </template>
        <template v-else>Sin análisis todavía</template>
      </KpiCard>
      <KpiCard label="Último análisis" :value="summary.last_analysis ? fmtAgo(summary.last_analysis.finished_at) : '—'" icon="pi-clock" compact>
        {{ summary.last_analysis ? 'Motor estadístico + explicación IA' : 'Nunca se ha ejecutado' }}
      </KpiCard>
    </div>
    <div class="kpis" v-else-if="loading">
      <Skeleton v-for="i in 4" :key="i" height="112px" border-radius="12px" />
    </div>

    <div class="cols">
      <section class="card">
        <header class="card-head">
          <h2>Variación vs. baseline</h2>
          <span class="muted small">Últimas 24 h por medidor · clic para abrir</span>
        </header>
        <Skeleton v-if="loading" height="280px" />
        <EChart v-else :option="chartOption" :height="290" label="Variación porcentual del consumo de cada medidor frente a su baseline" @click="openMeter" />
        <ul class="legend">
          <li><i style="background: var(--accent); opacity: .55" />{{ METER_STATUS.OK.label }}</li>
          <li><i style="background: var(--warn)" />{{ METER_STATUS.ALERT.label }}</li>
          <li><i style="background: var(--crit)" />{{ METER_STATUS.CRITICAL.label }}</li>
        </ul>
      </section>

      <section class="card">
        <header class="card-head">
          <h2>Cola de atención</h2>
          <RouterLink to="/anomalies" class="small">Ver todas</RouterLink>
        </header>
        <ol v-if="queue.length" class="queue">
          <li v-for="(a, i) in queue" :key="a.id">
            <RouterLink :to="{ name: 'anomaly', params: { id: a.id } }" class="q-row">
              <span class="rank mono">{{ i + 1 }}</span>
              <span class="q-main">
                <span class="q-title"><b>{{ a.meter_id }}</b> <TypeTag :type="a.type" /></span>
                <span class="q-sub muted">Detectada {{ fmtLocal(a.detected_at) }}</span>
              </span>
              <SeverityTag :severity="a.severity" />
            </RouterLink>
          </li>
        </ol>
        <p v-else class="muted empty-q">La cola aparecerá aquí cuando se ejecute el análisis.</p>
      </section>
    </div>
  </div>
</template>

<style scoped>
.page { display: grid; gap: 20px; }
.card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); padding: 20px 24px; }
.card-head { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 8px; gap: 12px; }
.small { font-size: 12px; }

.hero {
  display: flex; gap: 24px; justify-content: space-between; align-items: center; flex-wrap: wrap;
  padding: 24px 28px; border-radius: 16px;
  background: linear-gradient(135deg, rgba(45, 212, 167, 0.10), rgba(45, 212, 167, 0.02) 60%), var(--surface);
  border: 1px solid color-mix(in srgb, var(--accent) 35%, var(--border));
}
.hero.empty { border-style: dashed; }
.hero-main { display: grid; gap: 10px; flex: 1 1 480px; min-width: 0; }
.hero h2 { font-size: 22px; }
.eyebrow { color: var(--accent); font-size: 12px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; display: flex; gap: 8px; align-items: center; }
.tags { display: flex; gap: 14px; align-items: center; flex-wrap: wrap; }
.reason { margin: 0; max-width: 78ch; line-height: 1.6; }
.action { margin: 0; display: flex; gap: 10px; color: var(--text); background: var(--bg); border: 1px solid var(--border); border-radius: 10px; padding: 10px 14px; max-width: 78ch; }
.action .pi { color: var(--accent); margin-top: 4px; }
.hero-cta { display: flex; gap: 10px; flex-wrap: wrap; }

.kpis { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px; }
.cols { display: grid; grid-template-columns: minmax(0, 3fr) minmax(0, 2fr); gap: 16px; }

.legend { list-style: none; margin: 8px 0 0; padding: 0; display: flex; gap: 16px; font-size: 12px; color: var(--muted); }
.legend i { display: inline-block; width: 10px; height: 10px; border-radius: 3px; margin-right: 6px; }

.queue { list-style: none; margin: 0; padding: 0; display: grid; gap: 6px; }
.q-row { display: flex; align-items: center; gap: 12px; padding: 10px 12px; border-radius: 10px; color: var(--text); text-decoration: none; }
.q-row:hover { background: var(--surface-2); text-decoration: none; }
.rank { width: 24px; height: 24px; border-radius: 50%; background: var(--surface-2); display: grid; place-content: center; font-size: 12px; color: var(--muted); }
.q-main { flex: 1; min-width: 0; display: grid; }
.q-title { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
.q-sub { font-size: 12px; }
.empty-q { margin: 24px 0; text-align: center; }

@media (max-width: 1100px) { .kpis { grid-template-columns: repeat(2, minmax(0, 1fr)); } .cols { grid-template-columns: 1fr; } }
@media (max-width: 560px) { .kpis { grid-template-columns: 1fr; } }
</style>
