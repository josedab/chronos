import { useMemo } from 'react'
import type { Execution } from '../types'

interface DurationChartProps {
  executions: Execution[]
  className?: string
}

export default function DurationChart({ executions, className = '' }: DurationChartProps) {
  const chartData = useMemo(() => {
    // Filter to successful executions with duration data, sort by time
    const withDuration = executions
      .filter(e => e.duration && e.duration > 0)
      .sort((a, b) => new Date(a.started_at).getTime() - new Date(b.started_at).getTime())
      .slice(-20) // Last 20 executions

    if (withDuration.length === 0) return null

    // Convert duration from nanoseconds to milliseconds
    const durations = withDuration.map(e => (e.duration || 0) / 1e6)
    const maxDuration = Math.max(...durations)
    const minDuration = Math.min(...durations)
    const avgDuration = durations.reduce((a, b) => a + b, 0) / durations.length

    return {
      executions: withDuration,
      durations,
      maxDuration,
      minDuration,
      avgDuration,
    }
  }, [executions])

  if (!chartData || chartData.durations.length < 2) {
    return (
      <div className={`bg-white dark:bg-gray-800 rounded-lg shadow p-4 ${className}`}>
        <h3 className="text-sm font-medium text-gray-900 dark:text-white mb-2">Execution Duration Trend</h3>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Not enough data to display chart. Need at least 2 completed executions.
        </p>
      </div>
    )
  }

  const { durations, maxDuration, minDuration, avgDuration, executions: chartExecutions } = chartData
  const chartHeight = 120
  const chartWidth = 100 // percentage

  // Build SVG path for the line chart
  const points = durations.map((d, i) => {
    const x = (i / (durations.length - 1)) * chartWidth
    const y = chartHeight - (d / maxDuration) * (chartHeight - 20)
    return { x, y, duration: d, execution: chartExecutions[i] }
  })

  const linePath = points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`).join(' ')
  
  // Area fill path
  const areaPath = `${linePath} L ${chartWidth} ${chartHeight} L 0 ${chartHeight} Z`

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms.toFixed(0)}ms`
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
    return `${(ms / 60000).toFixed(1)}m`
  }

  return (
    <div className={`bg-white dark:bg-gray-800 rounded-lg shadow ${className}`}>
      <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <h3 className="text-sm font-medium text-gray-900 dark:text-white flex items-center gap-2">
          <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
          </svg>
          Execution Duration Trend
        </h3>
        <span className="text-xs text-gray-500 dark:text-gray-400">
          Last {durations.length} runs
        </span>
      </div>

      <div className="p-4">
        {/* Stats row */}
        <div className="flex justify-between text-xs mb-4">
          <div className="text-center">
            <div className="text-gray-500 dark:text-gray-400">Min</div>
            <div className="font-semibold text-green-600">{formatDuration(minDuration)}</div>
          </div>
          <div className="text-center">
            <div className="text-gray-500 dark:text-gray-400">Avg</div>
            <div className="font-semibold text-blue-600">{formatDuration(avgDuration)}</div>
          </div>
          <div className="text-center">
            <div className="text-gray-500 dark:text-gray-400">Max</div>
            <div className="font-semibold text-red-600">{formatDuration(maxDuration)}</div>
          </div>
        </div>

        {/* Chart */}
        <div className="relative h-32">
          <svg 
            viewBox={`0 0 ${chartWidth} ${chartHeight}`} 
            className="w-full h-full"
            preserveAspectRatio="none"
          >
            {/* Grid lines */}
            <line x1="0" y1={chartHeight * 0.25} x2={chartWidth} y2={chartHeight * 0.25} 
                  stroke="currentColor" className="text-gray-200 dark:text-gray-700" strokeDasharray="2,2" />
            <line x1="0" y1={chartHeight * 0.5} x2={chartWidth} y2={chartHeight * 0.5} 
                  stroke="currentColor" className="text-gray-200 dark:text-gray-700" strokeDasharray="2,2" />
            <line x1="0" y1={chartHeight * 0.75} x2={chartWidth} y2={chartHeight * 0.75} 
                  stroke="currentColor" className="text-gray-200 dark:text-gray-700" strokeDasharray="2,2" />

            {/* Average line */}
            {avgDuration > 0 && (
              <line 
                x1="0" 
                y1={chartHeight - (avgDuration / maxDuration) * (chartHeight - 20)} 
                x2={chartWidth} 
                y2={chartHeight - (avgDuration / maxDuration) * (chartHeight - 20)}
                stroke="currentColor" 
                className="text-blue-400"
                strokeDasharray="4,4"
                strokeWidth="1"
              />
            )}

            {/* Area fill */}
            <path 
              d={areaPath} 
              fill="url(#gradient)" 
              className="opacity-30"
            />
            
            {/* Line */}
            <path 
              d={linePath} 
              fill="none" 
              stroke="currentColor" 
              className="text-indigo-500"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />

            {/* Data points */}
            {points.map((p, i) => (
              <g key={i}>
                <circle 
                  cx={p.x} 
                  cy={p.y} 
                  r="3" 
                  fill="currentColor" 
                  className={`${
                    p.execution.status === 'failed' 
                      ? 'text-red-500' 
                      : 'text-indigo-500'
                  }`}
                />
                {/* Hover area for tooltip */}
                <circle 
                  cx={p.x} 
                  cy={p.y} 
                  r="8" 
                  fill="transparent"
                  className="cursor-pointer"
                >
                  <title>
                    {formatDuration(p.duration)} - {new Date(p.execution.started_at).toLocaleString()}
                  </title>
                </circle>
              </g>
            ))}

            {/* Gradient definition */}
            <defs>
              <linearGradient id="gradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="rgb(99, 102, 241)" stopOpacity="0.4" />
                <stop offset="100%" stopColor="rgb(99, 102, 241)" stopOpacity="0" />
              </linearGradient>
            </defs>
          </svg>
        </div>

        {/* Time axis labels */}
        <div className="flex justify-between text-xs text-gray-400 dark:text-gray-500 mt-1">
          <span>{new Date(chartExecutions[0].started_at).toLocaleDateString()}</span>
          <span>{new Date(chartExecutions[chartExecutions.length - 1].started_at).toLocaleDateString()}</span>
        </div>
      </div>
    </div>
  )
}
