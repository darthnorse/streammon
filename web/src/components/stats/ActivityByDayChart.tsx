import {
  ResponsiveContainer,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from 'recharts'
import type { DayOfWeekStat } from '../../types'
import { CHART_COLORS, BarChartTooltip } from '../../lib/chartUtils'

interface ActivityByDayChartProps {
  data: DayOfWeekStat[]
}

export function ActivityByDayChart({ data }: ActivityByDayChartProps) {
  const hasData = data && data.some(d => d.play_count > 0)

  return (
    <div className="card p-4 md:p-6">
      <h2 className="text-sm font-semibold uppercase tracking-wider text-muted dark:text-muted-dark mb-4">
        Activity by Day of Week
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
              dataKey="day_name"
              tick={{ fontSize: 11, fill: 'currentColor' }}
              className="text-muted dark:text-muted-dark"
              tickLine={false}
              axisLine={false}
            />
            <YAxis
              allowDecimals={false}
              tick={{ fontSize: 11, fill: 'currentColor' }}
              className="text-muted dark:text-muted-dark"
              tickLine={false}
              axisLine={false}
            />
            <Tooltip content={<BarChartTooltip />} />
            <Bar
              dataKey="play_count"
              fill={CHART_COLORS[0]}
              radius={[4, 4, 0, 0]}
            />
          </BarChart>
        </ResponsiveContainer>
      )}
    </div>
  )
}
