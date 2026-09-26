// Tipos de la API del backend (docs/DESIGN.md §7). Los timestamps son hora local de planta, sin zona
// ("2026-09-12T14:00:00"), salvo created_at/finished_at de las ejecuciones, que sí son instantes UTC.

export type MeterStatus = 'OK' | 'ALERT' | 'CRITICAL'
export type Severity = 'LOW' | 'MEDIUM' | 'HIGH'
export type AnomalyType = 'REAL_ANOMALY' | 'EXPLAINABLE_ANOMALY' | 'FALSE_POSITIVE' | 'DATA_QUALITY'
export type AnomalyStatus = 'OPEN' | 'ACKNOWLEDGED' | 'RESOLVED'
export type RunStatus = 'PENDING' | 'RUNNING' | 'COMPLETED' | 'FAILED'

export interface MeterSummary {
  meter_id: string
  name: string
  location: string | null
  total_kwh: number
  current_kwh: number
  baseline_kwh: number
  variation_pct: number
  status: MeterStatus
  anomaly_type: AnomalyType | null
  severity: Severity | null
  last_reading_at: string
}

export interface MeterEvent {
  timestamp: string
  type: string
  description: string
}

export interface MeterDetail extends MeterSummary {
  events: MeterEvent[]
}

export interface Reading {
  timestamp: string
  consumption_kwh: number
  voltage_v: number
  current_a: number
  power_factor: number
  status: string
}

export interface Evidence {
  metric: string
  baseline: number
  observed: number
  change_pct: number
}

export interface FindingDoc {
  kind: 'PERSISTENT_SHIFT' | 'TRANSIENT_DEVIATION' | 'ELECTRICAL_CHANGE' | 'DATA_QUALITY'
  start: string
  end: string | null
  evidence: Evidence[]
}

export interface AnomalyEvidence {
  findings: FindingDoc[]
  related_event: { timestamp: string; type: string; description: string } | null
  explanation_source: 'llm' | 'template' | ''
}

export interface Anomaly {
  id: string
  analysis_id: string
  meter_id: string
  meter_name: string
  detected_at: string
  type: AnomalyType
  severity: Severity
  confidence: number
  priority_score: number
  reason: string
  recommended_action: string
  evidence: AnomalyEvidence
  status: AnomalyStatus
  created_at: string
}

export interface AnalysisRun {
  id: string
  status: RunStatus
  current_step: string | null
  error: string | null
  summary: { meters_analyzed: number; anomalies: number; by_type: Record<string, number>; explained_by: Record<string, number> } | null
  created_at: string
  started_at: string | null
  finished_at: string | null
}

export interface DashboardSummary {
  meters: { total: number; ok: number; alert: number; critical: number }
  consumption: { total_kwh: number; current_kwh: number; baseline_kwh: number; variation_pct: number }
  anomalies: { total: number; open: number; high_priority: number; avg_confidence: number; by_severity: Record<string, number>; by_type: Record<string, number> }
  top_priority: { anomaly_id: string; meter_id: string; type: AnomalyType; severity: Severity } | null
  last_analysis: { id: string; status: string; finished_at: string | null } | null
}

export interface LoginResponse {
  access_token: string
  token_type: string
  expires_in: number
}
