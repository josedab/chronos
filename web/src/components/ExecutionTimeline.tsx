import { useMemo } from 'react'
import type { Execution } from '../types'
import { format, differenceInMinutes, startOfHour, endOfHour, addHours, subHours } from 'date-fns'

interface ExecutionTimelineProps {
  executions: Execution[]
  hoursToShow?: number
}

const STATUS_COLORS: Record<string, string> = {
  success: 'bg-green-500',
  failed: 'bg-red-500',
  running: 'bg-blue-500 animate-pulse',
  pending: 'bg-yellow-500',
  skipped: 'bg-gray-400',
}

const STATUS_HOVER_COLORS: Record<string, string> = {
  success: 'hover:bg-green-600',
  failed: 'hover:bg-red-600',
  running: 'hover:bg-blue-600',
  pending: 'hover:bg-yellow-600',
  skipped: 'hover:bg-gray-500',
}

export default function ExecutionTimeline({ executions, hoursToShow = 6 }: ExecutionTimelineProps) {
  const { timeRange, groupedExecutions, hourMarkers } = useMemo(() => {
    const now = new Date()
    const start = startOfHour(subHours(now, hoursToShow - 1))
    const end = endOfHour(now)
    const totalMinutes = differenceInMinutes(end, start)

    // Generate hour markers
    const markers: Date[] = []
    for (let i = 0; i <= hoursToShow; i++) {
      markers.push(addHours(start, i))
    }

    // Group executions by job
    const grouped = new Map<string, Execution[]>()
    executions.forEach((exec) => {
      const jobKey = exec.job_name || exec.job_id.slice(0, 8)
      if (!grouped.has(jobKey)) {
        grouped.set(jobKey, [])
      }
      grouped.get(jobKey)!.push(exec)
    })

    return {
      timeRange: { start, end, totalMinutes },
      groupedExecutions: grouped,
      hourMarkers: markers,
    }
  }, [executions, hoursToShow])

  const getExecutionPosition = (exec: Execution) => {
    const execStart = new Date(exec.started_at)
    const execEnd = exec.completed_at ? new Date(exec.completed_at) : new Date()
    
    const startOffset = Math.max(0, differenceInMinutes(execStart, timeRange.start))
    const duration = Math.max(2, differenceInMinutes(execEnd, execStart)) // Min 2 min for visibility
    
    const left = (startOffset / timeRange.totalMinutes) * 100
    const width = Math.min((duration / timeRange.totalMinutes) * 100, 100 - left)
    
    return { left: `${left}%`, width: `${Math.max(width, 0.5)}%` }
  }

  const formatDuration = (exec: Execution) => {
    if (!exec.completed_at) return 'Running...'
    const start = new Date(exec.started_at)
    const end = new Date(exec.completed_at)
    const ms = end.getTime() - start.getTime()
    if (ms < 1000) return `${ms}ms`
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
    return `${Math.round(ms / 60000)}m`
  }

  if (executions.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow p-6 text-center text-gray-500">
        No executions in the selected time range
      </div>
    )
  }

  return (
    <div className="bg-white rounded-lg shadow overflow-hidden">
      <div className="px-4 py-3 border-b border-gray-200 bg-gray-50">
        <h3 className="text-sm font-medium text-gray-700">
          Execution Timeline (Last {hoursToShow} hours)
        </h3>
      </div>
      
      <div className="p-4">
        {/* Time axis */}
        <div className="relative h-8 mb-2 border-b border-gray-200">
          {hourMarkers.map((marker, i) => (
            <div
              key={i}
              className="absolute top-0 flex flex-col items-center"
              style={{ left: `${(i / hoursToShow) * 100}%` }}
            >
              <div className="h-4 w-px bg-gray-300" />
              <span className="text-xs text-gray-500 mt-1">
                {format(marker, 'HH:mm')}
              </span>
            </div>
          ))}
          {/* Now indicator */}
          <div
            className="absolute top-0 h-full flex flex-col items-center"
            style={{ 
              left: `${(differenceInMinutes(new Date(), timeRange.start) / timeRange.totalMinutes) * 100}%` 
            }}
          >
            <div className="h-4 w-0.5 bg-red-500" />
            <span className="text-xs text-red-500 font-medium">Now</span>
          </div>
        </div>

        {/* Job rows */}
        <div className="space-y-2">
          {Array.from(groupedExecutions.entries()).map(([jobName, jobExecutions]) => (
            <div key={jobName} className="flex items-center gap-4">
              {/* Job name */}
              <div className="w-32 flex-shrink-0 text-sm font-medium text-gray-700 truncate" title={jobName}>
                {jobName}
              </div>
              
              {/* Timeline bar */}
              <div className="flex-1 relative h-8 bg-gray-100 rounded">
                {/* Hour grid lines */}
                {hourMarkers.map((_, i) => (
                  <div
                    key={i}
                    className="absolute top-0 h-full w-px bg-gray-200"
                    style={{ left: `${(i / hoursToShow) * 100}%` }}
                  />
                ))}
                
                {/* Execution blocks */}
                {jobExecutions.map((exec) => {
                  const position = getExecutionPosition(exec)
                  return (
                    <div
                      key={exec.id}
                      className={`absolute top-1 bottom-1 rounded cursor-pointer transition-colors ${STATUS_COLORS[exec.status]} ${STATUS_HOVER_COLORS[exec.status]}`}
                      style={position}
                      title={`${exec.status}\nStarted: ${format(new Date(exec.started_at), 'HH:mm:ss')}\nDuration: ${formatDuration(exec)}\nAttempts: ${exec.attempts}`}
                    >
                      {/* Show duration label if block is wide enough */}
                      {parseFloat(position.width) > 5 && (
                        <span className="absolute inset-0 flex items-center justify-center text-xs text-white font-medium truncate px-1">
                          {formatDuration(exec)}
                        </span>
                      )}
                    </div>
                  )
                })}
              </div>
            </div>
          ))}
        </div>

        {/* Legend */}
        <div className="mt-4 pt-4 border-t border-gray-200 flex items-center gap-6">
          <span className="text-xs text-gray-500">Status:</span>
          <div className="flex items-center gap-4">
            {Object.entries(STATUS_COLORS).map(([status, color]) => (
              <div key={status} className="flex items-center gap-1.5">
                <div className={`w-3 h-3 rounded ${color.replace(' animate-pulse', '')}`} />
                <span className="text-xs text-gray-600 capitalize">{status}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
