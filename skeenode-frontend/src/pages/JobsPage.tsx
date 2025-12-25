import { useState, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  Plus,
  Search,
  Play,
  Trash2,
  Calendar,
  Terminal,
  RefreshCw,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import Header from '@/components/layout/Header'
import JobFormModal from '@/components/jobs/JobFormModal'
import { api, type Job } from '@/lib/api'
import { formatDate } from '@/lib/utils'

function getJobTypeIcon(type: string) {
  switch (type) {
    case 'DOCKER': return '🐳'
    case 'HTTP': return '🌐'
    case 'KUBERNETES': return '☸️'
    default: return '💻'
  }
}

export default function JobsPage() {
  const [jobs, setJobs] = useState<Job[]>([])
  const [loading, setLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState('')
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [editingJob, setEditingJob] = useState<Job | null>(null)

  const fetchJobs = async () => {
    try {
      const data = await api.getJobs()
      setJobs(data || [])
    } catch (error) {
      console.error('Failed to fetch jobs:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchJobs()
  }, [])

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this job?')) return
    try {
      await api.deleteJob(id)
      setJobs(jobs.filter(j => j.id !== id))
    } catch (error) {
      console.error('Failed to delete job:', error)
    }
  }

  const handleTrigger = async (id: string) => {
    try {
      await api.triggerJob(id)
      // Show success feedback
    } catch (error) {
      console.error('Failed to trigger job:', error)
    }
  }

  const filteredJobs = jobs.filter(job =>
    job.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    job.command.toLowerCase().includes(searchQuery.toLowerCase())
  )

  const containerVariants = {
    hidden: { opacity: 0 },
    visible: {
      opacity: 1,
      transition: { staggerChildren: 0.05 }
    }
  }

  const itemVariants = {
    hidden: { opacity: 0, y: 20 },
    visible: { opacity: 1, y: 0 }
  }

  return (
    <div className="min-h-screen">
      <Header title="Jobs" description="Manage your scheduled jobs" />

      <div className="p-6 space-y-6">
        {/* Actions Bar */}
        <div className="flex flex-col sm:flex-row gap-4 justify-between">
          <div className="flex-1 max-w-md">
            <Input
              placeholder="Search jobs..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              icon={<Search className="w-4 h-4" />}
            />
          </div>
          <div className="flex gap-2">
            <Button variant="outline" onClick={fetchJobs}>
              <RefreshCw className="w-4 h-4 mr-2" />
              Refresh
            </Button>
            <Button onClick={() => { setEditingJob(null); setIsModalOpen(true) }}>
              <Plus className="w-4 h-4 mr-2" />
              New Job
            </Button>
          </div>
        </div>

        {/* Jobs Grid */}
        {loading ? (
          <div className="flex items-center justify-center py-20">
            <div className="animate-spin w-8 h-8 border-2 border-primary border-t-transparent rounded-full" />
          </div>
        ) : filteredJobs.length === 0 ? (
          <Card className="py-20">
            <CardContent className="text-center">
              <Calendar className="w-12 h-12 mx-auto text-muted-foreground mb-4" />
              <h3 className="text-lg font-semibold mb-2">No jobs found</h3>
              <p className="text-muted-foreground mb-4">
                {searchQuery ? 'Try a different search term' : 'Create your first job to get started'}
              </p>
              {!searchQuery && (
                <Button onClick={() => setIsModalOpen(true)}>
                  <Plus className="w-4 h-4 mr-2" />
                  Create Job
                </Button>
              )}
            </CardContent>
          </Card>
        ) : (
          <motion.div
            variants={containerVariants}
            initial="hidden"
            animate="visible"
            className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4"
          >
            <AnimatePresence>
              {filteredJobs.map((job) => (
                <motion.div
                  key={job.id}
                  variants={itemVariants}
                  layout
                  exit={{ opacity: 0, scale: 0.9 }}
                >
                  <Card className="group hover:border-primary/30 transition-colors">
                    <CardHeader className="pb-3">
                      <div className="flex items-start justify-between">
                        <div className="flex items-center gap-2">
                          <span className="text-2xl">{getJobTypeIcon(job.type)}</span>
                          <div>
                            <CardTitle className="text-base">{job.name}</CardTitle>
                            <Badge variant={job.status === 'ACTIVE' ? 'success' : 'secondary'} className="mt-1">
                              {job.status}
                            </Badge>
                          </div>
                        </div>
                        <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8"
                            onClick={() => handleTrigger(job.id)}
                          >
                            <Play className="w-4 h-4 text-success" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8"
                            onClick={() => handleDelete(job.id)}
                          >
                            <Trash2 className="w-4 h-4 text-destructive" />
                          </Button>
                        </div>
                      </div>
                    </CardHeader>
                    <CardContent className="space-y-3">
                      <div className="flex items-center gap-2 text-sm text-muted-foreground">
                        <Calendar className="w-4 h-4" />
                        <code className="bg-secondary px-2 py-0.5 rounded text-xs">{job.schedule}</code>
                      </div>
                      <div className="flex items-start gap-2 text-sm text-muted-foreground">
                        <Terminal className="w-4 h-4 mt-0.5 shrink-0" />
                        <code className="text-xs truncate">{job.command}</code>
                      </div>
                      {job.next_run_at && (
                        <p className="text-xs text-muted-foreground">
                          Next run: {formatDate(job.next_run_at)}
                        </p>
                      )}
                    </CardContent>
                  </Card>
                </motion.div>
              ))}
            </AnimatePresence>
          </motion.div>
        )}
      </div>

      {/* Create/Edit Modal */}
      <JobFormModal
        isOpen={isModalOpen}
        onClose={() => { setIsModalOpen(false); setEditingJob(null) }}
        job={editingJob}
        onSuccess={() => { setIsModalOpen(false); fetchJobs() }}
      />
    </div>
  )
}
