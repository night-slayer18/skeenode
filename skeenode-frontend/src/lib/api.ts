const API_BASE = '/api'

interface FetchOptions extends RequestInit {
  token?: string
}

async function fetchApi<T>(endpoint: string, options: FetchOptions = {}): Promise<T> {
  const { token, ...fetchOptions } = options
  
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...fetchOptions,
    headers,
  })

  if (!response.ok) {
    const error = await response.json().catch(() => ({ message: 'Request failed' }))
    throw new Error(error.message || `HTTP ${response.status}`)
  }

  return response.json()
}

// Job Types
export interface Job {
  id: string
  name: string
  schedule: string
  command: string
  type: 'SHELL' | 'DOCKER' | 'HTTP' | 'KUBERNETES'
  status: 'ACTIVE' | 'PAUSED' | 'DELETED'
  owner_id: string
  next_run_at?: string
  last_run_at?: string
  created_at: string
  updated_at: string
}

export interface CreateJobInput {
  name: string
  schedule: string
  command: string
  type: 'SHELL' | 'DOCKER' | 'HTTP' | 'KUBERNETES'
  owner_id?: string
}

// Execution Types
export interface Execution {
  id: string
  job_id: string
  status: 'PENDING' | 'RUNNING' | 'SUCCESS' | 'FAILED' | 'CANCELLED'
  scheduled_at: string
  started_at?: string
  completed_at?: string
  exit_code?: number
  node_id?: string
  log_path?: string
  duration_ms?: number
}

// Stats
export interface DashboardStats {
  total_jobs: number
  active_jobs: number
  paused_jobs: number
  total_executions: number
  success_rate: number
  failed_today: number
  running_now: number
}

// API Functions
export const api = {
  // Jobs
  getJobs: async (token?: string) => {
    const res = await fetchApi<{ jobs: Job[]; count?: number }>('/jobs', { token })
    return res.jobs || []
  },
  getJob: (id: string, token?: string) => fetchApi<Job>(`/jobs/${id}`, { token }),
  createJob: (data: CreateJobInput, token?: string) =>
    fetchApi<Job>('/jobs', { method: 'POST', body: JSON.stringify(data), token }),
  updateJob: (id: string, data: Partial<CreateJobInput>, token?: string) =>
    fetchApi<Job>(`/jobs/${id}`, { method: 'PUT', body: JSON.stringify(data), token }),
  deleteJob: (id: string, token?: string) =>
    fetchApi<void>(`/jobs/${id}`, { method: 'DELETE', token }),
  pauseJob: (id: string, token?: string) =>
    fetchApi<Job>(`/jobs/${id}/pause`, { method: 'POST', token }),
  resumeJob: (id: string, token?: string) =>
    fetchApi<Job>(`/jobs/${id}/resume`, { method: 'POST', token }),
  triggerJob: (id: string, token?: string) =>
    fetchApi<Execution>(`/jobs/${id}/trigger`, { method: 'POST', token }),

  // Executions
  getExecutions: (token?: string) => fetchApi<Execution[]>('/executions', { token }),
  getJobExecutions: (jobId: string, token?: string) =>
    fetchApi<Execution[]>(`/jobs/${jobId}/executions`, { token }),
  getExecution: (id: string, token?: string) =>
    fetchApi<Execution>(`/executions/${id}`, { token }),

  // Stats
  getStats: async (token?: string): Promise<DashboardStats> => {
    try {
      return await fetchApi<DashboardStats>('/stats', { token })
    } catch {
      // Return mock stats if endpoint doesn't exist
      return {
        total_jobs: 0,
        active_jobs: 0,
        paused_jobs: 0,
        total_executions: 0,
        success_rate: 0,
        failed_today: 0,
        running_now: 0,
      }
    }
  },

  // Health
  getHealth: () => fetchApi<{ status: string; service: string }>('/health'),
}
