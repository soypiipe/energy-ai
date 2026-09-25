// Colores de las gráficas (espejo de los tokens CSS: ECharts dibuja en canvas y no lee variables CSS).
export const C = {
  bg: '#131826',
  border: '#232B3D',
  text: '#F5F7FA',
  muted: '#8A93A6',
  accent: '#2DD4A7',
  ok: '#22C55E',
  warn: '#F5A623',
  crit: '#EF4444',
  info: '#38BDF8',
}

export const axisStyle = {
  axisLine: { lineStyle: { color: C.border } },
  axisTick: { show: false },
  axisLabel: { color: C.muted },
  splitLine: { lineStyle: { color: C.border, type: 'dashed' as const } },
}

export const tooltipStyle = {
  backgroundColor: '#1B2233',
  borderColor: C.border,
  textStyle: { color: C.text, fontSize: 12 },
  extraCssText: 'box-shadow: 0 8px 24px rgba(0,0,0,.4); border-radius: 8px;',
}
