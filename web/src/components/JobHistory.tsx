import { formatDistanceToNow } from 'date-fns'
import type { Job } from '../types'

interface JobHistoryProps {
  job: Job
  className?: string
}

export default function JobHistory({ job, className = '' }: JobHistoryProps) {
  const createdAt = new Date(job.created_at)
  const updatedAt = new Date(job.updated_at)
  const wasModified = createdAt.getTime() !== updatedAt.getTime()

  return (
    <div className={`bg-white dark:bg-gray-800 rounded-lg shadow ${className}`}>
      <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <h3 className="text-sm font-medium text-gray-900 dark:text-white flex items-center gap-2">
          <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          Job History
        </h3>
      </div>
      
      <div className="p-4">
        <div className="relative">
          {/* Timeline line */}
          <div className="absolute left-3 top-2 bottom-2 w-0.5 bg-gray-200 dark:bg-gray-700" />
          
          {/* Events */}
          <div className="space-y-4">
            {/* Last modified (if different from created) */}
            {wasModified && (
              <div className="relative flex items-start gap-3 pl-8">
                <div className="absolute left-1.5 w-3 h-3 rounded-full bg-blue-500 border-2 border-white dark:border-gray-800" />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-gray-900 dark:text-white">Modified</span>
                    <span className="text-xs text-gray-500 dark:text-gray-400">
                      {formatDistanceToNow(updatedAt, { addSuffix: true })}
                    </span>
                  </div>
                  <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                    {updatedAt.toLocaleString()}
                  </p>
                </div>
              </div>
            )}
            
            {/* Created */}
            <div className="relative flex items-start gap-3 pl-8">
              <div className="absolute left-1.5 w-3 h-3 rounded-full bg-green-500 border-2 border-white dark:border-gray-800" />
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-gray-900 dark:text-white">Created</span>
                  <span className="text-xs text-gray-500 dark:text-gray-400">
                    {formatDistanceToNow(createdAt, { addSuffix: true })}
                  </span>
                </div>
                <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                  {createdAt.toLocaleString()}
                </p>
              </div>
            </div>
          </div>
        </div>
        
        {/* Quick stats */}
        <div className="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700 grid grid-cols-2 gap-4 text-sm">
          <div>
            <span className="text-gray-500 dark:text-gray-400">Status</span>
            <div className="mt-1">
              {job.enabled ? (
                <span className="inline-flex items-center gap-1 text-green-600 dark:text-green-400">
                  <span className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
                  Active
                </span>
              ) : (
                <span className="inline-flex items-center gap-1 text-gray-600 dark:text-gray-400">
                  <span className="w-2 h-2 rounded-full bg-gray-400" />
                  Disabled
                </span>
              )}
            </div>
          </div>
          <div>
            <span className="text-gray-500 dark:text-gray-400">Concurrency</span>
            <div className="mt-1 font-medium text-gray-900 dark:text-white">
              {job.concurrency || 'forbid'}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
