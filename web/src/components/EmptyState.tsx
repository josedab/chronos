import { Link } from 'react-router-dom'

interface EmptyStateProps {
  icon?: 'jobs' | 'executions' | 'search' | 'error' | 'workflows' | 'templates'
  title: string
  description: string
  action?: {
    label: string
    href?: string
    onClick?: () => void
  }
  className?: string
}

const icons = {
  jobs: (
    <svg className="w-12 h-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M21 13.255A23.931 23.931 0 0112 15c-3.183 0-6.22-.62-9-1.745M16 6V4a2 2 0 00-2-2h-4a2 2 0 00-2 2v2m4 6h.01M5 20h14a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
    </svg>
  ),
  executions: (
    <svg className="w-12 h-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  ),
  search: (
    <svg className="w-12 h-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
    </svg>
  ),
  error: (
    <svg className="w-12 h-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
    </svg>
  ),
  workflows: (
    <svg className="w-12 h-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4" />
    </svg>
  ),
  templates: (
    <svg className="w-12 h-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
    </svg>
  ),
}

export default function EmptyState({ 
  icon = 'jobs', 
  title, 
  description, 
  action, 
  className = '' 
}: EmptyStateProps) {
  return (
    <div className={`flex flex-col items-center justify-center py-12 px-4 ${className}`}>
      <div className="text-gray-300 dark:text-gray-600 mb-4">
        {icons[icon]}
      </div>
      <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-1">
        {title}
      </h3>
      <p className="text-sm text-gray-500 dark:text-gray-400 text-center max-w-sm mb-4">
        {description}
      </p>
      {action && (
        action.href ? (
          <Link
            to={action.href}
            className="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-md hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
          >
            {action.label}
          </Link>
        ) : (
          <button
            onClick={action.onClick}
            className="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-md hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
          >
            {action.label}
          </button>
        )
      )}
    </div>
  )
}

// Specific empty state variants
export function NoJobsEmptyState() {
  return (
    <EmptyState
      icon="jobs"
      title="No jobs yet"
      description="Get started by creating your first scheduled job. Jobs run on a cron schedule and call webhooks when triggered."
      action={{ label: 'Create your first job', href: '/jobs/new' }}
    />
  )
}

export function NoExecutionsEmptyState() {
  return (
    <EmptyState
      icon="executions"
      title="No executions yet"
      description="Executions will appear here when your jobs run. Trigger a job manually or wait for the next scheduled run."
    />
  )
}

export function NoSearchResultsEmptyState({ onClear }: { onClear?: () => void }) {
  return (
    <EmptyState
      icon="search"
      title="No results found"
      description="We couldn't find anything matching your search. Try adjusting your filters or search terms."
      action={onClear ? { label: 'Clear search', onClick: onClear } : undefined}
    />
  )
}

export function ErrorEmptyState({ onRetry }: { onRetry?: () => void }) {
  return (
    <EmptyState
      icon="error"
      title="Something went wrong"
      description="We encountered an error while loading this data. Please try again."
      action={onRetry ? { label: 'Try again', onClick: onRetry } : undefined}
    />
  )
}
