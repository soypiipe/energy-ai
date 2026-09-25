<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Skeleton from 'primevue/skeleton'
import { useToast } from 'primevue/usetoast'
import { api, ApiError } from '@/api/client'
import type { Anomaly, AnomalyStatus, Evidence, Reading } from '@/api/types'
import EChart from '@/components/EChart.vue'
import SeverityTag from '@/components/SeverityTag.vue'
import TypeTag from '@/components/TypeTag.vue'
import { fmtLocal, fmtNum, fmtPct } from '@/utils/format'
import { ANOMALY_STATUS, ANOMALY_TYPE, EVENT_LABEL, FINDING_LABEL, METRIC_LABEL } from '@/utils/labels'
import { lineOption } from '@/utils/meterCharts'

const route = useRoute()
const toast = useToast()

const anomaly = ref<Anomaly>()
const readings = ref<Reading[]>([])
const events = ref<{ timestamp: string; type: string; description: string }[]>([])
const loading = ref(true)
const error = ref('')
const notFound = ref(false)
const saving = ref(false)

async function load() {
  error.value = ''
  notFound.value = false
  try {
    const a = await api.anomaly(String(route.params.id))
    anomaly.value = a
    const [r, m] = await Promise.all([api.readings(a.meter_id), api.meter(a.meter_id)])
    readings.value = r
    events.value = m.events
  } catch (e) {
    if (e instanceof ApiError && (e.status === 404 || e.status === 400)) notFound.value = true
    else error.value = e instanceof Error ? e.message : 'No se pudo cargar la anomalía.'
  } finally {
    loading.value = false
  }
}
onMounted(load)
watch(() => route.params.id, () => { loading.value = true; load() })

async function setStatus(status: AnomalyStatus) {
  if (!anomaly.value || saving.value) return
  saving.value = true
  try {
    anomaly.value = await api.setAnomalyStatus(anomaly.value.id, status)
    toast.add({ severity: 'success', summary: `Marcada como ${ANOMALY_STATUS[status].toLowerCase()}`, life: 3000 })
  } catch (e) {
    toast.add({ severity: 'error', summary: 'No se pudo actualizar', detail: e instanceof Error ? e.message : '', life: 5000 })
  } finally {
    saving.value = false
  }
}

const findings = computed(() => anomaly.value?.evidence.findings ?? [])
const related = computed(() => anomaly.value?.evidence.related_event ?? null)
const explains = computed(() => related.value?.type === 'OPERATIONAL_CHANGE' || related.value?.type === 'SCHEDULED_OUTAGE')
const chart = computed(() =>
  lineOption({ readings: readings.value, pick: (r) => r.consumption_kwh, name: 'Consumo', unit: 'kWh', baseline: true, events: events.value, anomaly: anomaly.value }),
)

// Un evento que "explica" (cambio operativo, apagado programado) vs. uno que no (sin evento / calidad de datos).
const verdict = computed(() => {
  if (!related.value) return { tone: 'warn', title: 'Ningún evento cerca', text: 'No hay eventos reportados a ±2 h del inicio: nada justifica este cambio.' }
  if (explains.value) return { tone: 'ok', title: 'Un evento lo explica', text: 'Coincide con un evento operativo reportado a ±2 h del inicio.' }
  if (related.value.type === 'UNKNOWN') return { tone: 'crit', title: 'Evento sin causa', text: 'El único evento cercano no reporta ninguna causa operativa: no explica el cambio.' }
  return { tone: 'warn', title: 'El evento no explica el cambio', text: 'El evento cercano habla de los datos del sensor, no de un cambio real de operación.' }
})

function fmtEv(e: Evidence, v: number) {
  return e.metric === 'power_factor' ? v.toFixed(3).replace('.', ',') : fmtNum(v)
}

const changeColor = (pct: number) => (Math.abs(pct) >= 50 ? 'var(--crit)' : Math.abs(pct) >= 15 ? 'var(--warn)' : 'var(--muted)')
// Baseline (gris) y observado (turquesa) a la misma escala: la diferencia se ve a ojo.
const barWidth = (e: Evidence) => {
  const max = Math.max(e.baseline, e.observed) || 1
  return { base: (e.baseline / max) * 100, obs: (e.observed / max) * 100 }
}
const period = (f: { start: string; end: string | null }) => (f.end ? `${fmtLocal(f.start)} → ${fmtLocal(f.end)}` : `Desde ${fmtLocal(f.start)} · sigue activo`)
</script>

<template>
  <div class="page">
    <RouterLink to="/anomalies" class="back"><i class="pi pi-arrow-left" aria-hidden="true" /> Anomalías IA</RouterLink>

    <Message v-if="notFound" severity="warn" :closable="false">Esa anomalía no existe. <RouterLink to="/anomalies">Volver a la lista</RouterLink></Message>
    <Message v-else-if="error" severity="error" :closable="false">{{ error }} <Button label="Reintentar" size="small" text @click="load" /></Message>
    <div v-else-if="loading" class="skeletons"><Skeleton height="72px" /><Skeleton height="180px" /><Skeleton height="320px" /></div>

    <template v-else-if="anomaly">
      <header class="head">
        <div>
          <div class="eyebrow">Investigación</div>
          <h2><RouterLink :to="{ name: 'meter', params: { id: anomaly.meter_id } }" class="title">{{ anomaly.meter_id }}</RouterLink></h2>
          <div class="tags"><TypeTag :type="anomaly.type" /><SeverityTag :severity="anomaly.severity" />
            <span class="st" :class="anomaly.status.toLowerCase()">{{ ANOMALY_STATUS[anomaly.status] }}</span></div>
        </div>
        <div class="actions">
          <Button v-if="anomaly.status === 'OPEN'" label="Reconocer" icon="pi pi-eye" severity="secondary" outlined :loading="saving" @click="setStatus('ACKNOWLEDGED')" />
          <Button v-if="anomaly.status !== 'RESOLVED'" label="Marcar resuelta" icon="pi pi-check" :loading="saving" @click="setStatus('RESOLVED')" />
          <Button v-if="anomaly.status !== 'OPEN'" label="Reabrir" icon="pi pi-refresh" severity="secondary" text :loading="saving" @click="setStatus('OPEN')" />
        </div>
      </header>

      <div class="layout">
        <div class="col">
          <!-- 1. Por qué + qué hacer -->
          <section class="card explain" aria-labelledby="why">
            <header class="card-head">
              <h2 id="why"><i class="pi pi-sparkles" aria-hidden="true" /> Explicación</h2>
              <span class="src muted" :title="anomaly.evidence.explanation_source === 'llm' ? 'Redactada por un modelo de lenguaje a partir de la evidencia' : 'Redactada con una plantilla determinista a partir de la evidencia'">
                {{ anomaly.evidence.explanation_source === 'llm' ? 'Redactada por IA' : 'Plantilla determinista' }}
              </span>
            </header>
            <p class="reason">{{ anomaly.reason }}</p>
            <div class="action">
              <div class="action-label"><i class="pi pi-arrow-right-arrow-left" aria-hidden="true" /> Acción recomendada</div>
              <p>{{ anomaly.recommended_action }}</p>
            </div>
            <p class="note muted">La IA solo redacta: el tipo, la severidad y las cifras los calcula el motor estadístico, así que puedes auditarlos abajo.</p>
          </section>

          <!-- 2. Comparación con el baseline -->
          <section class="card">
            <header class="card-head"><h2>Comparación con el baseline</h2><span class="muted small">Consumo por hora · tramo anómalo sombreado</span></header>
            <EChart :option="chart" :height="340" label="Consumo del medidor frente a su baseline, con el tramo anómalo y los eventos marcados" />
          </section>

          <!-- 3. Evidencia -->
          <section class="card">
            <header class="card-head"><h2>Evidencia</h2><span class="muted small">Lo que detectó el motor</span></header>
            <div v-for="(f, i) in findings" :key="i" class="finding">
              <div class="f-head"><b>{{ FINDING_LABEL[f.kind] ?? f.kind }}</b><span class="muted mono">{{ period(f) }}</span></div>
              <table class="ev">
                <thead><tr><th scope="col">Métrica</th><th scope="col" class="num">Baseline</th><th scope="col" class="num">Observado</th><th scope="col" class="num">Cambio</th><th scope="col" class="viz"><span class="sr">Comparación</span></th></tr></thead>
                <tbody>
                  <tr v-for="e in f.evidence" :key="e.metric">
                    <td>{{ METRIC_LABEL[e.metric] ?? e.metric }}</td>
                    <template v-if="e.metric === 'flagged_readings'">
                      <td class="num muted">0</td><td class="num mono">{{ fmtNum(e.observed) }}</td><td class="num muted">—</td><td class="viz" />
                    </template>
                    <template v-else>
                      <td class="num mono muted">{{ fmtEv(e, e.baseline) }}</td>
                      <td class="num mono">{{ fmtEv(e, e.observed) }}</td>
                      <td class="num mono chg" :style="{ color: changeColor(e.change_pct) }">{{ fmtPct(e.change_pct) }}</td>
                      <td class="viz"><span class="track thin"><i class="b base" :style="{ width: barWidth(e).base + '%' }" /></span><span class="track thin"><i class="b" :style="{ width: barWidth(e).obs + '%' }" /></span></td>
                    </template>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>

        <aside class="col side">
          <!-- Evento relacionado -->
          <section class="card" :class="'v-' + verdict.tone">
            <header class="card-head"><h2>Eventos</h2></header>
            <div class="verdict"><i class="pi" :class="verdict.tone === 'ok' ? 'pi-check-circle' : verdict.tone === 'crit' ? 'pi-times-circle' : 'pi-exclamation-circle'" aria-hidden="true" /><b>{{ verdict.title }}</b></div>
            <p class="muted">{{ verdict.text }}</p>
            <div v-if="related" class="ev-box">
              <div><b>{{ EVENT_LABEL[related.type] ?? related.type }}</b></div>
              <div class="muted mono">{{ fmtLocal(related.timestamp) }}</div>
              <div class="muted">{{ related.description }}</div>
            </div>
          </section>

          <!-- Confianza y prioridad -->
          <section class="card">
            <header class="card-head"><h2>Confianza y prioridad</h2></header>
            <div class="meter-row">
              <div class="mr-top"><span>Confianza</span><b class="mono">{{ Math.round(anomaly.confidence * 100) }}%</b></div>
              <span class="track"><i class="b" :style="{ width: anomaly.confidence * 100 + '%' }" /></span>
            </div>
            <div class="kv"><span class="muted">Puntaje de prioridad</span><b class="mono">{{ anomaly.priority_score.toFixed(2) }}</b></div>
            <div class="kv"><span class="muted">Detectada</span><span class="mono">{{ fmtLocal(anomaly.detected_at) }}</span></div>
            <p class="note muted">{{ ANOMALY_TYPE[anomaly.type].hint }}. Prioridad = severidad × magnitud × confianza.</p>
          </section>
        </aside>
      </div>
    </template>
  </div>
</template>

<style scoped>
.page { display: grid; gap: 16px; }
.back { color: var(--muted); font-size: 13px; display: inline-flex; gap: 8px; align-items: center; width: fit-content; }
.back:hover { color: var(--text); text-decoration: none; }
.skeletons { display: grid; gap: 16px; }
.head { display: flex; justify-content: space-between; gap: 16px; flex-wrap: wrap; align-items: flex-end; }
.eyebrow { color: var(--accent); font-size: 12px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
.head h2 { font-size: 30px; margin: 2px 0 8px; }
.title { color: var(--text); }
.tags { display: flex; gap: 14px; align-items: center; flex-wrap: wrap; }
.actions { display: flex; gap: 10px; flex-wrap: wrap; }
.st { font-size: 12px; font-weight: 600; padding: 2px 8px; border-radius: 6px; background: var(--surface-2); }
.st.acknowledged { color: var(--info); }
.st.resolved { color: var(--ok); }

.layout { display: grid; grid-template-columns: minmax(0, 2fr) minmax(280px, 1fr); gap: 16px; align-items: start; }
.col { display: grid; gap: 16px; min-width: 0; }
.card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); padding: 18px 22px; }
.card-head { display: flex; justify-content: space-between; align-items: baseline; gap: 12px; margin-bottom: 8px; }
.card-head h2 { display: flex; gap: 8px; align-items: center; }
.small { font-size: 12px; }

.explain { border-color: color-mix(in srgb, var(--accent) 35%, var(--border)); background: linear-gradient(135deg, rgba(45,212,167,.07), transparent 60%), var(--surface); }
.explain .pi-sparkles { color: var(--accent); }
.src { font-size: 12px; }
.reason { font-size: 15px; line-height: 1.65; margin: 4px 0 14px; }
.action { background: var(--bg); border: 1px solid var(--border); border-radius: 10px; padding: 12px 16px; }
.action-label { color: var(--accent); font-size: 12px; font-weight: 700; letter-spacing: .06em; text-transform: uppercase; display: flex; gap: 8px; align-items: center; }
.action p { margin: 6px 0 0; font-size: 15px; line-height: 1.55; }
.note { font-size: 12px; margin: 12px 0 0; }

.finding + .finding { margin-top: 18px; padding-top: 16px; border-top: 1px solid var(--border); }
.f-head { display: flex; justify-content: space-between; gap: 12px; flex-wrap: wrap; margin-bottom: 8px; }
.ev { width: 100%; border-collapse: collapse; }
.ev th { text-align: left; font-weight: 500; color: var(--muted); font-size: 12px; padding: 6px 8px; border-bottom: 1px solid var(--border); }
.ev td { padding: 9px 8px; border-bottom: 1px solid color-mix(in srgb, var(--border) 60%, transparent); }
.ev .num { text-align: right; white-space: nowrap; }
.ev .chg { font-weight: 700; }
.ev .viz { width: 22%; min-width: 80px; }
.sr { position: absolute; left: -9999px; }
.track { display: block; height: 6px; background: var(--surface-2); border-radius: 6px; overflow: hidden; }
.track.thin { height: 5px; margin: 2px 0; }
.track .b.base { background: var(--muted); opacity: .6; }
.track .b { display: block; height: 100%; background: var(--accent); border-radius: 6px; }

.side { position: sticky; top: 84px; }
.verdict { display: flex; gap: 10px; align-items: center; margin: 4px 0; }
.v-ok .verdict { color: var(--ok); }
.v-warn .verdict { color: var(--warn); }
.v-crit .verdict { color: var(--crit); }
.v-crit { border-color: color-mix(in srgb, var(--crit) 35%, var(--border)); }
.card p { margin: 6px 0; }
.ev-box { margin-top: 10px; padding: 10px 14px; background: var(--bg); border: 1px solid var(--border); border-radius: 10px; display: grid; gap: 2px; }
.meter-row { margin: 6px 0 14px; display: grid; gap: 6px; }
.mr-top { display: flex; justify-content: space-between; }
.kv { display: flex; justify-content: space-between; padding: 8px 0; border-top: 1px solid var(--border); }

@media (max-width: 1000px) { .layout { grid-template-columns: 1fr; } .side { position: static; } }
</style>
