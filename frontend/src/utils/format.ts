const MONTHS = ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic']

const nf = (min: number, max: number) => new Intl.NumberFormat('es-CO', { minimumFractionDigits: min, maximumFractionDigits: max })
const n0 = nf(0, 0)
const n1 = nf(1, 1)
const n2 = nf(0, 2)

export const fmtInt = (v: number) => n0.format(v)
export const fmtNum = (v: number) => n2.format(v)
export const fmtKwh = (v: number) => `${n0.format(v)} kWh`

/** +12,3% / −4,0% (con signo explícito). */
export function fmtPct(v: number, withSign = true): string {
  const s = n1.format(Math.abs(v))
  if (!withSign) return `${s}%`
  if (v > 0) return `+${s}%`
  if (v < 0) return `−${s}%`
  return `${s}%`
}

/**
 * Los timestamps de lecturas/eventos son hora local de planta sin zona ("2026-09-12T14:00:00").
 * Se formatean leyendo el texto, sin pasar por Date, para que el navegador no les aplique su zona horaria.
 */
export function fmtLocal(ts: string, withTime = true): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(ts)
  if (!m) return ts
  const date = `${Number(m[3])} ${MONTHS[Number(m[2]) - 1]}`
  return withTime ? `${date} ${m[4]}:${m[5]}` : date
}

/** Instantes reales (created_at/finished_at, UTC) → "hace 3 min". */
export function fmtAgo(iso: string | null | undefined, now = Date.now()): string {
  if (!iso) return '—'
  const secs = Math.max(0, Math.round((now - new Date(iso).getTime()) / 1000))
  if (secs < 45) return 'hace unos segundos'
  const mins = Math.round(secs / 60)
  if (mins < 60) return `hace ${mins} min`
  const hours = Math.round(mins / 60)
  if (hours < 24) return `hace ${hours} h`
  return `hace ${Math.round(hours / 24)} d`
}

/** Los medidores del dataset se llaman "Medidor M-109": no se repite el id cuando el nombre es genérico. */
export const meterTitle = (id: string, name: string) => (name === `Medidor ${id}` ? id : `${id} · ${name}`)
