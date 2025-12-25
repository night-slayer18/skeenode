import { useMemo } from 'react'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts'
import type { Execution } from '@/lib/api'

interface ExecutionChartProps {
  executions: Execution[]
}

export default function ExecutionChart({ executions }: ExecutionChartProps) {
  const chartData = useMemo(() => {
    // Group by day for last 7 days
    const days = new Map<string, { success: number; failed: number; total: number }>()
    
    // Initialize last 7 days
    for (let i = 6; i >= 0; i--) {
      const date = new Date()
      date.setDate(date.getDate() - i)
      const key = date.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' })
      days.set(key, { success: 0, failed: 0, total: 0 })
    }

    // Count executions
    executions.forEach(exec => {
      const date = new Date(exec.scheduled_at)
      const key = date.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' })
      
      if (days.has(key)) {
        const counts = days.get(key)!
        counts.total++
        if (exec.status === 'SUCCESS') counts.success++
        if (exec.status === 'FAILED') counts.failed++
      }
    })

    return Array.from(days.entries()).map(([date, counts]) => ({
      date,
      ...counts,
    }))
  }, [executions])

  if (executions.length === 0) {
    return (
      <div className="h-64 flex items-center justify-center text-muted-foreground">
        No execution data to display
      </div>
    )
  }

  return (
    <ResponsiveContainer width="100%" height={300}>
      <LineChart data={chartData} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" opacity={0.3} />
        <XAxis 
          dataKey="date" 
          stroke="hsl(var(--muted-foreground))"
          fontSize={12}
          tickLine={false}
        />
        <YAxis 
          stroke="hsl(var(--muted-foreground))"
          fontSize={12}
          tickLine={false}
          axisLine={false}
        />
        <Tooltip
          contentStyle={{
            backgroundColor: 'hsl(var(--card))',
            border: '1px solid hsl(var(--border))',
            borderRadius: '8px',
            color: 'hsl(var(--foreground))',
          }}
        />
        <Legend />
        <Line
          type="monotone"
          dataKey="success"
          name="Success"
          stroke="oklch(0.7 0.18 145)"
          strokeWidth={2}
          dot={{ fill: 'oklch(0.7 0.18 145)', strokeWidth: 0, r: 4 }}
          activeDot={{ r: 6, strokeWidth: 0 }}
        />
        <Line
          type="monotone"
          dataKey="failed"
          name="Failed"
          stroke="oklch(0.6 0.2 25)"
          strokeWidth={2}
          dot={{ fill: 'oklch(0.6 0.2 25)', strokeWidth: 0, r: 4 }}
          activeDot={{ r: 6, strokeWidth: 0 }}
        />
        <Line
          type="monotone"
          dataKey="total"
          name="Total"
          stroke="oklch(0.65 0.15 145)"
          strokeWidth={2}
          strokeDasharray="5 5"
          dot={false}
        />
      </LineChart>
    </ResponsiveContainer>
  )
}
