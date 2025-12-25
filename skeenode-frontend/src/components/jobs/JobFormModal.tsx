import { useState, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { X, Calendar, Terminal, Tag } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { api, type Job, type CreateJobInput } from '@/lib/api'

interface JobFormModalProps {
  isOpen: boolean
  onClose: () => void
  job?: Job | null
  onSuccess: () => void
}

const JOB_TYPES = ['SHELL', 'DOCKER', 'HTTP', 'KUBERNETES'] as const

export default function JobFormModal({ isOpen, onClose, job, onSuccess }: JobFormModalProps) {
  const [name, setName] = useState('')
  const [schedule, setSchedule] = useState('* * * * *')
  const [command, setCommand] = useState('')
  const [type, setType] = useState<typeof JOB_TYPES[number]>('SHELL')
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (job) {
      setName(job.name)
      setSchedule(job.schedule)
      setCommand(job.command)
      setType(job.type)
    } else {
      setName('')
      setSchedule('* * * * *')
      setCommand('')
      setType('SHELL')
    }
    setError('')
  }, [job, isOpen])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setIsLoading(true)

    try {
      const data: CreateJobInput = { name, schedule, command, type }
      
      if (job) {
        await api.updateJob(job.id, data)
      } else {
        await api.createJob(data)
      }
      
      onSuccess()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save job')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* Backdrop */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            className="fixed inset-0 bg-black/50 backdrop-blur-sm z-50"
          />

          {/* Modal */}
          <motion.div
            initial={{ opacity: 0, scale: 0.95, y: 20 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: 20 }}
            className="fixed inset-0 flex items-center justify-center z-50 p-4"
          >
            <Card className="w-full max-w-lg glass">
              <CardHeader className="flex flex-row items-center justify-between">
                <CardTitle>{job ? 'Edit Job' : 'Create New Job'}</CardTitle>
                <Button variant="ghost" size="icon" onClick={onClose}>
                  <X className="w-4 h-4" />
                </Button>
              </CardHeader>

              <CardContent>
                <form onSubmit={handleSubmit} className="space-y-4">
                  {error && (
                    <div className="p-3 rounded-lg bg-destructive/20 border border-destructive/30 text-destructive text-sm">
                      {error}
                    </div>
                  )}

                  <div className="space-y-2">
                    <label className="text-sm font-medium">Job Name</label>
                    <Input
                      placeholder="My Scheduled Job"
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      icon={<Tag className="w-4 h-4" />}
                      required
                    />
                  </div>

                  <div className="space-y-2">
                    <label className="text-sm font-medium">Cron Schedule</label>
                    <Input
                      placeholder="* * * * *"
                      value={schedule}
                      onChange={(e) => setSchedule(e.target.value)}
                      icon={<Calendar className="w-4 h-4" />}
                      required
                    />
                    <p className="text-xs text-muted-foreground">
                      Format: minute hour day month weekday
                    </p>
                  </div>

                  <div className="space-y-2">
                    <label className="text-sm font-medium">Command</label>
                    <Input
                      placeholder="echo 'Hello World'"
                      value={command}
                      onChange={(e) => setCommand(e.target.value)}
                      icon={<Terminal className="w-4 h-4" />}
                      required
                    />
                  </div>

                  <div className="space-y-2">
                    <label className="text-sm font-medium">Job Type</label>
                    <div className="grid grid-cols-4 gap-2">
                      {JOB_TYPES.map((t) => (
                        <button
                          key={t}
                          type="button"
                          onClick={() => setType(t)}
                          className={`px-3 py-2 rounded-lg border text-sm font-medium transition-colors ${
                            type === t
                              ? 'bg-primary text-primary-foreground border-primary'
                              : 'bg-card border-border hover:bg-secondary'
                          }`}
                        >
                          {t}
                        </button>
                      ))}
                    </div>
                  </div>

                  <div className="flex gap-2 pt-4">
                    <Button type="button" variant="outline" onClick={onClose} className="flex-1">
                      Cancel
                    </Button>
                    <Button type="submit" isLoading={isLoading} className="flex-1">
                      {job ? 'Update' : 'Create'} Job
                    </Button>
                  </div>
                </form>
              </CardContent>
            </Card>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  )
}
