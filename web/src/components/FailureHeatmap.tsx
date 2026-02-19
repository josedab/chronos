import React, { useMemo } from 'react'
import type { Execution } from '../types'

interface FailureHeatmapProps {
  executions: Execution[]
}

const DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const HOURS = Array.from({ length: 24 }, (_, i) => i)

function getColor(rate: number): string {
  if (rate === 0) return 'bg-green-100 dark:bg-green-900/30'
  if (rate < 0.1) return 'bg-yellow-200 dark:bg-yellow-800/40'
  if (rate < 0.25) return 'bg-orange-300 dark:bg-orange-700/50'
  if (rate < 0.5) return 'bg-red-300 dark:bg-red-700/60'
  return 'bg-red-500 dark:bg-red-600'
}

export default function FailureHeatmap({ executions }: FailureHeatmapProps) {
  const heatmapData = useMemo(() => {
    // Initialize grid: [day][hour] = { total, failed }
    const grid: { total: number; failed: number }[][] = Array.from({ length: 7 }, () =>
      Array.from({ length: 24 }, () => ({ total: 0, failed: 0 }))
    )

    for (const exec of executions) {
      const date = new Date(exec.started_at)
      const day = date.getDay()
      const hour = date.getHours()
      grid[day][hour].total++
      if (exec.status === 'failed') {
        grid[day][hour].failed++
      }
    }

    return grid
  }, [executions])

  const maxTotal = useMemo(() => {
    let max = 0
    for (const day of heatmapData) {
      for (const cell of day) {
        if (cell.total > max) max = cell.total
      }
    }
    return max
  }, [heatmapData])

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4">
      <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3">
        Failure Heatmap
        <span className="text-xs font-normal text-gray-500 ml-2">(Day × Hour)</span>
      </h3>

      <div className="overflow-x-auto">
        <div className="inline-grid gap-px" style={{ gridTemplateColumns: `60px repeat(24, 1fr)` }}>
          {/* Hour headers */}
          <div />
          {HOURS.map(h => (
            <div key={h} className="text-[10px] text-gray-400 text-center px-0.5">
              {h === 0 ? '12a' : h < 12 ? `${h}a` : h === 12 ? '12p' : `${h - 12}p`}
            </div>
          ))}

          {/* Data rows */}
          {DAYS.map((day, dayIdx) => (
            <React.Fragment key={dayIdx}>
              <div className="text-xs text-gray-500 dark:text-gray-400 pr-2 flex items-center">
                {day}
              </div>
              {HOURS.map(hour => {
                const cell = heatmapData[dayIdx][hour]
                const rate = cell.total > 0 ? cell.failed / cell.total : 0
                return (
                  <div
                    key={`${dayIdx}-${hour}`}
                    className={`w-full aspect-square rounded-sm ${getColor(rate)} cursor-pointer transition-transform hover:scale-110`}
                    title={`${day} ${hour}:00 — ${cell.failed}/${cell.total} failed (${(rate * 100).toFixed(0)}%)`}
                  />
                )
              })}
            </React.Fragment>
          ))}
        </div>
      </div>

      {/* Legend */}
      <div className="flex items-center gap-2 mt-3 text-[10px] text-gray-500">
        <span>Less</span>
        {[0, 0.05, 0.15, 0.35, 0.6].map((rate, i) => (
          <div key={i} className={`w-3 h-3 rounded-sm ${getColor(rate)}`} />
        ))}
        <span>More failures</span>
      </div>
    </div>
  )
}
