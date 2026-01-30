import axios from 'axios'
import type { ApiResponse, Job, JobsResponse, ExecutionsResponse, ClusterStatus, Execution } from '../types'

const api = axios.create({
  baseURL: '/api/v1',
  headers: {
    'Content-Type': 'application/json',
  },
})

// Jobs
export async function getJobs(): Promise<JobsResponse> {
  const { data } = await api.get<ApiResponse<JobsResponse>>('/jobs')
  if (!data.success || !data.data) {
    throw new Error(data.error?.message || 'Failed to fetch jobs')
  }
  return data.data
}

export async function getJob(id: string): Promise<Job> {
  const { data } = await api.get<ApiResponse<Job>>(`/jobs/${id}`)
  if (!data.success || !data.data) {
    throw new Error(data.error?.message || 'Failed to fetch job')
  }
  return data.data
}

export async function createJob(job: Partial<Job>): Promise<Job> {
  const { data } = await api.post<ApiResponse<Job>>('/jobs', job)
  if (!data.success || !data.data) {
    throw new Error(data.error?.message || 'Failed to create job')
  }
  return data.data
}

export async function updateJob(id: string, job: Partial<Job>): Promise<Job> {
  const { data } = await api.put<ApiResponse<Job>>(`/jobs/${id}`, job)
  if (!data.success || !data.data) {
    throw new Error(data.error?.message || 'Failed to update job')
  }
  return data.data
}

export async function deleteJob(id: string): Promise<void> {
  const { data } = await api.delete<ApiResponse<null>>(`/jobs/${id}`)
  if (!data.success) {
    throw new Error(data.error?.message || 'Failed to delete job')
  }
}

export async function triggerJob(id: string): Promise<Execution> {
  const { data } = await api.post<ApiResponse<Execution>>(`/jobs/${id}/trigger`)
  if (!data.success || !data.data) {
    throw new Error(data.error?.message || 'Failed to trigger job')
  }
  return data.data
}

export async function enableJob(id: string): Promise<Job> {
  const { data } = await api.post<ApiResponse<Job>>(`/jobs/${id}/enable`)
  if (!data.success || !data.data) {
    throw new Error(data.error?.message || 'Failed to enable job')
  }
  return data.data
}

export async function disableJob(id: string): Promise<Job> {
  const { data } = await api.post<ApiResponse<Job>>(`/jobs/${id}/disable`)
  if (!data.success || !data.data) {
    throw new Error(data.error?.message || 'Failed to disable job')
  }
  return data.data
}

// Executions
export async function getExecutions(jobId: string, limit = 20): Promise<ExecutionsResponse> {
  const { data } = await api.get<ApiResponse<ExecutionsResponse>>(`/jobs/${jobId}/executions`, {
    params: { limit },
  })
  if (!data.success || !data.data) {
    throw new Error(data.error?.message || 'Failed to fetch executions')
  }
  return data.data
}

// Cluster
export async function getClusterStatus(): Promise<ClusterStatus> {
  const { data } = await api.get<ApiResponse<ClusterStatus>>('/cluster/status')
  if (!data.success || !data.data) {
    throw new Error(data.error?.message || 'Failed to fetch cluster status')
  }
  return data.data
}
