import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useNavigate, Link } from 'react-router-dom'
import { createJob } from '../api/client'

export default function JobCreate() {
  const navigate = useNavigate()

  const [form, setForm] = useState({
    name: '',
    description: '',
    schedule: '* * * * *',
    timezone: '',
    webhookUrl: '',
    webhookMethod: 'GET',
    timeout: '5m',
    enabled: true,
    maxAttempts: 3,
  })

  const mutation = useMutation({
    mutationFn: createJob,
    onSuccess: (job) => navigate(`/jobs/${job.id}`),
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    mutation.mutate({
      name: form.name,
      description: form.description,
      schedule: form.schedule,
      timezone: form.timezone || undefined,
      webhook: {
        url: form.webhookUrl,
        method: form.webhookMethod,
      },
      timeout: form.timeout,
      retry_policy: {
        max_attempts: form.maxAttempts,
        initial_interval: '1s',
        max_interval: '1m',
        multiplier: 2.0,
      },
      enabled: form.enabled,
    })
  }

  return (
    <div className="max-w-2xl mx-auto">
      <h2 className="text-2xl font-bold text-gray-900 mb-6">Create New Job</h2>

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
              placeholder="daily-report"
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
              placeholder="Generate daily analytics report"
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
              placeholder="0 9 * * *"
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

          {/* Webhook URL */}
          <div>
            <label htmlFor="webhookUrl" className="block text-sm font-medium text-gray-700">
              Webhook URL *
            </label>
            <input
              type="url"
              id="webhookUrl"
              required
              value={form.webhookUrl}
              onChange={(e) => setForm({ ...form, webhookUrl: e.target.value })}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
              placeholder="https://api.example.com/webhook"
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
            </select>
          </div>

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

          {/* Max Attempts */}
          <div>
            <label htmlFor="maxAttempts" className="block text-sm font-medium text-gray-700">
              Max Retry Attempts
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
              Enable job immediately
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
            to="/jobs"
            className="inline-flex justify-center rounded-md border border-gray-300 py-2 px-4 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50"
          >
            Cancel
          </Link>
          <button
            type="submit"
            disabled={mutation.isPending}
            className="inline-flex justify-center rounded-md border border-transparent bg-indigo-600 py-2 px-4 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 focus:outline-none disabled:opacity-50"
          >
            {mutation.isPending ? 'Creating...' : 'Create Job'}
          </button>
        </div>
      </form>
    </div>
  )
}
