import { useState, useCallback } from 'react'
import type { Execution } from '../types'
import { triggerJob, replayExecution } from '../api/client'

interface OneClickRetryProps {
  execution: Execution
  onRetryComplete?: (newExecution: Execution) => void
}

export default function OneClickRetry({ execution, onRetryComplete }: OneClickRetryProps) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [retried, setRetried] = useState(false)

  const handleRetry = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      let newExec: Execution
      // Try replay first, fall back to trigger
      try {
        newExec = await replayExecution(execution.job_id, execution.id)
      } catch {
        newExec = await triggerJob(execution.job_id)
      }
      setRetried(true)
      onRetryComplete?.(newExec)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Retry failed')
    } finally {
      setLoading(false)
    }
  }, [execution, onRetryComplete])

  if (execution.status !== 'failed') {
    return null
  }

  if (retried) {
    return (
      <span className="inline-flex items-center text-xs text-green-600 dark:text-green-400">
        <svg className="w-3.5 h-3.5 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
        </svg>
        Retried
      </span>
    )
  }

  return (
    <div className="inline-flex items-center gap-1">
      <button
        onClick={handleRetry}
        disabled={loading}
        className="inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium text-white bg-red-500 hover:bg-red-600 rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        title="Retry this execution"
      >
        {loading ? (
          <svg className="animate-spin w-3 h-3" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
          </svg>
        ) : (
          <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        )}
        Retry
      </button>
      {error && <span className="text-xs text-red-500">{error}</span>}
    </div>
  )
}
