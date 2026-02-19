import { useMemo } from 'react'

interface SLOBurnRateProps {
  /** SLO target as a decimal (e.g., 0.999 for 99.9%) */
  target: number
  /** Total executions in the window */
  totalExecutions: number
  /** Failed executions in the window */
  failedExecutions: number
  /** SLO window label */
  windowLabel?: string
}

export default function SLOBurnRate({
  target,
  totalExecutions,
  failedExecutions,
  windowLabel = '30-day',
}: SLOBurnRateProps) {
  const { currentSLO, errorBudgetTotal, errorBudgetRemaining, burnRate, isHealthy } = useMemo(() => {
    if (totalExecutions === 0) {
      return {
        currentSLO: 1,
        errorBudgetTotal: 0,
        errorBudgetRemaining: 0,
        burnRate: 0,
        isHealthy: true,
      }
    }

    const successRate = (totalExecutions - failedExecutions) / totalExecutions
    const errorBudgetTotal = Math.floor(totalExecutions * (1 - target))
    const errorBudgetRemaining = Math.max(0, errorBudgetTotal - failedExecutions)
    const burnRate = errorBudgetTotal > 0 ? failedExecutions / errorBudgetTotal : 0

    return {
      currentSLO: successRate,
      errorBudgetTotal,
      errorBudgetRemaining,
      burnRate,
      isHealthy: successRate >= target,
    }
  }, [target, totalExecutions, failedExecutions])

  const pctUsed = errorBudgetTotal > 0
    ? Math.min(100, ((errorBudgetTotal - errorBudgetRemaining) / errorBudgetTotal) * 100)
    : 0

  const barColor = pctUsed < 50
    ? 'bg-green-500'
    : pctUsed < 80
    ? 'bg-yellow-500'
    : 'bg-red-500'

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200">
          SLO: {(target * 100).toFixed(1)}%
        </h3>
        <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${
          isHealthy
            ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
            : 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
        }`}>
          {isHealthy ? '● Healthy' : '● At Risk'}
        </span>
      </div>

      {/* Current SLO display */}
      <div className="text-2xl font-bold text-gray-900 dark:text-white mb-1">
        {(currentSLO * 100).toFixed(2)}%
      </div>
      <p className="text-xs text-gray-500 dark:text-gray-400 mb-3">
        {totalExecutions.toLocaleString()} executions ({windowLabel})
      </p>

      {/* Error budget bar */}
      <div className="mb-2">
        <div className="flex justify-between text-xs text-gray-500 mb-1">
          <span>Error Budget</span>
          <span>{errorBudgetRemaining} / {errorBudgetTotal} remaining</span>
        </div>
        <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
          <div
            className={`h-2 rounded-full transition-all duration-500 ${barColor}`}
            style={{ width: `${pctUsed}%` }}
          />
        </div>
      </div>

      {/* Burn rate */}
      <div className="flex items-center justify-between text-xs">
        <span className="text-gray-500 dark:text-gray-400">Burn Rate</span>
        <span className={`font-medium ${
          burnRate <= 1 ? 'text-green-600' : burnRate <= 2 ? 'text-yellow-600' : 'text-red-600'
        }`}>
          {burnRate.toFixed(2)}x
          {burnRate > 1 && ' ⚠'}
        </span>
      </div>
    </div>
  )
}
