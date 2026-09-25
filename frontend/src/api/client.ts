import type {
  Anomaly, AnomalyStatus, AnalysisRun, DashboardSummary, LoginResponse, MeterDetail, MeterSummary, Reading,
} from './types'

const BASE = '/api' // vite (dev) y nginx (prod) reenvían /api al backend

const TOKEN_KEY = 'voltix.token'

// localStorage puede fallar (modo privado, datos bloqueados): la app debe seguir funcionando.
export const tokenStore = {
  get(): string | null {
    try { return localStorage.getItem(TOKEN_KEY) } catch { return null }
  },
  set(token: string) {
    try { localStorage.setItem(TOKEN_KEY, token) } catch { /* sin persistencia */ }
    memoryToken = token
  },
  clear() {
    try { localStorage.removeItem(TOKEN_KEY) } catch { /* nada */ }
    memoryToken = null
  },
}
let memoryToken: string | null = null
const currentToken = () => memoryToken ?? tokenStore.get()

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message)
  }
}

/** Se dispara cuando el backend responde 401 con una sesión activa (token vencido o inválido). */
let onUnauthorized: () => void = () => {}
export function setUnauthorizedHandler(fn: () => void) { onUnauthorized = fn }

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  const token = currentToken()
  if (token) headers.Authorization = `Bearer ${token}`

  let res: Response
  try {
    res = await fetch(BASE + path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) })
  } catch {
    throw new ApiError(0, 'NETWORK', 'No se pudo conectar con el servidor')
  }

  if (!res.ok) {
    let code = 'ERROR'
    let message = `Error ${res.status}`
    try {
      const err = await res.json()
      code = err?.error?.code ?? code
      message = err?.error?.message ?? message
    } catch { /* cuerpo no JSON */ }
    if (res.status === 401 && token && path !== '/auth/login') {
      tokenStore.clear()
      onUnauthorized()
    }
    throw new ApiError(res.status, code, message)
  }
  return (await res.json()) as T
}

export const api = {
  login: (username: string, password: string) => request<LoginResponse>('POST', '/auth/login', { username, password }),

  dashboard: () => request<DashboardSummary>('GET', '/dashboard/summary'),

  meters: async () => (await request<{ data: MeterSummary[] }>('GET', '/meters')).data,
  meter: (id: string) => request<MeterDetail>('GET', `/meters/${encodeURIComponent(id)}`),
  readings: async (id: string) =>
    (await request<{ data: Reading[] }>('GET', `/meters/${encodeURIComponent(id)}/readings`)).data,

  anomalies: async (query: Record<string, string> = {}) => {
    const qs = new URLSearchParams(query).toString()
    return request<{ analysis_id: string | null; data: Anomaly[] }>('GET', `/anomalies${qs ? `?${qs}` : ''}`)
  },
  anomaly: (id: string) => request<Anomaly>('GET', `/anomalies/${encodeURIComponent(id)}`),
  setAnomalyStatus: (id: string, status: AnomalyStatus) =>
    request<Anomaly>('PATCH', `/anomalies/${encodeURIComponent(id)}`, { status }),

  startAnalysis: () => request<AnalysisRun>('POST', '/ai/analyze'),
  analysis: (id: string) => request<AnalysisRun>('GET', `/ai/analysis/${encodeURIComponent(id)}`),
}
