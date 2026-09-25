import { computed, ref } from 'vue'
import { api, ApiError } from '@/api/client'
import type { AnalysisRun } from '@/api/types'

/** Pasos reales del backend, en orden (analysis/analyzer.go). */
export const ANALYSIS_STEPS = [
  { key: 'loading_data', label: 'Cargando lecturas y eventos' },
  { key: 'detecting_anomalies', label: 'Detectando anomalías' },
  { key: 'explaining', label: 'Redactando explicaciones' },
  { key: 'saving_results', label: 'Guardando resultados' },
] as const

// Un análisis sobre 12 medidores tarda milisegundos: sin un ritmo mínimo el progreso se vería como un
// parpadeo. Cada paso se muestra al menos este tiempo (con un LLM lento, el ritmo real manda).
const MIN_STEP_MS = 450
const POLL_MS = 250

const running = ref(false)
const stepIndex = ref(0) // paso que se muestra ahora; ANALYSIS_STEPS.length = terminado
const lastRun = ref<AnalysisRun | null>(null)
const error = ref('')
/** Sube cada vez que termina un análisis: las vistas lo observan para volver a pedir sus datos. */
const version = ref(0)

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

function backendStep(run: AnalysisRun): number {
  if (run.status === 'COMPLETED') return ANALYSIS_STEPS.length
  const i = ANALYSIS_STEPS.findIndex((s) => s.key === run.current_step)
  return i < 0 ? 0 : i
}

async function start(): Promise<AnalysisRun | null> {
  if (running.value) return null
  running.value = true
  error.value = ''
  stepIndex.value = 0
  try {
    let run = await api.startAnalysis() // si ya hay uno en curso, el backend devuelve ese mismo
    for (;;) {
      if (run.status === 'FAILED') throw new ApiError(500, 'ANALYSIS_FAILED', run.error ?? 'El análisis falló.')
      const target = backendStep(run)
      while (stepIndex.value < target) {
        await sleep(MIN_STEP_MS)
        stepIndex.value++
      }
      if (run.status === 'COMPLETED') break
      await sleep(POLL_MS)
      run = await api.analysis(run.id)
    }
    lastRun.value = run
    version.value++
    return run
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'No se pudo completar el análisis.'
    return null
  } finally {
    running.value = false
  }
}

export function useAnalysis() {
  return {
    running,
    stepIndex,
    lastRun,
    error,
    version,
    start,
    currentLabel: computed(() => ANALYSIS_STEPS[Math.min(stepIndex.value, ANALYSIS_STEPS.length - 1)].label),
    progressPct: computed(() => Math.round((stepIndex.value / ANALYSIS_STEPS.length) * 100)),
  }
}
