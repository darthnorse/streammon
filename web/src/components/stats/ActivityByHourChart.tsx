import {
  ResponsiveContainer,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from 'recharts'
import type { HourStat } from '../../types'
import { CHART_COLORS, BarChartTooltip } from '../../lib/chartUtils'

interface ActivityByHourChartProps {
  data: HourStat[]
}

function formatHour(hour: number): string {
  if (hour === 0) return '12am'
  if (hour === 12) return '12pm'
  if (hour < 12) return `${hour}am`
  return `${hour - 12}pm`
}

export function ActivityByHourChart({ data }: ActivityByHourChartProps) {
  const hasData = data && data.some(d => d.play_count > 0)

  return (
    <div className="card p-4 md:p-6">
      <h2 className="text-sm font-semibold uppercase tracking-wider text-muted dark:text-muted-dark mb-4">
        Activity by Hour of Day
      </h2>

      {!hasData && (
        <div className="h-[200px] flex items-center justify-center text-muted dark:text-muted-dark text-sm">
          No activity data
        </div>
      )}

      {hasData && (
        <ResponsiveContainer width="100%" height={200}>
          <BarChart data={data} margin={{ top: 4, right: 4, bottom: 0, left: -20 }}>
            <CartesianGrid
              strokeDasharray="3 3"
              stroke="currentColor"
              className="text-border dark:text-border-dark"
              opacity={0.5}
            />
            <XAxis
              dataKey="hour"
              tickFormatter={formatHour}
              tick={{ fontSize: 10, fill: 'currentColor' }}
              className="text-muted dark:text-muted-dark"
              tickLine={false}
              axisLine={false}
              interval={2}
            />
            <YAxis
              allowDecimals={false}
              tick={{ fontSize: 11, fill: 'currentColor' }}
              className="text-muted dark:text-muted-dark"
              tickLine={false}
              axisLine={false}
            />
            <Tooltip
              content={<BarChartTooltip labelFormatter={(h) => formatHour(Number(h))} />}
              wrapperStyle={{ backgroundColor: 'transparent', border: 'none', boxShadow: 'none' }}
              cursor={{ fill: 'currentColor', opacity: 0.1 }}
            />
            <Bar
              dataKey="play_count"
              fill={CHART_COLORS[1]}
              radius={[2, 2, 0, 0]}
            />
          </BarChart>
        </ResponsiveContainer>
      )}
    </div>
  )
}
