import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { getJob, getExecutions, deleteJob, enableJob, disableJob, cancelExecution } from '../api/client'
import { format } from 'date-fns'
import TriggerModal from '../components/TriggerModal'
import JobHistory from '../components/JobHistory'
import DurationChart from '../components/DurationChart'

export default function JobDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [showTriggerModal, setShowTriggerModal] = useState(false)

  const { data: job, isLoading: jobLoading } = useQuery({
    queryKey: ['job', id],
    queryFn: () => getJob(id!),
    enabled: !!id,
  })

  const { data: executionsData, isLoading: execLoading } = useQuery({
    queryKey: ['executions', id],
    queryFn: () => getExecutions(id!, 20),
    enabled: !!id,
  })

  const deleteMutation = useMutation({
    mutationFn: deleteJob,
    onSuccess: () => navigate('/jobs'),
  })

  const toggleMutation = useMutation({
    mutationFn: () => (job?.enabled ? disableJob(id!) : enableJob(id!)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['job', id] }),
  })

  const cancelMutation = useMutation({
    mutationFn: (executionId: string) => cancelExecution(id!, executionId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['executions', id] }),
  })

  if (jobLoading) {
    return <div className="text-center py-12">Loading job...</div>
  }

  if (!job) {
    return <div className="text-center py-12 text-red-600">Job not found</div>
  }

  const statusColors = {
    pending: 'bg-gray-100 text-gray-800',
    running: 'bg-blue-100 text-blue-800',
    success: 'bg-green-100 text-green-800',
    failed: 'bg-red-100 text-red-800',
    skipped: 'bg-yellow-100 text-yellow-800',
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex justify-between items-start">
        <div>
          <div className="flex items-center space-x-3">
            <h2 className="text-2xl font-bold text-gray-900">{job.name}</h2>
            <span className={`px-2 py-1 text-xs font-semibold rounded-full ${
              job.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
            }`}>
              {job.enabled ? 'Enabled' : 'Disabled'}
            </span>
          </div>
          {job.description && <p className="mt-1 text-gray-500">{job.description}</p>}
        </div>
        <div className="flex space-x-2">
          <button
            onClick={() => setShowTriggerModal(true)}
            className="px-4 py-2 text-sm text-white bg-indigo-600 rounded hover:bg-indigo-700 disabled:opacity-50 flex items-center gap-1"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
            </svg>
            Trigger
          </button>
          <Link
            to={`/jobs/${id}/edit`}
            className="px-4 py-2 text-sm text-indigo-600 border border-indigo-300 rounded hover:bg-indigo-50"
          >
            Edit
          </Link>
          <Link
            to={`/jobs/create?duplicate=${id}`}
            className="px-4 py-2 text-sm text-purple-600 border border-purple-300 rounded hover:bg-purple-50 flex items-center gap-1"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
            Duplicate
          </Link>
          <button
            onClick={() => toggleMutation.mutate()}
            disabled={toggleMutation.isPending}
            className={`px-4 py-2 text-sm rounded ${
              job.enabled
                ? 'text-yellow-600 border border-yellow-300 hover:bg-yellow-50'
                : 'text-green-600 border border-green-300 hover:bg-green-50'
            }`}
          >
            {job.enabled ? 'Disable' : 'Enable'}
          </button>
          <button
            onClick={() => {
              if (confirm('Are you sure you want to delete this job?')) {
                deleteMutation.mutate(id!)
              }
            }}
            disabled={deleteMutation.isPending}
            className="px-4 py-2 text-sm text-red-600 border border-red-300 rounded hover:bg-red-50"
          >
            Delete
          </button>
        </div>
      </div>

      {/* Job Details */}
      <div className="bg-white shadow rounded-lg">
        <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
          <h3 className="text-lg font-medium text-gray-900">Configuration</h3>
        </div>
        <div className="px-4 py-5 sm:p-6">
          <dl className="grid grid-cols-1 gap-x-4 gap-y-6 sm:grid-cols-2">
            <div>
              <dt className="text-sm font-medium text-gray-500">Schedule</dt>
              <dd className="mt-1 text-sm text-gray-900 font-mono">{job.schedule}</dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Timezone</dt>
              <dd className="mt-1 text-sm text-gray-900">{job.timezone || 'UTC'}</dd>
            </div>
            {job.next_run && job.enabled && (
              <div>
                <dt className="text-sm font-medium text-gray-500">Next Run</dt>
                <dd className="mt-1 text-sm text-gray-900">
                  {format(new Date(job.next_run), 'MMM d, yyyy HH:mm:ss')}
                </dd>
              </div>
            )}
            <div>
              <dt className="text-sm font-medium text-gray-500">Webhook URL</dt>
              <dd className="mt-1 text-sm text-gray-900 break-all">{job.webhook.url}</dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">HTTP Method</dt>
              <dd className="mt-1 text-sm text-gray-900">{job.webhook.method || 'GET'}</dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Timeout</dt>
              <dd className="mt-1 text-sm text-gray-900">{job.timeout || '5m'}</dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Concurrency</dt>
              <dd className="mt-1 text-sm text-gray-900">{job.concurrency || 'forbid'}</dd>
            </div>
            {job.retry_policy && (
              <div>
                <dt className="text-sm font-medium text-gray-500">Retry Policy</dt>
                <dd className="mt-1 text-sm text-gray-900">
                  Max {job.retry_policy.max_attempts} attempts, 
                  starting at {job.retry_policy.initial_interval}
                </dd>
              </div>
            )}
          </dl>
        </div>
      </div>

      {/* Charts and History Row */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Duration Chart - takes 2 columns */}
        <div className="lg:col-span-2">
          {executionsData && (
            <DurationChart executions={executionsData.executions} />
          )}
        </div>
        
        {/* Job History - takes 1 column */}
        <JobHistory job={job} />
      </div>

      {/* Executions */}
      <div className="bg-white shadow rounded-lg">
        <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
          <h3 className="text-lg font-medium text-gray-900">Recent Executions</h3>
        </div>
        <div className="px-4 py-5 sm:p-6">
          {execLoading ? (
            <p className="text-gray-500">Loading executions...</p>
          ) : executionsData?.executions.length === 0 ? (
            <p className="text-gray-500">No executions yet</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead>
                  <tr>
                    <th className="px-3 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                    <th className="px-3 py-3 text-left text-xs font-medium text-gray-500 uppercase">Started</th>
                    <th className="px-3 py-3 text-left text-xs font-medium text-gray-500 uppercase">Duration</th>
                    <th className="px-3 py-3 text-left text-xs font-medium text-gray-500 uppercase">Attempts</th>
                    <th className="px-3 py-3 text-left text-xs font-medium text-gray-500 uppercase">Response</th>
                    <th className="px-3 py-3 text-left text-xs font-medium text-gray-500 uppercase">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {executionsData?.executions.map((exec) => (
                    <tr key={exec.id}>
                      <td className="px-3 py-4 whitespace-nowrap">
                        <span className={`px-2 py-1 text-xs font-semibold rounded-full ${statusColors[exec.status]}`}>
                          {exec.status}
                        </span>
                      </td>
                      <td className="px-3 py-4 whitespace-nowrap text-sm text-gray-900">
                        {format(new Date(exec.started_at), 'MMM d, HH:mm:ss')}
                      </td>
                      <td className="px-3 py-4 whitespace-nowrap text-sm text-gray-500">
                        {exec.duration ? `${(exec.duration / 1e9).toFixed(2)}s` : '-'}
                      </td>
                      <td className="px-3 py-4 whitespace-nowrap text-sm text-gray-500">
                        {exec.attempts}
                      </td>
                      <td className="px-3 py-4 text-sm text-gray-500 max-w-xs truncate">
                        {exec.error || exec.response || '-'}
                      </td>
                      <td className="px-3 py-4 whitespace-nowrap">
                        {(exec.status === 'running' || exec.status === 'pending') && (
                          <button
                            onClick={() => cancelMutation.mutate(exec.id)}
                            disabled={cancelMutation.isPending}
                            className="text-xs text-red-600 hover:text-red-800 flex items-center gap-1"
                          >
                            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                            </svg>
                            Cancel
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>

      <div className="flex justify-start">
        <Link to="/jobs" className="text-indigo-600 hover:text-indigo-800">
          ← Back to Jobs
        </Link>
      </div>

      {/* Trigger Modal */}
      {showTriggerModal && (
        <TriggerModal
          jobId={id!}
          jobName={job.name}
          onClose={() => setShowTriggerModal(false)}
        />
      )}
    </div>
  )
}
