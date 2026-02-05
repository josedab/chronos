import { Link, useLocation } from 'react-router-dom'

interface BreadcrumbItem {
  label: string
  path?: string
}

interface BreadcrumbsProps {
  items?: BreadcrumbItem[]
  className?: string
}

// Generate breadcrumbs from current path
function generateBreadcrumbs(pathname: string): BreadcrumbItem[] {
  const segments = pathname.split('/').filter(Boolean)
  const breadcrumbs: BreadcrumbItem[] = [{ label: 'Dashboard', path: '/' }]

  let currentPath = ''
  segments.forEach((segment, index) => {
    currentPath += `/${segment}`
    const isLast = index === segments.length - 1

    // Map segment to label
    let label = segment.charAt(0).toUpperCase() + segment.slice(1)
    
    // Handle special cases
    if (segment === 'jobs' && !isLast) {
      label = 'Jobs'
    } else if (segment === 'new') {
      label = 'Create New'
    } else if (segment === 'edit') {
      label = 'Edit'
    } else if (segment === 'executions') {
      label = 'Executions'
    } else if (segment === 'workflows') {
      label = 'Workflows'
    } else if (segment === 'templates') {
      label = 'Templates'
    } else if (segment === 'cluster') {
      label = 'Cluster'
    } else if (segment === 'settings') {
      label = 'Settings'
    }

    // UUIDs/IDs are not clickable when last
    const isId = /^[a-f0-9-]{20,}$/i.test(segment)
    if (isId) {
      label = segment.slice(0, 8) + '...'
    }

    breadcrumbs.push({
      label,
      path: isLast ? undefined : currentPath,
    })
  })

  return breadcrumbs
}

export default function Breadcrumbs({ items, className = '' }: BreadcrumbsProps) {
  const location = useLocation()
  const breadcrumbs = items || generateBreadcrumbs(location.pathname)

  if (breadcrumbs.length <= 1) return null

  return (
    <nav className={`flex items-center text-sm ${className}`} aria-label="Breadcrumb">
      <ol className="flex items-center space-x-2">
        {breadcrumbs.map((crumb, index) => {
          const isLast = index === breadcrumbs.length - 1

          return (
            <li key={index} className="flex items-center">
              {index > 0 && (
                <svg
                  className="w-4 h-4 mx-2 text-gray-400"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                </svg>
              )}

              {isLast || !crumb.path ? (
                <span className="text-gray-500 dark:text-gray-400 font-medium">
                  {crumb.label}
                </span>
              ) : (
                <Link
                  to={crumb.path}
                  className="text-gray-600 dark:text-gray-300 hover:text-indigo-600 dark:hover:text-indigo-400 transition-colors"
                >
                  {index === 0 ? (
                    <span className="flex items-center gap-1">
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
                      </svg>
                      <span className="hidden sm:inline">{crumb.label}</span>
                    </span>
                  ) : (
                    crumb.label
                  )}
                </Link>
              )}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
