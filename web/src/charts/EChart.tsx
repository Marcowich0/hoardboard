import { BarChart, LineChart, PieChart, TreemapChart } from 'echarts/charts'
import type { BarSeriesOption, LineSeriesOption, PieSeriesOption, TreemapSeriesOption } from 'echarts/charts'
import { AriaComponent, GridComponent, TooltipComponent } from 'echarts/components'
import type { AriaComponentOption, GridComponentOption, TooltipComponentOption } from 'echarts/components'
import * as echarts from 'echarts/core'
import type { ComposeOption, ECElementEvent } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { useEffect, useRef } from 'react'

echarts.use([
  BarChart,
  LineChart,
  PieChart,
  TreemapChart,
  AriaComponent,
  GridComponent,
  TooltipComponent,
  CanvasRenderer,
])

export type ChartOption = ComposeOption<
  | BarSeriesOption
  | LineSeriesOption
  | PieSeriesOption
  | TreemapSeriesOption
  | AriaComponentOption
  | GridComponentOption
  | TooltipComponentOption
>

export interface EChartProps {
  option: ChartOption
  /** CSS height, e.g. 280 or "100%". */
  height: number | string
  /** What the chart shows, for screen readers. */
  label: string
  onClick?: (event: ECElementEvent) => void
  /** Called with the highlighted data index, or null when the pointer leaves. */
  onHover?: (dataIndex: number | null) => void
  className?: string
}

/**
 * One ECharts chart. It fills its width and follows the container's size;
 * a new option replaces the old one whole (pages build options from data
 * and the theme, nothing is merged).
 */
export default function EChart({ option, height, label, onClick, onHover, className }: EChartProps) {
  const element = useRef<HTMLDivElement>(null)
  const chart = useRef<echarts.ECharts | null>(null)
  const handlers = useRef({ onClick, onHover })
  useEffect(() => {
    handlers.current = { onClick, onHover }
  })

  useEffect(() => {
    const el = element.current
    if (!el) return
    const instance = echarts.init(el, null, { renderer: 'canvas' })
    chart.current = instance
    instance.on('click', (e) => handlers.current.onClick?.(e as ECElementEvent))
    instance.on('mouseover', (e) => handlers.current.onHover?.((e as ECElementEvent).dataIndex ?? null))
    instance.on('mouseout', () => handlers.current.onHover?.(null))
    instance.on('globalout', () => handlers.current.onHover?.(null))
    const observer = new ResizeObserver(() => instance.resize())
    observer.observe(el)
    return () => {
      observer.disconnect()
      instance.dispose()
      chart.current = null
    }
  }, [])

  useEffect(() => {
    chart.current?.setOption({ animationDuration: 450, animationEasing: 'cubicOut', ...option }, { notMerge: true })
  }, [option])

  return <div ref={element} role="img" aria-label={label} className={className} style={{ height, width: '100%' }} />
}
