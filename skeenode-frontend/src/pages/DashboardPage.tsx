import { useEffect, useState } from 'react'
import { motion } from 'framer-motion'
import {
  Calendar,
  Activity,
  CheckCircle,
  XCircle,
  Clock,
  TrendingUp,
  Play,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import Header from '@/components/layout/Header'
import { api, type Job, type Execution } from '@/lib/api'
import { formatRelativeTime, truncateId } from '@/lib/utils'
import ExecutionChart from '@/components/charts/ExecutionChart'

interface StatCardProps {
  title: string
  value: string | number
  icon: React.ReactNode
  trend?: { value: number; positive: boolean }
  color?: 'primary' | 'success' | 'warning' | 'destructive'
}

function StatCard({ title, value, icon, trend, color = 'primary' }: StatCardProps) {
  const colorClasses = {
    primary: 'from-primary/20 to-primary/5 text-primary',
    success: 'from-success/20 to-success/5 text-success',
    warning: 'from-warning/20 to-warning/5 text-warning',
    destructive: 'from-destructive/20 to-destructive/5 text-destructive',
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      whileHover={{ scale: 1.02 }}
      transition={{ duration: 0.2 }}
    >
      <Card className="overflow-hidden">
        <CardContent className="p-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-muted-foreground">{title}</p>
              <motion.p
                className="text-3xl font-bold mt-1"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ delay: 0.2 }}
              >
                {value}
              </motion.p>
              {trend && (
                <div className="flex items-center gap-1 mt-1">
                  <TrendingUp className={`w-3 h-3 ${trend.positive ? 'text-success' : 'text-destructive'}`} />
                  <span className={`text-xs ${trend.positive ? 'text-success' : 'text-destructive'}`}>
                    {trend.positive ? '+' : ''}{trend.value}%
                  </span>
                </div>
              )}
            </div>
            <div className={`w-12 h-12 rounded-xl bg-gradient-to-br ${colorClasses[color]} flex items-center justify-center`}>
              {icon}
            </div>
          </div>
        </CardContent>
      </Card>
    </motion.div>
  )
}

function getStatusBadgeVariant(status: string) {
  switch (status.toUpperCase()) {
    case 'SUCCESS': return 'success'
    case 'FAILED': return 'destructive'
    case 'RUNNING': return 'default'
    case 'PENDING': return 'secondary'
    default: return 'outline'
  }
}

export default function DashboardPage() {
  const [jobs, setJobs] = useState<Job[]>([])
  const [executions, setExecutions] = useState<Execution[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [jobsData, execsData] = await Promise.all([
          api.getJobs(),
          api.getExecutions(),
        ])
        setJobs(jobsData || [])
        setExecutions(execsData || [])
      } catch (error) {
        console.error('Failed to fetch data:', error)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
    const interval = setInterval(fetchData, 10000)
    return () => clearInterval(interval)
  }, [])

  const activeJobs = jobs.filter(j => j.status === 'ACTIVE').length
  const successRate = executions.length > 0
    ? Math.round((executions.filter(e => e.status === 'SUCCESS').length / executions.length) * 100)
    : 0
  const runningNow = executions.filter(e => e.status === 'RUNNING').length
  const failedToday = executions.filter(e => {
    const today = new Date().toDateString()
    return e.status === 'FAILED' && new Date(e.scheduled_at).toDateString() === today
  }).length

  const recentExecutions = executions.slice(0, 8)

  const containerVariants = {
    hidden: { opacity: 0 },
    visible: {
      opacity: 1,
      transition: { staggerChildren: 0.1 }
    }
  }

  return (
    <div className="min-h-screen">
      <Header title="Dashboard" description="Overview of your job scheduler" />
      
      <div className="p-6 space-y-6">
        {/* Stats Grid */}
        <motion.div
          variants={containerVariants}
          initial="hidden"
          animate="visible"
          className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4"
        >
          <StatCard
            title="Total Jobs"
            value={jobs.length}
            icon={<Calendar className="w-6 h-6" />}
            color="primary"
          />
          <StatCard
            title="Active Jobs"
            value={activeJobs}
            icon={<Play className="w-6 h-6" />}
            color="success"
            trend={{ value: 12, positive: true }}
          />
          <StatCard
            title="Success Rate"
            value={`${successRate}%`}
            icon={<CheckCircle className="w-6 h-6" />}
            color="success"
          />
          <StatCard
            title="Failed Today"
            value={failedToday}
            icon={<XCircle className="w-6 h-6" />}
            color={failedToday > 0 ? 'destructive' : 'success'}
          />
        </motion.div>

        {/* Charts and Activity */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Chart */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.3 }}
            className="lg:col-span-2"
          >
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Activity className="w-5 h-5 text-primary" />
                  Execution History
                </CardTitle>
              </CardHeader>
              <CardContent>
                <ExecutionChart executions={executions} />
              </CardContent>
            </Card>
          </motion.div>

          {/* Recent Activity */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.4 }}
          >
            <Card className="h-full">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Clock className="w-5 h-5 text-primary" />
                  Recent Activity
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                {loading ? (
                  <div className="flex items-center justify-center py-8">
                    <div className="animate-spin w-6 h-6 border-2 border-primary border-t-transparent rounded-full" />
                  </div>
                ) : recentExecutions.length === 0 ? (
                  <p className="text-center text-muted-foreground py-8">
                    No recent executions
                  </p>
                ) : (
                  recentExecutions.map((exec, i) => (
                    <motion.div
                      key={exec.id}
                      initial={{ opacity: 0, x: -10 }}
                      animate={{ opacity: 1, x: 0 }}
                      transition={{ delay: i * 0.05 }}
                      className="flex items-center justify-between py-2 border-b border-border last:border-0"
                    >
                      <div className="flex items-center gap-3">
                        <Badge
                          variant={getStatusBadgeVariant(exec.status)}
                          pulse={exec.status === 'RUNNING'}
                        >
                          {exec.status}
                        </Badge>
                        <span className="text-sm font-mono text-muted-foreground">
                          {truncateId(exec.job_id)}
                        </span>
                      </div>
                      <span className="text-xs text-muted-foreground">
                        {formatRelativeTime(exec.scheduled_at)}
                      </span>
                    </motion.div>
                  ))
                )}
              </CardContent>
            </Card>
          </motion.div>
        </div>

        {/* Running Jobs */}
        {runningNow > 0 && (
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.5 }}
          >
            <Card className="border-primary/30 bg-primary/5">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-primary">
                  <div className="w-2 h-2 rounded-full bg-primary animate-pulse" />
                  {runningNow} Job{runningNow > 1 ? 's' : ''} Running Now
                </CardTitle>
              </CardHeader>
            </Card>
          </motion.div>
        )}
      </div>
    </div>
  )
}
