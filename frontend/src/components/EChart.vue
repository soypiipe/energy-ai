<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts/core'
import { BarChart, LineChart } from 'echarts/charts'
import {
  DataZoomComponent, GridComponent, LegendComponent, MarkAreaComponent, MarkLineComponent, TooltipComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { EChartsCoreOption } from 'echarts/core'

echarts.use([
  BarChart, LineChart, GridComponent, TooltipComponent, LegendComponent, MarkAreaComponent, MarkLineComponent,
  DataZoomComponent, CanvasRenderer,
])

const props = defineProps<{ option: EChartsCoreOption; height?: number; label: string }>()
const emit = defineEmits<{ (e: 'click', params: { name: string; dataIndex: number }): void }>()

const el = ref<HTMLDivElement>()
let chart: echarts.ECharts | undefined
let observer: ResizeObserver | undefined

onMounted(() => {
  if (!el.value) return
  chart = echarts.init(el.value, undefined, { renderer: 'canvas' })
  chart.setOption(props.option)
  chart.on('click', (p) => emit('click', { name: String(p.name), dataIndex: p.dataIndex ?? -1 }))
  observer = new ResizeObserver(() => chart?.resize())
  observer.observe(el.value)
})

watch(() => props.option, (opt) => chart?.setOption(opt, true), { deep: true })

onBeforeUnmount(() => {
  observer?.disconnect()
  chart?.dispose()
})
</script>

<template>
  <div ref="el" role="img" :aria-label="label" :style="{ height: (height ?? 280) + 'px', width: '100%' }" />
</template>
