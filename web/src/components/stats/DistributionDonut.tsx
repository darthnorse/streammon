import {
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  Tooltip,
  Legend,
} from 'recharts'
import type { DistributionStat } from '../../types'
import { CHART_COLORS, PieChartTooltip } from '../../lib/chartUtils'

interface DistributionDonutProps {
  title: string
  data: DistributionStat[]
}

export function DistributionDonut({ title, data }: DistributionDonutProps) {
  const hasData = data && data.length > 0 && data.some(d => d.count > 0)
  const chartData = data?.slice(0, 8) || []

  return (
    <div className="card p-4 md:p-6">
      <h2 className="text-sm font-semibold uppercase tracking-wider text-muted dark:text-muted-dark mb-4">
        {title}
      </h2>

      {!hasData && (
        <div className="h-[200px] flex items-center justify-center text-muted dark:text-muted-dark text-sm">
          No data available
        </div>
      )}

      {hasData && (
        <ResponsiveContainer width="100%" height={200}>
          <PieChart>
            <Pie
              data={chartData}
              dataKey="count"
              nameKey="name"
              cx="50%"
              cy="50%"
              innerRadius={50}
              outerRadius={80}
              paddingAngle={2}
            >
              {chartData.map((_, index) => (
                <Cell key={`cell-${index}`} fill={CHART_COLORS[index % CHART_COLORS.length]} />
              ))}
            </Pie>
            <Tooltip content={<PieChartTooltip />} />
            <Legend
              iconType="circle"
              iconSize={8}
              wrapperStyle={{ fontSize: 11 }}
              formatter={(value: string) => (
                <span className="text-gray-700 dark:text-gray-300">{value}</span>
              )}
            />
          </PieChart>
        </ResponsiveContainer>
      )}
    </div>
  )
}
