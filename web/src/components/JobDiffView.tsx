import { useMemo } from 'react'
import type { Job } from '../types'

interface JobDiffViewProps {
  original: Partial<Job>
  modified: Partial<Job>
  onClose?: () => void
}

type ChangeType = 'added' | 'removed' | 'modified' | 'unchanged'

interface DiffItem {
  path: string
  label: string
  oldValue: string | undefined
  newValue: string | undefined
  changeType: ChangeType
}

function stringifyValue(value: unknown): string {
  if (value === undefined || value === null) return ''
  if (typeof value === 'object') return JSON.stringify(value, null, 2)
  return String(value)
}

function getNestedValue(obj: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce((acc: unknown, key) => {
    if (acc && typeof acc === 'object') {
      return (acc as Record<string, unknown>)[key]
    }
    return undefined
  }, obj)
}

const FIELD_LABELS: Record<string, string> = {
  name: 'Name',
  description: 'Description',
  schedule: 'Schedule',
  timezone: 'Timezone',
  'webhook.url': 'Webhook URL',
  'webhook.method': 'HTTP Method',
  'webhook.headers': 'Headers',
  'webhook.body': 'Request Body',
  timeout: 'Timeout',
  concurrency: 'Concurrency Policy',
  enabled: 'Enabled',
  'retry_policy.max_attempts': 'Max Attempts',
  'retry_policy.initial_interval': 'Initial Interval',
  'retry_policy.max_interval': 'Max Interval',
  'retry_policy.multiplier': 'Backoff Multiplier',
}

const TRACKED_FIELDS = Object.keys(FIELD_LABELS)

export default function JobDiffView({ original, modified, onClose }: JobDiffViewProps) {
  const diffs = useMemo<DiffItem[]>(() => {
    const result: DiffItem[] = []
    
    for (const path of TRACKED_FIELDS) {
      const oldValue = stringifyValue(getNestedValue(original as Record<string, unknown>, path))
      const newValue = stringifyValue(getNestedValue(modified as Record<string, unknown>, path))
      
      if (oldValue === newValue) continue // Skip unchanged fields
      
      let changeType: ChangeType = 'unchanged'
      if (!oldValue && newValue) changeType = 'added'
      else if (oldValue && !newValue) changeType = 'removed'
      else if (oldValue !== newValue) changeType = 'modified'
      
      result.push({
        path,
        label: FIELD_LABELS[path] || path,
        oldValue: oldValue || undefined,
        newValue: newValue || undefined,
        changeType,
      })
    }
    
    return result
  }, [original, modified])

  const hasChanges = diffs.length > 0

  const getChangeColor = (type: ChangeType) => {
    switch (type) {
      case 'added': return 'bg-green-50 border-green-200'
      case 'removed': return 'bg-red-50 border-red-200'
      case 'modified': return 'bg-yellow-50 border-yellow-200'
      default: return 'bg-gray-50 border-gray-200'
    }
  }

  const getChangeIcon = (type: ChangeType) => {
    switch (type) {
      case 'added': return '+'
      case 'removed': return '−'
      case 'modified': return '~'
      default: return ''
    }
  }

  const getChangeLabel = (type: ChangeType) => {
    switch (type) {
      case 'added': return 'Added'
      case 'removed': return 'Removed'
      case 'modified': return 'Modified'
      default: return ''
    }
  }

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow-lg">
      <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
        <h3 className="text-lg font-medium text-gray-900 dark:text-white">
          Review Changes
        </h3>
        {onClose && (
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            ✕
          </button>
        )}
      </div>

      <div className="p-4">
        {!hasChanges ? (
          <div className="text-center py-8 text-gray-500">
            No changes detected
          </div>
        ) : (
          <>
            {/* Summary */}
            <div className="mb-4 flex items-center gap-4 text-sm">
              <span className="text-gray-500">{diffs.length} change{diffs.length !== 1 ? 's' : ''}</span>
              <div className="flex items-center gap-3">
                {diffs.filter(d => d.changeType === 'added').length > 0 && (
                  <span className="text-green-600">
                    +{diffs.filter(d => d.changeType === 'added').length} added
                  </span>
                )}
                {diffs.filter(d => d.changeType === 'modified').length > 0 && (
                  <span className="text-yellow-600">
                    ~{diffs.filter(d => d.changeType === 'modified').length} modified
                  </span>
                )}
                {diffs.filter(d => d.changeType === 'removed').length > 0 && (
                  <span className="text-red-600">
                    −{diffs.filter(d => d.changeType === 'removed').length} removed
                  </span>
                )}
              </div>
            </div>

            {/* Diff List */}
            <div className="space-y-3">
              {diffs.map((diff) => (
                <div
                  key={diff.path}
                  className={`rounded-lg border p-3 ${getChangeColor(diff.changeType)}`}
                >
                  <div className="flex items-center gap-2 mb-2">
                    <span className={`w-5 h-5 rounded text-xs font-bold flex items-center justify-center ${
                      diff.changeType === 'added' ? 'bg-green-500 text-white' :
                      diff.changeType === 'removed' ? 'bg-red-500 text-white' :
                      'bg-yellow-500 text-white'
                    }`}>
                      {getChangeIcon(diff.changeType)}
                    </span>
                    <span className="font-medium text-gray-900 dark:text-gray-800">
                      {diff.label}
                    </span>
                    <span className="text-xs text-gray-500">
                      ({getChangeLabel(diff.changeType)})
                    </span>
                  </div>
                  
                  <div className="space-y-1 text-sm">
                    {diff.oldValue && (
                      <div className="flex gap-2">
                        <span className="text-red-600 font-mono">−</span>
                        <pre className="flex-1 bg-red-100 dark:bg-red-900/20 px-2 py-1 rounded text-red-800 dark:text-red-300 overflow-x-auto whitespace-pre-wrap">
                          {diff.oldValue}
                        </pre>
                      </div>
                    )}
                    {diff.newValue && (
                      <div className="flex gap-2">
                        <span className="text-green-600 font-mono">+</span>
                        <pre className="flex-1 bg-green-100 dark:bg-green-900/20 px-2 py-1 rounded text-green-800 dark:text-green-300 overflow-x-auto whitespace-pre-wrap">
                          {diff.newValue}
                        </pre>
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </>
        )}
      </div>
    </div>
  )
}
