import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { getAllExecutions } from '../api/client'
import { formatDistanceToNow } from 'date-fns'
import type { Execution } from '../types'

interface ActivityFeedProps {
  limit?: number
  className?: string
}

function getActivityIcon(status: Execution['status']) {
  switch (status) {
    case 'success':
      return (
        <div className="w-8 h-8 rounded-full bg-green-100 dark:bg-green-900/30 flex items-center justify-center">
          <svg className="w-4 h-4 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
          </svg>
        </div>
      )
    case 'failed':
      return (
        <div className="w-8 h-8 rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center">
          <svg className="w-4 h-4 text-red-600 dark:text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </div>
      )
    case 'running':
      return (
        <div className="w-8 h-8 rounded-full bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center">
          <svg className="w-4 h-4 text-blue-600 dark:text-blue-400 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
          </svg>
        </div>
      )
    case 'pending':
      return (
        <div className="w-8 h-8 rounded-full bg-yellow-100 dark:bg-yellow-900/30 flex items-center justify-center">
          <svg className="w-4 h-4 text-yellow-600 dark:text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
      )
    default:
      return (
        <div className="w-8 h-8 rounded-full bg-gray-100 dark:bg-gray-700 flex items-center justify-center">
          <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
      )
  }
}

function getActivityText(exec: Execution) {
  const jobName = exec.job_name || exec.job_id.slice(0, 8)
  
  switch (exec.status) {
    case 'success':
      return (
        <>
          <span className="font-medium text-gray-900 dark:text-white">{jobName}</span>
          <span className="text-gray-500 dark:text-gray-400"> completed successfully</span>
        </>
      )
    case 'failed':
      return (
        <>
          <span className="font-medium text-gray-900 dark:text-white">{jobName}</span>
          <span className="text-gray-500 dark:text-gray-400"> failed</span>
          {exec.error && (
            <span className="text-red-500 dark:text-red-400 text-xs block mt-0.5 truncate max-w-xs">
              {exec.error}
            </span>
          )}
        </>
      )
    case 'running':
      return (
        <>
          <span className="font-medium text-gray-900 dark:text-white">{jobName}</span>
          <span className="text-gray-500 dark:text-gray-400"> is running</span>
        </>
      )
    case 'pending':
      return (
        <>
          <span className="font-medium text-gray-900 dark:text-white">{jobName}</span>
          <span className="text-gray-500 dark:text-gray-400"> is pending</span>
        </>
      )
    default:
      return (
        <>
          <span className="font-medium text-gray-900 dark:text-white">{jobName}</span>
          <span className="text-gray-500 dark:text-gray-400"> - {exec.status}</span>
        </>
      )
  }
}

export default function ActivityFeed({ limit = 10, className = '' }: ActivityFeedProps) {
  const { data, isLoading } = useQuery({
    queryKey: ['activity-feed', limit],
    queryFn: () => getAllExecutions(limit),
    refetchInterval: 10000, // Refresh every 10 seconds
  })

  if (isLoading) {
    return (
      <div className={`bg-white dark:bg-gray-800 rounded-lg shadow ${className}`}>
        <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          <h3 className="text-sm font-medium text-gray-900 dark:text-white">Recent Activity</h3>
        </div>
        <div className="p-4 space-y-4 animate-pulse">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="flex items-start gap-3">
              <div className="w-8 h-8 rounded-full bg-gray-200 dark:bg-gray-700" />
              <div className="flex-1">
                <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-3/4 mb-1" />
                <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded w-1/4" />
              </div>
            </div>
          ))}
        </div>
      </div>
    )
  }

  const executions = data?.executions || []

  return (
    <div className={`bg-white dark:bg-gray-800 rounded-lg shadow ${className}`}>
      <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <h3 className="text-sm font-medium text-gray-900 dark:text-white flex items-center gap-2">
          <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
          Recent Activity
        </h3>
        <Link 
          to="/executions" 
          className="text-xs text-indigo-600 dark:text-indigo-400 hover:underline"
        >
          View all
        </Link>
      </div>

      <div className="p-4">
        {executions.length === 0 ? (
          <p className="text-sm text-gray-500 dark:text-gray-400 text-center py-4">
            No recent activity
          </p>
        ) : (
          <div className="space-y-4">
            {executions.map((exec) => (
              <div key={exec.id} className="flex items-start gap-3 group">
                {getActivityIcon(exec.status)}
                <div className="flex-1 min-w-0">
                  <p className="text-sm">
                    {getActivityText(exec)}
                  </p>
                  <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
                    {formatDistanceToNow(new Date(exec.started_at), { addSuffix: true })}
                  </p>
                </div>
                <Link
                  to={`/jobs/${exec.job_id}`}
                  className="opacity-0 group-hover:opacity-100 text-xs text-indigo-600 dark:text-indigo-400 hover:underline transition-opacity"
                >
                  View
                </Link>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
