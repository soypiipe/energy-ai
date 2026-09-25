import type { AnomalyStatus, AnomalyType, MeterStatus, Severity } from '@/api/types'

export const SEVERITY: Record<Severity, { label: string; color: string }> = {
  HIGH: { label: 'Alta', color: 'var(--crit)' },
  MEDIUM: { label: 'Media', color: 'var(--warn)' },
  LOW: { label: 'Baja', color: 'var(--ok)' },
}

export const METER_STATUS: Record<MeterStatus, { label: string; color: string }> = {
  OK: { label: 'Normal', color: 'var(--ok)' },
  ALERT: { label: 'Alerta', color: 'var(--warn)' },
  CRITICAL: { label: 'Crítico', color: 'var(--crit)' },
}

export const ANOMALY_TYPE: Record<AnomalyType, { label: string; hint: string; icon: string }> = {
  REAL_ANOMALY: { label: 'Anomalía real', hint: 'Nada la explica: requiere revisión', icon: 'pi-exclamation-triangle' },
  EXPLAINABLE_ANOMALY: { label: 'Explicable', hint: 'Coincide con un evento operativo reportado', icon: 'pi-info-circle' },
  FALSE_POSITIVE: { label: 'Falso positivo', hint: 'Desviación prevista (p. ej. mantenimiento)', icon: 'pi-check-circle' },
  DATA_QUALITY: { label: 'Calidad de datos', hint: 'Las lecturas no son coherentes: revisar el sensor', icon: 'pi-wave-pulse' },
}

export const ANOMALY_STATUS: Record<AnomalyStatus, string> = {
  OPEN: 'Abierta',
  ACKNOWLEDGED: 'Reconocida',
  RESOLVED: 'Resuelta',
}

export const METRIC_LABEL: Record<string, string> = {
  consumption_kwh: 'Consumo (kWh/h)',
  power_factor: 'Factor de potencia',
  current_a: 'Corriente (A)',
  voltage_v: 'Voltaje (V)',
  kwh_vi_ratio: 'Razón kWh / (V·I·FP)',
  flagged_readings: 'Lecturas incoherentes',
}

export const FINDING_LABEL: Record<string, string> = {
  PERSISTENT_SHIFT: 'Cambio de nivel sostenido',
  TRANSIENT_DEVIATION: 'Desviación temporal',
  ELECTRICAL_CHANGE: 'Cambio eléctrico',
  DATA_QUALITY: 'Lecturas incoherentes',
}

export const EVENT_LABEL: Record<string, string> = {
  OPERATIONAL_CHANGE: 'Cambio operativo',
  SCHEDULED_OUTAGE: 'Apagado programado',
  DATA_QUALITY: 'Problema de datos',
  UNKNOWN: 'Sin evento reportado',
}
