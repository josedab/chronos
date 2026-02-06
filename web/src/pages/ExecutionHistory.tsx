import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { getAllExecutions } from '../api/client'
import { format, subHours, subDays, isAfter, parseISO } from 'date-fns'
import { useState, useMemo } from 'react'
import type { Execution } from '../types'
import ExecutionLogViewer from '../components/ExecutionLogViewer'
import ExecutionTimeline from '../components/ExecutionTimeline'

type StatusFilter = 'all' | 'success' | 'failed' | 'running' | 'pending'
type TimeRange = '1h' | '6h' | '24h' | '7d' | '30d' | 'custom'

export default function ExecutionHistory() {
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [limit, setLimit] = useState(100)
  const [selectedExecution, setSelectedExecution] = useState<Execution | null>(null)
  const [viewMode, setViewMode] = useState<'table' | 'timeline'>('table')
  const [timeRange, setTimeRange] = useState<TimeRange>('24h')
  const [searchQuery, setSearchQuery] = useState('')
  const [customDateStart, setCustomDateStart] = useState('')
  const [customDateEnd, setCustomDateEnd] = useState('')

  const { data, isLoading, error } = useQuery({
    queryKey: ['all-executions', limit],
    queryFn: () => getAllExecutions(limit),
    refetchInterval: 5000,
  })

  const filteredExecutions = useMemo(() => {
    let executions = data?.executions || []
    
    // Filter by status
    if (statusFilter !== 'all') {
      executions = executions.filter(exec => exec.status === statusFilter)
    }
    
    // Filter by time range
    const now = new Date()
    let startDate: Date | null = null
    
    switch (timeRange) {
      case '1h': startDate = subHours(now, 1); break
      case '6h': startDate = subHours(now, 6); break
      case '24h': startDate = subDays(now, 1); break
      case '7d': startDate = subDays(now, 7); break
      case '30d': startDate = subDays(now, 30); break
      case 'custom':
        if (customDateStart) startDate = parseISO(customDateStart)
        break
    }
    
    if (startDate) {
      executions = executions.filter(exec => 
        isAfter(parseISO(exec.started_at), startDate!)
      )
    }
    
    if (timeRange === 'custom' && customDateEnd) {
      const endDate = parseISO(customDateEnd)
      executions = executions.filter(exec =>
        !isAfter(parseISO(exec.started_at), endDate)
      )
    }
    
    // Filter by search query (job name)
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase()
      executions = executions.filter(exec => 
        exec.job_id.toLowerCase().includes(query) ||
        exec.id.toLowerCase().includes(query)
      )
    }
    
    return executions
  }, [data?.executions, statusFilter, timeRange, customDateStart, customDateEnd, searchQuery])

  const getStatusBadge = (status: Execution['status']) => {
    const styles: Record<string, string> = {
      success: 'bg-green-100 text-green-800',
      failed: 'bg-red-100 text-red-800',
      running: 'bg-blue-100 text-blue-800',
      pending: 'bg-yellow-100 text-yellow-800',
      skipped: 'bg-gray-100 text-gray-800',
    }
    return (
      <span className={`px-2 py-1 text-xs font-medium rounded-full ${styles[status] || 'bg-gray-100 text-gray-800'}`}>
        {status}
      </span>
    )
  }

  const formatDuration = (ms?: number) => {
    if (!ms) return '-'
    if (ms < 1000) return `${ms}ms`
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
    return `${(ms / 60000).toFixed(1)}m`
  }

  if (isLoading) {
    return <div className="text-center py-12">Loading executions...</div>
  }

  if (error) {
    return <div className="text-center py-12 text-red-600">Error loading executions</div>
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Execution History</h2>
        <div className="flex items-center gap-4">
          {/* View Mode Toggle */}
          <div className="flex rounded-md shadow-sm">
            <button
              onClick={() => setViewMode('table')}
              className={`px-3 py-2 text-sm font-medium rounded-l-md border ${
                viewMode === 'table'
                  ? 'bg-indigo-600 text-white border-indigo-600'
                  : 'bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-600'
              }`}
            >
              Table
            </button>
            <button
              onClick={() => setViewMode('timeline')}
              className={`px-3 py-2 text-sm font-medium rounded-r-md border-t border-b border-r ${
                viewMode === 'timeline'
                  ? 'bg-indigo-600 text-white border-indigo-600'
                  : 'bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-600'
              }`}
            >
              Timeline
            </button>
          </div>
        </div>
      </div>

      {/* Search and Filters */}
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4">
        <div className="flex flex-wrap gap-4">
          {/* Search */}
          <div className="flex-1 min-w-[200px]">
            <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Search</label>
            <div className="relative">
              <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search by job ID..."
                className="w-full pl-10 pr-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm dark:bg-gray-700 dark:text-white"
              />
            </div>
          </div>

          {/* Time Range */}
          <div>
            <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Time Range</label>
            <select
              value={timeRange}
              onChange={(e) => setTimeRange(e.target.value as TimeRange)}
              className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm dark:bg-gray-700 dark:text-white"
            >
              <option value="1h">Last 1 hour</option>
              <option value="6h">Last 6 hours</option>
              <option value="24h">Last 24 hours</option>
              <option value="7d">Last 7 days</option>
              <option value="30d">Last 30 days</option>
              <option value="custom">Custom range</option>
            </select>
          </div>

          {/* Custom Date Range */}
          {timeRange === 'custom' && (
            <>
              <div>
                <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">From</label>
                <input
                  type="datetime-local"
                  value={customDateStart}
                  onChange={(e) => setCustomDateStart(e.target.value)}
                  className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm dark:bg-gray-700 dark:text-white"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">To</label>
                <input
                  type="datetime-local"
                  value={customDateEnd}
                  onChange={(e) => setCustomDateEnd(e.target.value)}
                  className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm dark:bg-gray-700 dark:text-white"
                />
              </div>
            </>
          )}

          {/* Status Filter */}
          <div>
            <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Status</label>
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value as StatusFilter)}
              className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm dark:bg-gray-700 dark:text-white"
            >
              <option value="all">All Statuses</option>
              <option value="success">Success</option>
              <option value="failed">Failed</option>
              <option value="running">Running</option>
              <option value="pending">Pending</option>
            </select>
          </div>

          {/* Limit */}
          <div>
            <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Max Results</label>
            <select
              value={limit}
              onChange={(e) => setLimit(Number(e.target.value))}
              className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm dark:bg-gray-700 dark:text-white"
            >
              <option value={25}>25</option>
              <option value={50}>50</option>
              <option value={100}>100</option>
              <option value={200}>200</option>
            </select>
          </div>
        </div>

        {/* Results count */}
        <div className="mt-3 text-sm text-gray-500 dark:text-gray-400">
          Showing {filteredExecutions.length} of {data?.total || 0} executions
        </div>
      </div>

      {/* Stats summary */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-4">
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4">
          <div className="text-sm text-gray-500 dark:text-gray-400">Filtered Results</div>
          <div className="text-2xl font-semibold dark:text-white">{filteredExecutions.length}</div>
        </div>
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4">
          <div className="text-sm text-gray-500 dark:text-gray-400">Success Rate</div>
          <div className="text-2xl font-semibold text-green-600">
            {filteredExecutions.length
              ? Math.round(
                  (filteredExecutions.filter((e) => e.status === 'success').length /
                    filteredExecutions.length) *
                    100
                )
              : 0}%
          </div>
        </div>
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4">
          <div className="text-sm text-gray-500 dark:text-gray-400">Running</div>
          <div className="text-2xl font-semibold text-blue-600">
            {filteredExecutions.filter((e) => e.status === 'running').length}
          </div>
        </div>
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4">
          <div className="text-sm text-gray-500 dark:text-gray-400">Failed</div>
          <div className="text-2xl font-semibold text-red-600">
            {filteredExecutions.filter((e) => e.status === 'failed').length}
          </div>
        </div>
      </div>

      {/* Timeline View */}
      {viewMode === 'timeline' && (
        <ExecutionTimeline executions={filteredExecutions} hoursToShow={6} />
      )}

      {/* Executions table */}
      {viewMode === 'table' && (
      <div className="bg-white shadow overflow-hidden sm:rounded-lg">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Job
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Status
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Started
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Duration
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Node
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Attempts
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {filteredExecutions.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-6 py-12 text-center text-gray-500">
                  No executions found
                </td>
              </tr>
            ) : (
              filteredExecutions.map((exec) => (
                <tr key={exec.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <Link
                      to={`/jobs/${exec.job_id}`}
                      className="text-indigo-600 hover:text-indigo-800 font-medium"
                    >
                      {exec.job_name || exec.job_id.slice(0, 8)}
                    </Link>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    {getStatusBadge(exec.status)}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {format(new Date(exec.started_at), 'MMM d, HH:mm:ss')}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {formatDuration(exec.duration)}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {exec.node_id || '-'}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {exec.attempts}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm">
                    <button
                      onClick={() => setSelectedExecution(exec)}
                      className="text-indigo-600 hover:text-indigo-800"
                    >
                      View Logs
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
      )}

      {/* Log Viewer Modal */}
      {selectedExecution && (
        <ExecutionLogViewer
          execution={selectedExecution}
          onClose={() => setSelectedExecution(null)}
        />
      )}
    </div>
  )
}
