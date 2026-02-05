interface SkeletonProps {
  className?: string
}

// Base skeleton pulse component
export function Skeleton({ className = '' }: SkeletonProps) {
  return (
    <div className={`animate-pulse bg-gray-200 dark:bg-gray-700 rounded ${className}`} />
  )
}

// Text skeleton - single line
export function SkeletonText({ className = 'h-4 w-24' }: SkeletonProps) {
  return <Skeleton className={className} />
}

// Card skeleton
export function SkeletonCard({ className = '' }: SkeletonProps) {
  return (
    <div className={`bg-white dark:bg-gray-800 rounded-lg shadow p-4 ${className}`}>
      <Skeleton className="h-4 w-1/3 mb-3" />
      <Skeleton className="h-8 w-1/2 mb-2" />
      <Skeleton className="h-3 w-2/3" />
    </div>
  )
}

// Table row skeleton
export function SkeletonTableRow({ columns = 5 }: { columns?: number }) {
  return (
    <tr className="animate-pulse">
      {Array.from({ length: columns }).map((_, i) => (
        <td key={i} className="px-3 py-4">
          <Skeleton className="h-4 w-full max-w-[120px]" />
        </td>
      ))}
    </tr>
  )
}

// Table skeleton with multiple rows
export function SkeletonTable({ rows = 5, columns = 5 }: { rows?: number; columns?: number }) {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow overflow-hidden">
      <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <Skeleton className="h-5 w-32" />
      </div>
      <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
        <thead className="bg-gray-50 dark:bg-gray-900">
          <tr>
            {Array.from({ length: columns }).map((_, i) => (
              <th key={i} className="px-3 py-3">
                <Skeleton className="h-3 w-16" />
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
          {Array.from({ length: rows }).map((_, i) => (
            <SkeletonTableRow key={i} columns={columns} />
          ))}
        </tbody>
      </table>
    </div>
  )
}

// Stats card skeleton (for dashboard)
export function SkeletonStatsCard() {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 animate-pulse">
      <div className="flex items-center gap-4">
        <Skeleton className="w-12 h-12 rounded-lg" />
        <div className="flex-1">
          <Skeleton className="h-3 w-20 mb-2" />
          <Skeleton className="h-6 w-12" />
        </div>
      </div>
    </div>
  )
}

// Dashboard skeleton
export function SkeletonDashboard() {
  return (
    <div className="space-y-6">
      {/* Stats row */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <SkeletonStatsCard />
        <SkeletonStatsCard />
        <SkeletonStatsCard />
        <SkeletonStatsCard />
      </div>

      {/* Chart area */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 bg-white dark:bg-gray-800 rounded-lg shadow p-4">
          <Skeleton className="h-4 w-32 mb-4" />
          <Skeleton className="h-48 w-full" />
        </div>
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4">
          <Skeleton className="h-4 w-24 mb-4" />
          <div className="space-y-3">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        </div>
      </div>

      {/* Table */}
      <SkeletonTable rows={5} columns={4} />
    </div>
  )
}

// Job list skeleton
export function SkeletonJobList() {
  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex justify-between items-center">
        <Skeleton className="h-8 w-32" />
        <Skeleton className="h-10 w-28" />
      </div>

      {/* Filters */}
      <div className="flex gap-3">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-10 w-32" />
        <Skeleton className="h-10 w-32" />
      </div>

      {/* Table */}
      <SkeletonTable rows={8} columns={6} />
    </div>
  )
}

// Job detail skeleton
export function SkeletonJobDetail() {
  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex justify-between items-start">
        <div>
          <Skeleton className="h-8 w-48 mb-2" />
          <Skeleton className="h-4 w-64" />
        </div>
        <div className="flex gap-2">
          <Skeleton className="h-10 w-24" />
          <Skeleton className="h-10 w-20" />
          <Skeleton className="h-10 w-20" />
        </div>
      </div>

      {/* Config card */}
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
        <Skeleton className="h-5 w-28 mb-4" />
        <div className="grid grid-cols-2 gap-6">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i}>
              <Skeleton className="h-3 w-16 mb-2" />
              <Skeleton className="h-5 w-32" />
            </div>
          ))}
        </div>
      </div>

      {/* Executions table */}
      <SkeletonTable rows={5} columns={5} />
    </div>
  )
}

// Form skeleton
export function SkeletonForm({ fields = 5 }: { fields?: number }) {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 space-y-6">
      {Array.from({ length: fields }).map((_, i) => (
        <div key={i}>
          <Skeleton className="h-4 w-24 mb-2" />
          <Skeleton className="h-10 w-full" />
        </div>
      ))}
      <div className="flex justify-end gap-3 pt-4">
        <Skeleton className="h-10 w-20" />
        <Skeleton className="h-10 w-24" />
      </div>
    </div>
  )
}
