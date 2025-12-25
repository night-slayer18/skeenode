import { useState, useEffect } from 'react'
import { motion } from 'framer-motion'
import { Activity, Filter, RefreshCw, Clock, CheckCircle, XCircle, Loader } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import Header from '@/components/layout/Header'
import { api, type Execution } from '@/lib/api'
import { formatDate, truncateId, cn } from '@/lib/utils'

function getStatusIcon(status: string) {
  switch (status) {
    case 'SUCCESS': return <CheckCircle className="w-4 h-4 text-success" />
    case 'FAILED': return <XCircle className="w-4 h-4 text-destructive" />
    case 'RUNNING': return <Loader className="w-4 h-4 text-primary animate-spin" />
    default: return <Clock className="w-4 h-4 text-muted-foreground" />
  }
}

function getStatusBadgeVariant(status: string): 'success' | 'destructive' | 'default' | 'secondary' {
  switch (status) {
    case 'SUCCESS': return 'success'
    case 'FAILED': return 'destructive'
    case 'RUNNING': return 'default'
    default: return 'secondary'
  }
}

export default function ExecutionsPage() {
  const [executions, setExecutions] = useState<Execution[]>([])
  const [loading, setLoading] = useState(true)
  const [statusFilter, setStatusFilter] = useState<string | null>(null)

  const fetchExecutions = async () => {
    setLoading(true)
    try {
      const data = await api.getExecutions()
      setExecutions(data || [])
    } catch (error) {
      console.error('Failed to fetch executions:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchExecutions()
    const interval = setInterval(fetchExecutions, 15000)
    return () => clearInterval(interval)
  }, [])

  const filteredExecutions = statusFilter
    ? executions.filter(e => e.status === statusFilter)
    : executions

  const statuses = ['SUCCESS', 'FAILED', 'RUNNING', 'PENDING']

  return (
    <div className="min-h-screen">
      <Header title="Executions" description="View job execution history" />

      <div className="p-6 space-y-6">
        {/* Filters */}
        <div className="flex flex-wrap gap-2 items-center">
          <Filter className="w-4 h-4 text-muted-foreground" />
          <Button
            variant={statusFilter === null ? 'default' : 'outline'}
            size="sm"
            onClick={() => setStatusFilter(null)}
          >
            All
          </Button>
          {statuses.map(status => (
            <Button
              key={status}
              variant={statusFilter === status ? 'default' : 'outline'}
              size="sm"
              onClick={() => setStatusFilter(status)}
            >
              {status}
            </Button>
          ))}
          <div className="flex-1" />
          <Button variant="outline" size="sm" onClick={fetchExecutions}>
            <RefreshCw className="w-4 h-4 mr-2" />
            Refresh
          </Button>
        </div>

        {/* Executions Table */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Activity className="w-5 h-5 text-primary" />
              Execution History
              <Badge variant="secondary" className="ml-2">
                {filteredExecutions.length} total
              </Badge>
            </CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="flex items-center justify-center py-20">
                <div className="animate-spin w-8 h-8 border-2 border-primary border-t-transparent rounded-full" />
              </div>
            ) : filteredExecutions.length === 0 ? (
              <div className="text-center py-20 text-muted-foreground">
                No executions found
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="border-b border-border">
                      <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">Status</th>
                      <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">Execution ID</th>
                      <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">Job ID</th>
                      <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">Scheduled</th>
                      <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">Started</th>
                      <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">Duration</th>
                      <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">Exit Code</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredExecutions.map((exec, i) => (
                      <motion.tr
                        key={exec.id}
                        initial={{ opacity: 0, y: 10 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ delay: i * 0.02 }}
                        className={cn(
                          'border-b border-border/50 hover:bg-secondary/30 transition-colors',
                          exec.status === 'RUNNING' && 'bg-primary/5'
                        )}
                      >
                        <td className="py-3 px-4">
                          <div className="flex items-center gap-2">
                            {getStatusIcon(exec.status)}
                            <Badge variant={getStatusBadgeVariant(exec.status)} pulse={exec.status === 'RUNNING'}>
                              {exec.status}
                            </Badge>
                          </div>
                        </td>
                        <td className="py-3 px-4">
                          <code className="text-xs font-mono text-muted-foreground">
                            {truncateId(exec.id)}
                          </code>
                        </td>
                        <td className="py-3 px-4">
                          <code className="text-xs font-mono text-muted-foreground">
                            {truncateId(exec.job_id)}
                          </code>
                        </td>
                        <td className="py-3 px-4 text-sm">
                          {formatDate(exec.scheduled_at)}
                        </td>
                        <td className="py-3 px-4 text-sm">
                          {exec.started_at ? formatDate(exec.started_at) : '-'}
                        </td>
                        <td className="py-3 px-4 text-sm text-muted-foreground">
                          {exec.duration_ms ? `${(exec.duration_ms / 1000).toFixed(2)}s` : '-'}
                        </td>
                        <td className="py-3 px-4">
                          {exec.exit_code !== undefined ? (
                            <Badge variant={exec.exit_code === 0 ? 'success' : 'destructive'}>
                              {exec.exit_code}
                            </Badge>
                          ) : '-'}
                        </td>
                      </motion.tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
