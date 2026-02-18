import { useMemo } from 'react'
import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  Legend,
} from 'recharts'
import { useFetch } from '../hooks/useFetch'
import type { DayStat } from '../types'
import { CHART_COLORS, LineChartTooltip } from '../lib/chartUtils'
import { parseYMD } from '../lib/format'
import { useLocalToday } from '../hooks/useLocalToday'

interface DailyChartProps {
  days: number // 0 = all time (default to 90), 7, or 30
  startDate?: string // YYYY-MM-DD (for custom range)
  endDate?: string   // YYYY-MM-DD (for custom range)
  serverIds?: number[]
}

const seriesConfig = [
  { key: 'movies', label: 'Movies', color: CHART_COLORS[0] },
  { key: 'tv', label: 'TV', color: CHART_COLORS[1] },
  { key: 'livetv', label: 'Live TV', color: CHART_COLORS[3] },
  { key: 'music', label: 'Music', color: CHART_COLORS[2] },
  { key: 'audiobooks', label: 'Audiobooks', color: CHART_COLORS[4] },
  { key: 'books', label: 'Books', color: CHART_COLORS[5] },
] as const

const DEFAULT_ALL_TIME_RANGE = 90

function formatDateLabel(dateStr: string): string {
  const { year, month, day } = parseYMD(dateStr)
  const dt = new Date(Date.UTC(year, month, day))
  return dt.toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' })
}

function buildDateRange(days: number, today: string): { start: string; end: string } {
  const { year, month, day } = parseYMD(today)
  const start = new Date(Date.UTC(year, month, day - days + 1))
  return { start: start.toISOString().slice(0, 10), end: today }
}

export function DailyChart({ days, startDate, endDate, serverIds }: DailyChartProps) {
  const range = days === 0 ? DEFAULT_ALL_TIME_RANGE : days
  const today = useLocalToday()

  const computed = useMemo(() => buildDateRange(range, today), [range, today])
  const start = startDate || computed.start
  const end = endDate || computed.end
  const params = new URLSearchParams({ start, end })
  if (serverIds && serverIds.length > 0) {
    params.set('server_ids', serverIds.join(','))
  }
  const url = `/api/history/daily?${params.toString()}`
  const { data, loading, error } = useFetch<DayStat[]>(url)

  const hasData = data && data.some(d =>
    seriesConfig.some(s => d[s.key] > 0)
  )

  function renderChart() {
    if (loading) {
      return (
        <div className="h-[240px] flex items-center justify-center text-muted dark:text-muted-dark text-sm">
          Loading chart data...
        </div>
      )
    }
    if (error) {
      return (
        <div className="h-[240px] flex items-center justify-center text-red-500 text-sm">
          Failed to load chart data
        </div>
      )
    }
    if (!data || !hasData) {
      return (
        <div className="h-[240px] flex items-center justify-center text-muted dark:text-muted-dark text-sm">
          No play data for this period
        </div>
      )
    }
    return (
      <ResponsiveContainer width="100%" height={240}>
        <LineChart data={data} margin={{ top: 4, right: 4, bottom: 0, left: -20 }}>
          <CartesianGrid
            strokeDasharray="3 3"
            stroke="currentColor"
            className="text-border dark:text-border-dark"
            opacity={0.5}
          />
          <XAxis
            dataKey="date"
            tickFormatter={formatDateLabel}
            tick={{ fontSize: 11, fill: 'currentColor' }}
            className="text-muted dark:text-muted-dark"
            tickLine={false}
            axisLine={false}
            interval="preserveStartEnd"
          />
          <YAxis
            allowDecimals={false}
            tick={{ fontSize: 11, fill: 'currentColor' }}
            className="text-muted dark:text-muted-dark"
            tickLine={false}
            axisLine={false}
          />
          <Tooltip content={<LineChartTooltip labelFormatter={formatDateLabel} />} />
          <Legend
            iconType="circle"
            iconSize={8}
            wrapperStyle={{ fontSize: 12, paddingTop: 8 }}
          />
          {seriesConfig.map(s => (
            <Line
              key={s.key}
              type="monotone"
              dataKey={s.key}
              name={s.label}
              stroke={s.color}
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 4, strokeWidth: 0 }}
            />
          ))}
        </LineChart>
      </ResponsiveContainer>
    )
  }

  return (
    <div className="card p-4 md:p-6">
      <h2 className="text-sm font-semibold uppercase tracking-wider text-muted dark:text-muted-dark mb-4">
        Daily Plays
      </h2>
      {renderChart()}
    </div>
  )
}
