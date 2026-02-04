import type { ChartTooltipPayloadItem, PieTooltipPayloadItem } from '../types'

// Shared color palette for charts
export const CHART_COLORS = [
  '#00e5ff', // cyan
  '#a78bfa', // purple
  '#34d399', // green
  '#ffab00', // amber
  '#f472b6', // pink
  '#fb923c', // orange
  '#60a5fa', // blue
  '#f87171', // red
] as const

// Tooltip wrapper styling shared across all chart tooltips
const tooltipWrapper = `bg-panel dark:bg-panel-dark border border-border dark:border-border-dark
  rounded-lg px-3 py-2 shadow-lg text-sm`

interface BaseTooltipProps {
  active?: boolean
  label?: string | number
}

interface BarTooltipProps extends BaseTooltipProps {
  payload?: ChartTooltipPayloadItem[]
  labelFormatter?: (label: string | number) => string
}

export function BarChartTooltip({ active, payload, label, labelFormatter }: BarTooltipProps) {
  if (!active || !payload?.length) return null
  const formattedLabel = labelFormatter && label !== undefined ? labelFormatter(label) : label
  return (
    <div className={tooltipWrapper}>
      <div className="font-medium mb-1 text-gray-800 dark:text-gray-100">{formattedLabel}</div>
      {payload.map(item => (
        <div key={item.name} className="flex items-center gap-2 text-xs">
          <span className="w-2 h-2 rounded-full" style={{ background: item.color }} />
          <span className="text-muted dark:text-muted-dark">Plays</span>
          <span className="font-mono ml-auto">{item.value}</span>
        </div>
      ))}
    </div>
  )
}

interface LineTooltipProps extends BaseTooltipProps {
  payload?: ChartTooltipPayloadItem[]
  labelFormatter?: (label: string) => string
  filterZero?: boolean
}

export function LineChartTooltip({ active, payload, label, labelFormatter, filterZero = true }: LineTooltipProps) {
  if (!active || !payload?.length) return null
  const formattedLabel = labelFormatter && typeof label === 'string' ? labelFormatter(label) : label
  const items = filterZero ? payload.filter(item => item.value > 0) : payload
  return (
    <div className={tooltipWrapper}>
      <div className="font-medium mb-1 text-gray-800 dark:text-gray-100">{formattedLabel}</div>
      {items.map(item => (
        <div key={item.name} className="flex items-center gap-2 text-xs">
          <span className="w-2 h-2 rounded-full" style={{ background: item.color }} />
          <span className="text-muted dark:text-muted-dark">{item.name}</span>
          <span className="font-mono ml-auto">{item.value}</span>
        </div>
      ))}
    </div>
  )
}

interface AreaTooltipProps extends BaseTooltipProps {
  payload?: ChartTooltipPayloadItem[]
  labelFormatter?: (label: string) => string
}

export function AreaChartTooltip({ active, payload, label, labelFormatter }: AreaTooltipProps) {
  if (!active || !payload?.length) return null
  const formattedLabel = labelFormatter && typeof label === 'string' ? labelFormatter(label) : label
  return (
    <div className={tooltipWrapper}>
      <div className="font-medium mb-1 text-gray-800 dark:text-gray-100">{formattedLabel}</div>
      {payload.map(item => (
        <div key={item.name} className="flex items-center gap-2 text-xs">
          <span className="w-2 h-2 rounded-full" style={{ background: item.color }} />
          <span className="text-muted dark:text-muted-dark">{item.name}</span>
          <span className="font-mono ml-auto">{item.value}</span>
        </div>
      ))}
    </div>
  )
}

interface PieTooltipProps<T extends { percentage: number }> extends BaseTooltipProps {
  payload?: PieTooltipPayloadItem<T>[]
}

export function PieChartTooltip<T extends { percentage: number }>({ active, payload }: PieTooltipProps<T>) {
  if (!active || !payload?.length) return null
  const item = payload[0]
  return (
    <div className={tooltipWrapper}>
      <div className="font-medium mb-1 text-gray-800 dark:text-gray-100">{item.name}</div>
      <div className="flex items-center gap-2 text-xs">
        <span className="text-muted dark:text-muted-dark">Count</span>
        <span className="font-mono ml-auto">{item.value}</span>
      </div>
      <div className="flex items-center gap-2 text-xs">
        <span className="text-muted dark:text-muted-dark">Percentage</span>
        <span className="font-mono ml-auto">{item.payload.percentage.toFixed(1)}%</span>
      </div>
    </div>
  )
}
