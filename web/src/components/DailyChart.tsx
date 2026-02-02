import { useState, useMemo, useEffect } from 'react'
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

type Range = 7 | 30 | 90

const ranges: { value: Range; label: string }[] = [
  { value: 7, label: '7d' },
  { value: 30, label: '30d' },
  { value: 90, label: '90d' },
]

const seriesConfig = [
  { key: 'movies', label: 'Movies', color: '#00e5ff' },
  { key: 'tv', label: 'TV', color: '#a78bfa' },
  { key: 'livetv', label: 'Live TV', color: '#ffab00' },
  { key: 'music', label: 'Music', color: '#34d399' },
  { key: 'audiobooks', label: 'Audiobooks', color: '#f472b6' },
  { key: 'books', label: 'Books', color: '#fb923c' },
] as const

function formatDateLabel(dateStr: string): string {
  const [y, m, d] = dateStr.split('-').map(Number)
  const dt = new Date(Date.UTC(y, m - 1, d))
  return dt.toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' })
}

function todayUTC(): string {
  return new Date().toISOString().slice(0, 10)
}

function buildDateRange(days: Range, today: string): { start: string; end: string } {
  const [y, m, d] = today.split('-').map(Number)
  const endExclusive = new Date(Date.UTC(y, m - 1, d + 1))
  const start = new Date(Date.UTC(y, m - 1, d - days + 1))
  const fmt = (dt: Date) => dt.toISOString().slice(0, 10)
  return { start: fmt(start), end: fmt(endExclusive) }
}

interface TooltipPayloadItem {
  color: string
  name: string
  value: number
}

function ChartTooltip({ active, payload, label }: {
  active?: boolean
  payload?: TooltipPayloadItem[]
  label?: string
}) {
  if (!active || !payload?.length) return null
  return (
    <div className="bg-panel dark:bg-panel-dark border border-border dark:border-border-dark
                    rounded-lg px-3 py-2 shadow-lg text-sm">
      <div className="font-medium mb-1 text-gray-800 dark:text-gray-100">
        {label && formatDateLabel(label)}
      </div>
      {payload.filter(item => item.value > 0).map(item => (
        <div key={item.name} className="flex items-center gap-2 text-xs">
          <span className="w-2 h-2 rounded-full" style={{ background: item.color }} />
          <span className="text-muted dark:text-muted-dark">{item.name}</span>
          <span className="font-mono ml-auto">{item.value}</span>
        </div>
      ))}
    </div>
  )
}

export function DailyChart() {
  const [range, setRange] = useState<Range>(30)
  const [today, setToday] = useState(todayUTC)

  useEffect(() => {
    const interval = setInterval(() => {
      const now = todayUTC()
      if (now !== today) setToday(now)
    }, 60_000)
    return () => clearInterval(interval)
  }, [today])

  const { start, end } = useMemo(() => buildDateRange(range, today), [range, today])
  const url = `/api/history/daily?start=${start}&end=${end}`
  const { data, loading, error } = useFetch<DayStat[]>(url)

  const hasData = data && data.some(d =>
    seriesConfig.some(s => d[s.key] > 0)
  )

  return (
    <div className="card p-4 md:p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted dark:text-muted-dark">
          Daily Plays
        </h2>
        <div className="flex gap-1">
          {ranges.map(r => (
            <button
              key={r.value}
              onClick={() => setRange(r.value)}
              className={`px-2.5 py-1 rounded text-xs font-mono font-medium transition-colors
                ${range === r.value
                  ? 'bg-accent/15 text-accent-dim dark:text-accent'
                  : 'text-muted dark:text-muted-dark hover:text-gray-800 dark:hover:text-gray-200'
                }`}
            >
              {r.label}
            </button>
          ))}
        </div>
      </div>

      {loading && (
        <div className="h-[240px] flex items-center justify-center text-muted dark:text-muted-dark text-sm">
          Loading chart data...
        </div>
      )}

      {error && (
        <div className="h-[240px] flex items-center justify-center text-red-500 text-sm">
          Failed to load chart data
        </div>
      )}

      {data && !hasData && (
        <div className="h-[240px] flex items-center justify-center text-muted dark:text-muted-dark text-sm">
          No play data for this period
        </div>
      )}

      {data && hasData && (
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
            <Tooltip content={<ChartTooltip />} />
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
      )}
    </div>
  )
}
