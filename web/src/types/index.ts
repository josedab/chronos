export interface Job {
  id: string
  name: string
  description?: string
  schedule: string
  timezone?: string
  webhook: WebhookConfig
  retry_policy?: RetryPolicy
  timeout?: string
  concurrency?: 'allow' | 'forbid' | 'replace'
  tags?: Record<string, string>
  enabled: boolean
  created_at: string
  updated_at: string
  next_run?: string
}

export interface WebhookConfig {
  url: string
  method: string
  headers?: Record<string, string>
  body?: string
  auth?: AuthConfig
  success_codes?: number[]
}

export interface AuthConfig {
  type: 'basic' | 'bearer' | 'api_key'
  token?: string
  api_key?: string
  header?: string
  username?: string
  password?: string
}

export interface RetryPolicy {
  max_attempts: number
  initial_interval: string
  max_interval: string
  multiplier: number
}

export interface Execution {
  id: string
  job_id: string
  job_name?: string
  scheduled_time: string
  started_at: string
  completed_at?: string
  status: 'pending' | 'running' | 'success' | 'failed' | 'skipped'
  attempts: number
  status_code?: number
  response?: string
  error?: string
  duration?: number
  node_id?: string
}

export interface ApiResponse<T> {
  success: boolean
  data?: T
  error?: {
    code: string
    message: string
  }
}

export interface JobsResponse {
  jobs: Job[]
  total: number
}

export interface ExecutionsResponse {
  executions: Execution[]
  total: number
}

export interface ClusterStatus {
  is_leader: boolean
  jobs_total: number
  running: number
  next_runs: { job_id: string; next_run: string }[]
}
