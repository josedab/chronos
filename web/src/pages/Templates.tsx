import { useState, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createJob } from '../api/client'
import type { Job } from '../types'
import { useListNavigation, useScrollIntoView } from '../hooks/useListNavigation'

interface JobTemplate {
  id: string
  name: string
  description: string
  category: string
  icon: string
  job: Partial<Job>
}

// Category metadata with icons and colors
const CATEGORY_META: Record<string, { icon: string; color: string; darkColor: string }> = {
  'Monitoring': { icon: '📊', color: 'bg-blue-100 text-blue-800', darkColor: 'dark:bg-blue-900 dark:text-blue-200' },
  'Operations': { icon: '⚙️', color: 'bg-gray-100 text-gray-800', darkColor: 'dark:bg-gray-700 dark:text-gray-200' },
  'Analytics': { icon: '📈', color: 'bg-green-100 text-green-800', darkColor: 'dark:bg-green-900 dark:text-green-200' },
  'Performance': { icon: '⚡', color: 'bg-yellow-100 text-yellow-800', darkColor: 'dark:bg-yellow-900 dark:text-yellow-200' },
  'Integration': { icon: '🔗', color: 'bg-purple-100 text-purple-800', darkColor: 'dark:bg-purple-900 dark:text-purple-200' },
  'Notifications': { icon: '🔔', color: 'bg-pink-100 text-pink-800', darkColor: 'dark:bg-pink-900 dark:text-pink-200' },
  'Security': { icon: '🛡️', color: 'bg-red-100 text-red-800', darkColor: 'dark:bg-red-900 dark:text-red-200' },
}

const JOB_TEMPLATES: JobTemplate[] = [
  {
    id: 'health-check',
    name: 'Health Check',
    description: 'Ping an endpoint every minute to verify availability',
    category: 'Monitoring',
    icon: '💓',
    job: {
      name: 'health-check',
      schedule: '* * * * *',
      timezone: 'UTC',
      webhook: {
        url: 'https://api.example.com/health',
        method: 'GET',
      },
      retry_policy: {
        max_attempts: 3,
        initial_interval: '5s',
        max_interval: '30s',
        multiplier: 2,
      },
      timeout: '30s',
    },
  },
  {
    id: 'daily-backup',
    name: 'Daily Backup',
    description: 'Trigger a backup job every day at 2 AM',
    category: 'Operations',
    icon: '💾',
    job: {
      name: 'daily-backup',
      schedule: '0 2 * * *',
      timezone: 'America/New_York',
      webhook: {
        url: 'https://api.example.com/backup/start',
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
      },
      retry_policy: {
        max_attempts: 3,
        initial_interval: '1m',
        max_interval: '10m',
        multiplier: 2,
      },
      timeout: '30m',
    },
  },
  {
    id: 'hourly-report',
    name: 'Hourly Report',
    description: 'Generate and send metrics report every hour',
    category: 'Analytics',
    icon: '📊',
    job: {
      name: 'hourly-report',
      schedule: '0 * * * *',
      timezone: 'UTC',
      webhook: {
        url: 'https://api.example.com/reports/generate',
        method: 'POST',
        body: JSON.stringify({ type: 'hourly', format: 'json' }),
      },
      timeout: '5m',
    },
  },
  {
    id: 'cache-warmup',
    name: 'Cache Warmup',
    description: 'Pre-populate cache with frequently accessed data',
    category: 'Performance',
    icon: '🔥',
    job: {
      name: 'cache-warmup',
      schedule: '*/15 * * * *',
      timezone: 'UTC',
      webhook: {
        url: 'https://api.example.com/cache/warmup',
        method: 'POST',
      },
      timeout: '10m',
    },
  },
  {
    id: 'cleanup-job',
    name: 'Cleanup Job',
    description: 'Remove expired data and temporary files daily',
    category: 'Operations',
    icon: '🧹',
    job: {
      name: 'cleanup-job',
      schedule: '0 3 * * *',
      timezone: 'UTC',
      webhook: {
        url: 'https://api.example.com/cleanup',
        method: 'POST',
        body: JSON.stringify({ olderThan: '30d' }),
      },
      timeout: '1h',
    },
  },
  {
    id: 'sync-job',
    name: 'Data Sync',
    description: 'Synchronize data between services every 5 minutes',
    category: 'Integration',
    icon: '🔄',
    job: {
      name: 'data-sync',
      schedule: '*/5 * * * *',
      timezone: 'UTC',
      webhook: {
        url: 'https://api.example.com/sync',
        method: 'POST',
      },
      retry_policy: {
        max_attempts: 5,
        initial_interval: '10s',
        max_interval: '2m',
        multiplier: 2,
      },
      timeout: '2m',
    },
  },
  {
    id: 'email-digest',
    name: 'Email Digest',
    description: 'Send daily email digest at 9 AM',
    category: 'Notifications',
    icon: '📧',
    job: {
      name: 'email-digest',
      schedule: '0 9 * * *',
      timezone: 'America/New_York',
      webhook: {
        url: 'https://api.example.com/email/digest',
        method: 'POST',
      },
      timeout: '5m',
    },
  },
  {
    id: 'ssl-check',
    name: 'SSL Certificate Check',
    description: 'Check SSL certificate expiration daily',
    category: 'Security',
    icon: '🔒',
    job: {
      name: 'ssl-check',
      schedule: '0 6 * * *',
      timezone: 'UTC',
      webhook: {
        url: 'https://api.example.com/security/ssl-check',
        method: 'POST',
      },
      timeout: '5m',
    },
  },
  {
    id: 'db-stats',
    name: 'Database Stats',
    description: 'Collect database statistics every 10 minutes',
    category: 'Monitoring',
    icon: '🗃️',
    job: {
      name: 'db-stats',
      schedule: '*/10 * * * *',
      timezone: 'UTC',
      webhook: {
        url: 'https://api.example.com/metrics/db',
        method: 'GET',
      },
      timeout: '1m',
    },
  },
  {
    id: 'weekly-report',
    name: 'Weekly Report',
    description: 'Generate weekly summary report on Mondays at 8 AM',
    category: 'Analytics',
    icon: '📈',
    job: {
      name: 'weekly-report',
      schedule: '0 8 * * 1',
      timezone: 'America/New_York',
      webhook: {
        url: 'https://api.example.com/reports/weekly',
        method: 'POST',
      },
      timeout: '15m',
    },
  },
  {
    id: 'api-key-rotation',
    name: 'API Key Rotation',
    description: 'Rotate API keys monthly on the 1st',
    category: 'Security',
    icon: '🔑',
    job: {
      name: 'api-key-rotation',
      schedule: '0 0 1 * *',
      timezone: 'UTC',
      webhook: {
        url: 'https://api.example.com/security/rotate-keys',
        method: 'POST',
      },
      timeout: '10m',
    },
  },
  {
    id: 'heartbeat',
    name: 'Heartbeat',
    description: 'Simple heartbeat ping every 30 seconds',
    category: 'Monitoring',
    icon: '❤️',
    job: {
      name: 'heartbeat',
      schedule: '@every 30s',
      webhook: {
        url: 'https://api.example.com/heartbeat',
        method: 'GET',
      },
      timeout: '10s',
    },
  },
]

const CATEGORIES = [...new Set(JOB_TEMPLATES.map((t) => t.category))].sort()

export default function Templates() {
  const [selectedCategory, setSelectedCategory] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedTemplate, setSelectedTemplate] = useState<JobTemplate | null>(null)
  const [editedConfig, setEditedConfig] = useState<string>('')
  const [configError, setConfigError] = useState<string | null>(null)
  const gridRef = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const createMutation = useMutation({
    mutationFn: createJob,
    onSuccess: (job) => {
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      navigate(`/jobs/${job.id}`)
    },
  })

  const filteredTemplates = JOB_TEMPLATES.filter((template) => {
    const matchesCategory = !selectedCategory || template.category === selectedCategory
    const matchesSearch =
      !searchQuery ||
      template.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      template.description.toLowerCase().includes(searchQuery.toLowerCase())
    return matchesCategory && matchesSearch
  })

  // Keyboard navigation for templates grid
  const { selectedIndex, setSelectedIndex } = useListNavigation({
    items: filteredTemplates,
    onEnter: (template) => handleUseTemplate(template),
    enabled: !selectedTemplate, // Disable when modal is open
  })
  useScrollIntoView(selectedIndex, gridRef)

  const handleUseTemplate = (template: JobTemplate) => {
    setSelectedTemplate(template)
    setEditedConfig(JSON.stringify(template.job, null, 2))
    setConfigError(null)
  }

  const handleConfigChange = (value: string) => {
    setEditedConfig(value)
    try {
      JSON.parse(value)
      setConfigError(null)
    } catch {
      setConfigError('Invalid JSON')
    }
  }

  const handleCreateJob = () => {
    if (selectedTemplate && !configError) {
      try {
        const config = JSON.parse(editedConfig)
        createMutation.mutate({ ...config, enabled: false })
      } catch {
        setConfigError('Invalid JSON configuration')
      }
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Job Templates</h2>
          <p className="text-gray-500 dark:text-gray-400 text-sm mt-1">
            Quick-start templates for common job patterns. Press j/k to navigate, Enter to select.
          </p>
        </div>
      </div>

      {/* Search and filter */}
      <div className="flex flex-col sm:flex-row gap-4">
        <input
          type="text"
          placeholder="Search templates..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="flex-1 px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
        />
        <div className="flex gap-2 overflow-x-auto pb-2">
          <button
            onClick={() => setSelectedCategory(null)}
            className={`px-3 py-1 text-sm rounded-full whitespace-nowrap flex items-center gap-1 ${
              !selectedCategory
                ? 'bg-indigo-600 text-white'
                : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
            }`}
          >
            All ({JOB_TEMPLATES.length})
          </button>
          {CATEGORIES.map((category) => {
            const meta = CATEGORY_META[category] || { icon: '📁', color: 'bg-gray-100 text-gray-800', darkColor: '' }
            const count = JOB_TEMPLATES.filter(t => t.category === category).length
            return (
              <button
                key={category}
                onClick={() => setSelectedCategory(category)}
                className={`px-3 py-1 text-sm rounded-full whitespace-nowrap flex items-center gap-1 ${
                  selectedCategory === category
                    ? 'bg-indigo-600 text-white'
                    : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
                }`}
              >
                <span>{meta.icon}</span>
                {category} ({count})
              </button>
            )
          })}
        </div>
      </div>

      {/* Templates grid */}
      <div ref={gridRef} className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {filteredTemplates.map((template, index) => {
          const meta = CATEGORY_META[template.category] || { icon: '📁', color: 'bg-gray-100 text-gray-800', darkColor: '' }
          return (
            <div
              key={template.id}
              data-index={index}
              onClick={() => setSelectedIndex(index)}
              onDoubleClick={() => handleUseTemplate(template)}
              className={`bg-white dark:bg-gray-800 rounded-lg shadow hover:shadow-md transition-all border overflow-hidden cursor-pointer ${
                selectedIndex === index 
                  ? 'border-indigo-500 ring-2 ring-indigo-500/50' 
                  : 'border-gray-200 dark:border-gray-700'
              }`}
            >
              <div className="p-4">
                <div className="flex items-start gap-3">
                  <span className="text-2xl">{template.icon}</span>
                  <div className="flex-1 min-w-0">
                    <h3 className="font-medium text-gray-900 dark:text-white">{template.name}</h3>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">{template.description}</p>
                  </div>
                </div>
                <div className="mt-4 flex items-center justify-between">
                  <span className={`text-xs px-2 py-1 rounded ${meta.color} ${meta.darkColor}`}>
                    {template.category}
                  </span>
                  <button
                    onClick={(e) => { e.stopPropagation(); handleUseTemplate(template) }}
                    className="text-sm text-indigo-600 dark:text-indigo-400 hover:text-indigo-800 dark:hover:text-indigo-300 font-medium"
                  >
                    Use Template →
                  </button>
                </div>
              </div>
              <div className="bg-gray-50 dark:bg-gray-900 px-4 py-2 border-t border-gray-200 dark:border-gray-700 flex justify-between items-center">
                <code className="text-xs text-gray-500 dark:text-gray-400 font-mono">{template.job.schedule}</code>
                {template.job.timeout && (
                  <span className="text-xs text-gray-400 dark:text-gray-500">timeout: {String(template.job.timeout)}</span>
                )}
              </div>
            </div>
          )
        })}
      </div>

      {filteredTemplates.length === 0 && (
        <div className="text-center py-12 text-gray-500 dark:text-gray-400">
          No templates found matching your criteria
        </div>
      )}

      {/* Template preview modal */}
      {selectedTemplate && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-3xl w-full max-h-[90vh] overflow-hidden">
            <div className="p-6 border-b border-gray-200 dark:border-gray-700">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <span className="text-3xl">{selectedTemplate.icon}</span>
                  <div>
                    <h3 className="text-xl font-semibold text-gray-900 dark:text-white">
                      {selectedTemplate.name}
                    </h3>
                    <p className="text-gray-500 dark:text-gray-400">{selectedTemplate.description}</p>
                  </div>
                </div>
                <button 
                  onClick={() => setSelectedTemplate(null)}
                  className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                >
                  ✕
                </button>
              </div>
            </div>
            <div className="p-6 overflow-y-auto max-h-[50vh]">
              <div className="flex items-center justify-between mb-2">
                <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300">Configuration (editable)</h4>
                {configError && (
                  <span className="text-sm text-red-500">{configError}</span>
                )}
              </div>
              <textarea
                value={editedConfig}
                onChange={(e) => handleConfigChange(e.target.value)}
                className={`w-full h-64 bg-gray-900 text-gray-100 rounded-lg p-4 font-mono text-sm resize-none focus:outline-none focus:ring-2 ${
                  configError ? 'focus:ring-red-500 border border-red-500' : 'focus:ring-indigo-500'
                }`}
                spellCheck={false}
              />
              <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
                Edit the JSON configuration above to customize before creating the job.
              </p>
            </div>
            <div className="p-4 bg-gray-50 dark:bg-gray-900 border-t border-gray-200 dark:border-gray-700 flex justify-between items-center">
              <div className="text-sm text-gray-500 dark:text-gray-400">
                <span className={`inline-block w-2 h-2 rounded-full mr-2 ${configError ? 'bg-red-500' : 'bg-green-500'}`}></span>
                {configError ? 'Invalid JSON' : 'Valid configuration'}
              </div>
              <div className="flex gap-3">
                <button
                  onClick={() => setSelectedTemplate(null)}
                  className="px-4 py-2 text-gray-700 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-100 dark:hover:bg-gray-700"
                >
                  Cancel
                </button>
                <button
                  onClick={() => {
                    try {
                      const config = JSON.parse(editedConfig)
                      navigate('/jobs/new', { state: { template: config } })
                    } catch {
                      setConfigError('Invalid JSON')
                    }
                  }}
                  className="px-4 py-2 text-indigo-600 dark:text-indigo-400 border border-indigo-600 dark:border-indigo-400 rounded-md hover:bg-indigo-50 dark:hover:bg-indigo-900/20"
                >
                  Customize in Form
                </button>
                <button
                  onClick={handleCreateJob}
                  disabled={createMutation.isPending || !!configError}
                  className="px-4 py-2 bg-indigo-600 text-white rounded-md hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {createMutation.isPending ? 'Creating...' : 'Create Job Now'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
