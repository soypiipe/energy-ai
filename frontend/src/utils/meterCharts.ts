import type { Anomaly, MeterEvent, Reading, Severity } from '@/api/types'
import { axisStyle, C, tooltipStyle } from './chart'
import { fmtLocal } from './format'
import { EVENT_LABEL } from './labels'

const BASELINE_DAYS = 7

function median(xs: number[]): number {
  const s = [...xs].sort((a, b) => a - b)
  const n = s.length
  return n % 2 ? s[(n - 1) / 2] : (s[n / 2 - 1] + s[n / 2]) / 2
}

/** Baseline por hora del día: mediana de los primeros 7 días (igual que el motor del backend). */
export function hourlyBaseline(readings: Reading[], pick: (r: Reading) => number): number[] {
  const byHour: number[][] = Array.from({ length: 24 }, () => [])
  const start = readings[0] ? new Date(readings[0].timestamp + 'Z').getTime() : 0
  for (const r of readings) {
    const t = new Date(r.timestamp + 'Z').getTime()
    if (t - start < BASELINE_DAYS * 86400_000) byHour[Number(r.timestamp.slice(11, 13))].push(pick(r))
  }
  return byHour.map((v) => (v.length ? median(v) : NaN))
}

export const baselineSeries = (readings: Reading[], pick: (r: Reading) => number) => {
  const base = hourlyBaseline(readings, pick)
  return readings.map((r) => base[Number(r.timestamp.slice(11, 13))])
}

const SEVERITY_TINT: Record<Severity, string> = {
  HIGH: 'rgba(239, 68, 68, 0.10)',
  MEDIUM: 'rgba(245, 166, 35, 0.10)',
  LOW: 'rgba(34, 197, 94, 0.08)',
}

const EVENT_COLOR: Record<string, string> = {
  OPERATIONAL_CHANGE: C.info,
  SCHEDULED_OUTAGE: C.info,
  DATA_QUALITY: C.warn,
  UNKNOWN: C.muted,
}

interface Opts {
  readings: Reading[]
  pick: (r: Reading) => number
  name: string
  unit: string
  events?: MeterEvent[]
  anomaly?: Anomaly
  baseline?: boolean
  decimals?: number
  height?: 'big' | 'small'
  yMin?: (min: number) => number
}

/** Serie temporal del medidor, con baseline opcional, eventos como líneas verticales y el tramo anómalo sombreado. */
export function lineOption(o: Opts) {
  const times = o.readings.map((r) => r.timestamp)
  const values = o.readings.map(o.pick)
  const big = o.height !== 'small'
  const dec = o.decimals ?? 1

  // Tramo anómalo: desde el primer hallazgo hasta su fin (o el final de los datos si sigue activo).
  const findings = o.anomaly?.evidence.findings ?? []
  const start = findings.length ? findings.map((f) => f.start).sort()[0] : undefined
  const ends = findings.map((f) => f.end)
  const end = ends.length && ends.every((e) => e) ? ends.map((e) => e as string).sort().at(-1) : times.at(-1)
  const markArea = o.anomaly && start ? {
    silent: true,
    itemStyle: { color: SEVERITY_TINT[o.anomaly.severity] },
    data: [[{ xAxis: start }, { xAxis: end }]],
  } : undefined

  const markLine = o.events?.length ? {
    silent: true, symbol: 'none',
    lineStyle: { type: 'dashed' as const, width: 1.5 },
    label: { position: 'end' as const, rotate: 0, align: 'left' as const, fontSize: 11, color: C.text, backgroundColor: 'rgba(11,15,25,.85)', padding: [3, 6], borderRadius: 4 },
    data: o.events.map((e) => ({
      xAxis: e.timestamp,
      lineStyle: { color: EVENT_COLOR[e.type] ?? C.muted },
      label: { formatter: EVENT_LABEL[e.type] ?? e.type, show: big },
    })),
  } : undefined

  const series: object[] = [{
    name: o.name, type: 'line', data: values, showSymbol: false, smooth: false, sampling: 'lttb',
    lineStyle: { color: C.accent, width: big ? 2 : 1.5 },
    itemStyle: { color: C.accent },
    areaStyle: big ? { color: 'rgba(45, 212, 167, 0.08)' } : undefined,
    markArea, markLine,
    z: 3,
  }]
  if (o.baseline) {
    series.push({
      name: 'Baseline', type: 'line', data: baselineSeries(o.readings, o.pick), showSymbol: false,
      lineStyle: { color: C.muted, width: 1.5, type: 'dashed' }, itemStyle: { color: C.muted }, z: 2,
    })
  }

  return {
    animation: false,
    grid: { left: 8, right: 16, top: big ? 36 : 12, bottom: big ? 56 : 28, containLabel: true },
    legend: big ? { top: 0, right: 0, textStyle: { color: C.muted }, itemWidth: 14, itemHeight: 3 } : undefined,
    tooltip: {
      ...tooltipStyle, trigger: 'axis',
      formatter: (ps: { axisValue: string; marker: string; seriesName: string; value: number }[]) =>
        `<span style="color:${C.muted}">${fmtLocal(ps[0].axisValue)}</span><br/>` +
        ps.map((p) => `${p.marker} ${p.seriesName}: <b>${Number(p.value).toFixed(dec)} ${o.unit}</b>`).join('<br/>'),
    },
    xAxis: {
      type: 'category', data: times, boundaryGap: false, ...axisStyle, splitLine: { show: false },
      axisLabel: {
        color: C.muted, fontSize: 11, formatter: (v: string) => fmtLocal(v, false),
        // una marca por día (cada dos en las gráficas pequeñas), en la medianoche
        interval: (_i: number, v: string) => v.slice(11, 13) === '00' && (big || Number(v.slice(8, 10)) % 2 === 1),
      },
    },
    yAxis: { type: 'value', scale: true, ...axisStyle, min: o.yMin, axisLabel: { color: C.muted, fontSize: 11 } },
    dataZoom: big ? [{ type: 'slider', height: 18, bottom: 8, borderColor: C.border, backgroundColor: C.bg, fillerColor: 'rgba(45,212,167,.12)', textStyle: { color: C.muted }, handleStyle: { color: C.accent } }] : undefined,
    series,
  }
}
