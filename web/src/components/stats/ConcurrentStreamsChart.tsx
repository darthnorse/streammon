import {
  ResponsiveContainer,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  Legend,
} from 'recharts'
import type { ConcurrentTimePoint } from '../../types'
import { CHART_COLORS, AreaChartTooltip } from '../../lib/chartUtils'

interface ConcurrentStreamsChartProps {
  data: ConcurrentTimePoint[]
}

function formatDateTime(dateStr: string): string {
  const dt = new Date(dateStr)
  return dt.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

export function ConcurrentStreamsChart({ data }: ConcurrentStreamsChartProps) {
  const hasData = data && data.length > 0 && data.some(d => d.total > 0)

  return (
    <div className="card p-4 md:p-6">
      <h2 className="text-sm font-semibold uppercase tracking-wider text-muted dark:text-muted-dark mb-4">
        Concurrent Streams Over Time
      </h2>

      {!hasData && (
        <div className="h-[240px] flex items-center justify-center text-muted dark:text-muted-dark text-sm">
          No concurrent stream data
        </div>
      )}

      {hasData && (
        <ResponsiveContainer width="100%" height={240}>
          <AreaChart data={data} margin={{ top: 4, right: 4, bottom: 0, left: -20 }}>
            <CartesianGrid
              strokeDasharray="3 3"
              stroke="currentColor"
              className="text-border dark:text-border-dark"
              opacity={0.5}
            />
            <XAxis
              dataKey="time"
              tickFormatter={(v: string) => {
                const dt = new Date(v)
                return dt.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
              }}
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
            <Tooltip content={<AreaChartTooltip labelFormatter={formatDateTime} />} />
            <Legend
              iconType="circle"
              iconSize={8}
              wrapperStyle={{ fontSize: 12, paddingTop: 8 }}
            />
            <Area
              type="stepAfter"
              dataKey="direct_play"
              name="Direct Play"
              stackId="1"
              stroke={CHART_COLORS[2]}
              fill={CHART_COLORS[2]}
              fillOpacity={0.6}
            />
            <Area
              type="stepAfter"
              dataKey="transcode"
              name="Transcode"
              stackId="1"
              stroke={CHART_COLORS[4]}
              fill={CHART_COLORS[4]}
              fillOpacity={0.6}
            />
          </AreaChart>
        </ResponsiveContainer>
      )}
    </div>
  )
}
