import { useState, useEffect } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useNavigate, Link, useSearchParams } from 'react-router-dom'
import { createJob, getJob } from '../api/client'
import CronBuilder from '../components/CronBuilder'
import CronExplainer from '../components/CronExplainer'
import TimezonePicker from '../components/TimezonePicker'
import WebhookTester from '../components/WebhookTester'

export default function JobCreate() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const duplicateId = searchParams.get('duplicate')
  const [showWebhookTester, setShowWebhookTester] = useState(false)

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

  // Fetch job to duplicate if duplicateId is provided
  const { data: sourceJob } = useQuery({
    queryKey: ['job', duplicateId],
    queryFn: () => getJob(duplicateId!),
    enabled: !!duplicateId,
  })

  // Populate form when source job loads (for duplication)
  useEffect(() => {
    if (sourceJob) {
      setForm({
        name: `${sourceJob.name}-copy`,
        description: sourceJob.description || '',
        schedule: sourceJob.schedule,
        timezone: sourceJob.timezone || '',
        webhookUrl: sourceJob.webhook.url,
        webhookMethod: sourceJob.webhook.method || 'GET',
        timeout: sourceJob.timeout || '5m',
        enabled: false, // Duplicated jobs start disabled for safety
        maxAttempts: sourceJob.retry_policy?.max_attempts || 3,
      })
    }
  }, [sourceJob])

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
      <h2 className="text-2xl font-bold text-gray-900 dark:text-white mb-6">
        {duplicateId ? 'Duplicate Job' : 'Create New Job'}
      </h2>
      
      {duplicateId && sourceJob && (
        <div className="mb-4 p-3 bg-purple-50 dark:bg-purple-900/20 border border-purple-200 dark:border-purple-800 rounded-lg flex items-center gap-2">
          <svg className="w-5 h-5 text-purple-600 dark:text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
          </svg>
          <span className="text-sm text-purple-700 dark:text-purple-300">
            Duplicating from <strong>{sourceJob.name}</strong>. Job will be created disabled.
          </span>
        </div>
      )}

      <form onSubmit={handleSubmit} className="bg-white dark:bg-gray-800 shadow rounded-lg">
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
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Schedule *
            </label>
            <CronBuilder
              value={form.schedule}
              onChange={(schedule) => setForm({ ...form, schedule })}
            />
            <CronExplainer expression={form.schedule} className="mt-3" />
          </div>

          {/* Timezone */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Timezone
            </label>
            <TimezonePicker
              value={form.timezone}
              onChange={(timezone) => setForm({ ...form, timezone })}
            />
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Leave empty for UTC
            </p>
          </div>

          {/* Webhook URL */}
          <div>
            <label htmlFor="webhookUrl" className="block text-sm font-medium text-gray-700">
              Webhook URL *
            </label>
            <div className="mt-1 flex gap-2">
              <input
                type="url"
                id="webhookUrl"
                required
                value={form.webhookUrl}
                onChange={(e) => setForm({ ...form, webhookUrl: e.target.value })}
                className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                placeholder="https://api.example.com/webhook"
              />
              <button
                type="button"
                onClick={() => setShowWebhookTester(true)}
                disabled={!form.webhookUrl}
                className="px-3 py-2 text-sm bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1 whitespace-nowrap"
                title="Test webhook URL"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
                Test
              </button>
            </div>
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

      {/* Webhook Tester Modal */}
      {showWebhookTester && (
        <WebhookTester
          url={form.webhookUrl}
          method={form.webhookMethod}
          onClose={() => setShowWebhookTester(false)}
        />
      )}
    </div>
  )
}
