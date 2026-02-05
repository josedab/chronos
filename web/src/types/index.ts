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

export interface ClusterNode {
  id: string
  address: string
  state: string
  commitIndex: number
  appliedIndex: number
  lastContact?: string
}

export interface StorageInfo {
  totalKeys: number
  lsmSize: number
  vlogSize: number
}

export interface ClusterDetails {
  state: string
  leader: string
  term: number
  commitIndex: number
  appliedIndex: number
  lastLogIndex: number
  snapshotIndex: number
  nodes: ClusterNode[]
  storage?: StorageInfo
}

export interface ChronosConfig {
  cluster?: {
    node_id?: string
    data_dir?: string
    raft?: {
      address?: string
      peers?: string[]
    }
  }
  server?: {
    http?: {
      address?: string
      read_timeout?: string
      write_timeout?: string
    }
  }
  scheduler?: {
    tick_interval?: string
    execution_timeout?: string
    default_retry_policy?: {
      max_attempts?: number
      initial_interval?: string
      max_interval?: string
      multiplier?: number
    }
  }
  dispatcher?: {
    http?: {
      timeout?: string
      max_idle_conns?: number
    }
  }
  metrics?: {
    prometheus?: {
      enabled?: boolean
      path?: string
    }
  }
  logging?: {
    level?: string
    format?: string
  }
}
