import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { getJob, updateJob } from '../api/client'
import type { Job } from '../types'

export default function JobEdit() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data: job, isLoading } = useQuery({
    queryKey: ['job', id],
    queryFn: () => getJob(id!),
    enabled: !!id,
  })

  const [form, setForm] = useState({
    name: '',
    description: '',
    schedule: '',
    timezone: '',
    webhookUrl: '',
    webhookMethod: 'GET',
    webhookHeaders: '',
    webhookBody: '',
    timeout: '5m',
    concurrency: 'forbid',
    enabled: true,
    maxAttempts: 3,
    initialInterval: '1s',
    maxInterval: '1m',
    multiplier: 2.0,
  })

  // Populate form when job data loads
  useEffect(() => {
    if (job) {
      setForm({
        name: job.name,
        description: job.description || '',
        schedule: job.schedule,
        timezone: job.timezone || '',
        webhookUrl: job.webhook.url,
        webhookMethod: job.webhook.method || 'GET',
        webhookHeaders: job.webhook.headers ? JSON.stringify(job.webhook.headers, null, 2) : '',
        webhookBody: job.webhook.body || '',
        timeout: job.timeout || '5m',
        concurrency: job.concurrency || 'forbid',
        enabled: job.enabled,
        maxAttempts: job.retry_policy?.max_attempts || 3,
        initialInterval: job.retry_policy?.initial_interval || '1s',
        maxInterval: job.retry_policy?.max_interval || '1m',
        multiplier: job.retry_policy?.multiplier || 2.0,
      })
    }
  }, [job])

  const mutation = useMutation({
    mutationFn: (data: Partial<Job>) => updateJob(id!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['job', id] })
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      navigate(`/jobs/${id}`)
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    let headers: Record<string, string> | undefined
    if (form.webhookHeaders.trim()) {
      try {
        headers = JSON.parse(form.webhookHeaders)
      } catch {
        alert('Invalid JSON in headers field')
        return
      }
    }

    mutation.mutate({
      name: form.name,
      description: form.description || undefined,
      schedule: form.schedule,
      timezone: form.timezone || undefined,
      webhook: {
        url: form.webhookUrl,
        method: form.webhookMethod,
        headers,
        body: form.webhookBody || undefined,
      },
      timeout: form.timeout,
      concurrency: form.concurrency as 'allow' | 'forbid' | 'replace',
      retry_policy: {
        max_attempts: form.maxAttempts,
        initial_interval: form.initialInterval,
        max_interval: form.maxInterval,
        multiplier: form.multiplier,
      },
      enabled: form.enabled,
    })
  }

  if (isLoading) {
    return <div className="text-center py-12">Loading job...</div>
  }

  if (!job) {
    return <div className="text-center py-12 text-red-600">Job not found</div>
  }

  return (
    <div className="max-w-2xl mx-auto">
      <h2 className="text-2xl font-bold text-gray-900 mb-6">Edit Job: {job.name}</h2>

      <form onSubmit={handleSubmit} className="bg-white shadow rounded-lg">
        <div className="px-4 py-5 sm:p-6 space-y-6">
          {/* Name */}
          <div>
            <label htmlFor="name" className="block text-sm font-medium text-gray-700">
              Name *
            </label>
            <input
              type="text"
              id="name"
              required
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            />
          </div>

          {/* Description */}
          <div>
            <label htmlFor="description" className="block text-sm font-medium text-gray-700">
              Description
            </label>
            <textarea
              id="description"
              rows={2}
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            />
          </div>

          {/* Schedule */}
          <div>
            <label htmlFor="schedule" className="block text-sm font-medium text-gray-700">
              Schedule (Cron Expression) *
            </label>
            <input
              type="text"
              id="schedule"
              required
              value={form.schedule}
              onChange={(e) => setForm({ ...form, schedule: e.target.value })}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm font-mono"
            />
            <p className="mt-1 text-sm text-gray-500">
              Examples: <code>* * * * *</code> (every minute), <code>0 9 * * *</code> (daily at 9am), <code>@hourly</code>
            </p>
          </div>

          {/* Timezone */}
          <div>
            <label htmlFor="timezone" className="block text-sm font-medium text-gray-700">
              Timezone
            </label>
            <input
              type="text"
              id="timezone"
              value={form.timezone}
              onChange={(e) => setForm({ ...form, timezone: e.target.value })}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
              placeholder="America/New_York (default: UTC)"
            />
          </div>

          <hr className="border-gray-200" />
          <h3 className="text-lg font-medium text-gray-900">Webhook Configuration</h3>

          {/* Webhook URL */}
          <div>
            <label htmlFor="webhookUrl" className="block text-sm font-medium text-gray-700">
              URL *
            </label>
            <input
              type="url"
              id="webhookUrl"
              required
              value={form.webhookUrl}
              onChange={(e) => setForm({ ...form, webhookUrl: e.target.value })}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            />
          </div>

          {/* HTTP Method */}
          <div>
            <label htmlFor="webhookMethod" className="block text-sm font-medium text-gray-700">
              HTTP Method
            </label>
            <select
              id="webhookMethod"
              value={form.webhookMethod}
              onChange={(e) => setForm({ ...form, webhookMethod: e.target.value })}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            >
              <option value="GET">GET</option>
              <option value="POST">POST</option>
              <option value="PUT">PUT</option>
              <option value="DELETE">DELETE</option>
              <option value="PATCH">PATCH</option>
            </select>
          </div>

          {/* Headers */}
          <div>
            <label htmlFor="webhookHeaders" className="block text-sm font-medium text-gray-700">
              Headers (JSON)
            </label>
            <textarea
              id="webhookHeaders"
              rows={3}
              value={form.webhookHeaders}
              onChange={(e) => setForm({ ...form, webhookHeaders: e.target.value })}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm font-mono text-xs"
              placeholder='{"Authorization": "Bearer token123"}'
            />
          </div>

          {/* Body */}
          <div>
            <label htmlFor="webhookBody" className="block text-sm font-medium text-gray-700">
              Request Body
            </label>
            <textarea
              id="webhookBody"
              rows={3}
              value={form.webhookBody}
              onChange={(e) => setForm({ ...form, webhookBody: e.target.value })}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm font-mono text-xs"
              placeholder='{"key": "value"}'
            />
          </div>

          <hr className="border-gray-200" />
          <h3 className="text-lg font-medium text-gray-900">Execution Settings</h3>

          {/* Timeout */}
          <div>
            <label htmlFor="timeout" className="block text-sm font-medium text-gray-700">
              Timeout
            </label>
            <input
              type="text"
              id="timeout"
              value={form.timeout}
              onChange={(e) => setForm({ ...form, timeout: e.target.value })}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
              placeholder="5m"
            />
          </div>

          {/* Concurrency */}
          <div>
            <label htmlFor="concurrency" className="block text-sm font-medium text-gray-700">
              Concurrency Policy
            </label>
            <select
              id="concurrency"
              value={form.concurrency}
              onChange={(e) => setForm({ ...form, concurrency: e.target.value })}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            >
              <option value="forbid">Forbid (skip if already running)</option>
              <option value="allow">Allow (concurrent executions)</option>
              <option value="replace">Replace (cancel existing)</option>
            </select>
          </div>

          <hr className="border-gray-200" />
          <h3 className="text-lg font-medium text-gray-900">Retry Policy</h3>

          <div className="grid grid-cols-2 gap-4">
            {/* Max Attempts */}
            <div>
              <label htmlFor="maxAttempts" className="block text-sm font-medium text-gray-700">
                Max Attempts
              </label>
              <input
                type="number"
                id="maxAttempts"
                min={1}
                max={10}
                value={form.maxAttempts}
                onChange={(e) => setForm({ ...form, maxAttempts: parseInt(e.target.value) })}
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
              />
            </div>

            {/* Initial Interval */}
            <div>
              <label htmlFor="initialInterval" className="block text-sm font-medium text-gray-700">
                Initial Interval
              </label>
              <input
                type="text"
                id="initialInterval"
                value={form.initialInterval}
                onChange={(e) => setForm({ ...form, initialInterval: e.target.value })}
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                placeholder="1s"
              />
            </div>

            {/* Max Interval */}
            <div>
              <label htmlFor="maxInterval" className="block text-sm font-medium text-gray-700">
                Max Interval
              </label>
              <input
                type="text"
                id="maxInterval"
                value={form.maxInterval}
                onChange={(e) => setForm({ ...form, maxInterval: e.target.value })}
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                placeholder="1m"
              />
            </div>

            {/* Multiplier */}
            <div>
              <label htmlFor="multiplier" className="block text-sm font-medium text-gray-700">
                Backoff Multiplier
              </label>
              <input
                type="number"
                id="multiplier"
                min={1}
                max={10}
                step={0.1}
                value={form.multiplier}
                onChange={(e) => setForm({ ...form, multiplier: parseFloat(e.target.value) })}
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
              />
            </div>
          </div>

          {/* Enabled */}
          <div className="flex items-center">
            <input
              type="checkbox"
              id="enabled"
              checked={form.enabled}
              onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
              className="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
            />
            <label htmlFor="enabled" className="ml-2 block text-sm text-gray-900">
              Job enabled
            </label>
          </div>

          {/* Error message */}
          {mutation.isError && (
            <div className="text-red-600 text-sm">
              Error: {(mutation.error as Error).message}
            </div>
          )}
        </div>

        {/* Actions */}
        <div className="px-4 py-3 bg-gray-50 text-right sm:px-6 space-x-3">
          <Link
            to={`/jobs/${id}`}
            className="inline-flex justify-center rounded-md border border-gray-300 py-2 px-4 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50"
          >
            Cancel
          </Link>
          <button
            type="submit"
            disabled={mutation.isPending}
            className="inline-flex justify-center rounded-md border border-transparent bg-indigo-600 py-2 px-4 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 focus:outline-none disabled:opacity-50"
          >
            {mutation.isPending ? 'Saving...' : 'Save Changes'}
          </button>
        </div>
      </form>
    </div>
  )
}
