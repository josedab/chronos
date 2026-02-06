import { useMemo } from 'react'
import type { RetryPolicy } from '../types'

interface RetryPolicyPreviewProps {
  policy: RetryPolicy
  compact?: boolean
}

function parseDuration(duration: string): number {
  const match = duration.match(/^(\d+(?:\.\d+)?)(s|m|h|ms)$/)
  if (!match) return 1000
  
  const value = parseFloat(match[1])
  const unit = match[2]
  
  switch (unit) {
    case 'ms': return value
    case 's': return value * 1000
    case 'm': return value * 60 * 1000
    case 'h': return value * 60 * 60 * 1000
    default: return value * 1000
  }
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  if (ms < 3600000) return `${(ms / 60000).toFixed(1)}m`
  return `${(ms / 3600000).toFixed(1)}h`
}

export default function RetryPolicyPreview({ policy, compact = false }: RetryPolicyPreviewProps) {
  const { attempts, maxDelay } = useMemo(() => {
    const initialMs = parseDuration(policy.initial_interval)
    const maxMs = parseDuration(policy.max_interval)
    const attempts: { attempt: number; delay: number; cumulative: number }[] = []
    
    let cumulative = 0
    for (let i = 1; i <= policy.max_attempts; i++) {
      const delay = i === 1 ? 0 : Math.min(initialMs * Math.pow(policy.multiplier, i - 2), maxMs)
      cumulative += delay
      attempts.push({ attempt: i, delay, cumulative })
    }
    
    return { 
      attempts, 
      maxDelay: Math.max(...attempts.map(a => a.delay)),
    }
  }, [policy])

  const totalTime = attempts[attempts.length - 1]?.cumulative || 0

  if (compact) {
    return (
      <div className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
        <span>{policy.max_attempts} attempts</span>
        <span>•</span>
        <span className="font-mono">{policy.initial_interval}</span>
        <span>→</span>
        <span className="font-mono">{policy.max_interval}</span>
        <span>•</span>
        <span>{policy.multiplier}x</span>
      </div>
    )
  }

  return (
    <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-4">
      <div className="flex items-center justify-between mb-3">
        <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300">Retry Policy Preview</h4>
        <span className="text-xs text-gray-500">
          Total max time: {formatDuration(totalTime)}
        </span>
      </div>

      {/* Visual Timeline */}
      <div className="mb-4">
        <div className="flex items-end gap-1 h-16">
          {attempts.map((attempt, i) => {
            const height = maxDelay > 0 ? (attempt.delay / maxDelay) * 100 : (i === 0 ? 100 : 0)
            return (
              <div
                key={attempt.attempt}
                className="flex-1 flex flex-col items-center"
              >
                <div
                  className={`w-full rounded-t transition-all ${
                    i === 0 
                      ? 'bg-indigo-500' 
                      : i === attempts.length - 1 
                        ? 'bg-red-400'
                        : 'bg-yellow-400'
                  }`}
                  style={{ height: `${Math.max(height, 10)}%` }}
                  title={`Attempt ${attempt.attempt}: ${i === 0 ? 'Immediate' : `Wait ${formatDuration(attempt.delay)}`}`}
                />
                <span className="text-xs text-gray-500 mt-1">{attempt.attempt}</span>
              </div>
            )
          })}
        </div>
        <div className="flex justify-between text-xs text-gray-500 mt-1">
          <span>Attempt</span>
          <span>Wait before attempt</span>
        </div>
      </div>

      {/* Detailed Breakdown */}
      <div className="space-y-2">
        {attempts.map((attempt, i) => (
          <div 
            key={attempt.attempt}
            className="flex items-center gap-3 text-sm"
          >
            <span className={`w-6 h-6 rounded-full flex items-center justify-center text-white text-xs font-medium ${
              i === 0 
                ? 'bg-indigo-500' 
                : i === attempts.length - 1 
                  ? 'bg-red-400'
                  : 'bg-yellow-400'
            }`}>
              {attempt.attempt}
            </span>
            <div className="flex-1">
              <span className="text-gray-700 dark:text-gray-300">
                {i === 0 ? 'Execute immediately' : `Wait ${formatDuration(attempt.delay)}`}
              </span>
              {i > 0 && (
                <span className="text-gray-400 text-xs ml-2">
                  ({formatDuration(attempt.cumulative)} total)
                </span>
              )}
            </div>
            {i === attempts.length - 1 && (
              <span className="text-xs text-red-500">Final attempt</span>
            )}
          </div>
        ))}
      </div>

      {/* Formula Explanation */}
      <div className="mt-4 pt-3 border-t border-gray-200 dark:border-gray-700">
        <p className="text-xs text-gray-500">
          Delay formula: <code className="bg-gray-200 dark:bg-gray-700 px-1 rounded">
            min({policy.initial_interval} × {policy.multiplier}^(attempt-1), {policy.max_interval})
          </code>
        </p>
      </div>
    </div>
  )
}
