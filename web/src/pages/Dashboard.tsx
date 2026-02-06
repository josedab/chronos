import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router-dom'
import { getClusterStatus, getJobs, getAllExecutions, enableJob, disableJob } from '../api/client'
import { format, formatDistanceToNow } from 'date-fns'
import { useState, useEffect, useMemo } from 'react'
import JobDependencyGraph from '../components/JobDependencyGraph'
import type { Job } from '../types'

type TimeRange = '1h' | '24h' | '7d' | '30d'

// Status icons as SVG components for better visuals
const icons = {
  jobs: (
    <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
    </svg>
  ),
  enabled: (
    <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  ),
  running: (
    <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
    </svg>
  ),
  cluster: (
    <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
    </svg>
  ),
  success: (
    <svg className="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
    </svg>
  ),
  failed: (
    <svg className="w-5 h-5 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
    </svg>
  ),
  pending: (
    <svg className="w-5 h-5 text-yellow-500 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  ),
}

export default function Dashboard() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [timeRange, setTimeRange] = useState<TimeRange>('24h')
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [lastRefresh, setLastRefresh] = useState(new Date())

  // Queries with auto-refresh
  const { data: status, isLoading: statusLoading } = useQuery({
    queryKey: ['cluster-status'],
    queryFn: getClusterStatus,
    refetchInterval: autoRefresh ? 10000 : false,
  })

  const { data: jobsData, isLoading: jobsLoading } = useQuery({
    queryKey: ['jobs'],
    queryFn: getJobs,
    refetchInterval: autoRefresh ? 10000 : false,
  })

  const { data: executionsData, isLoading: executionsLoading } = useQuery({
    queryKey: ['all-executions'],
    queryFn: () => getAllExecutions(100),
    refetchInterval: autoRefresh ? 10000 : false,
  })

  // Toggle job mutation
  const toggleMutation = useMutation({
    mutationFn: (job: Job) => (job.enabled ? disableJob(job.id) : enableJob(job.id)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['jobs'] }),
  })

  // Update last refresh time
  useEffect(() => {
    if (autoRefresh) {
      const interval = setInterval(() => setLastRefresh(new Date()), 10000)
      return () => clearInterval(interval)
    }
  }, [autoRefresh])

  // Computed stats
  const stats = useMemo(() => {
    const jobs = jobsData?.jobs || []
    const executions = executionsData?.executions || []
    
    // Filter executions by time range
    const now = new Date()
    const rangeMs = {
      '1h': 60 * 60 * 1000,
      '24h': 24 * 60 * 60 * 1000,
      '7d': 7 * 24 * 60 * 60 * 1000,
      '30d': 30 * 24 * 60 * 60 * 1000,
    }[timeRange]
    
    const filteredExecutions = executions.filter(e => 
      new Date(e.started_at).getTime() > now.getTime() - rangeMs
    )
    
    const successCount = filteredExecutions.filter(e => e.status === 'success').length
    const failedCount = filteredExecutions.filter(e => e.status === 'failed').length
    const runningCount = filteredExecutions.filter(e => e.status === 'running').length
    const totalExecutions = filteredExecutions.length
    const successRate = totalExecutions > 0 ? Math.round((successCount / totalExecutions) * 100) : 0
    
    // Recent failed jobs (unique)
    const recentFailed = filteredExecutions
      .filter(e => e.status === 'failed')
      .slice(0, 5)
    
    return {
      totalJobs: jobs.length,
      enabledJobs: jobs.filter(j => j.enabled).length,
      disabledJobs: jobs.filter(j => !j.enabled).length,
      successCount,
      failedCount,
      runningCount,
      totalExecutions,
      successRate,
      recentFailed,
    }
  }, [jobsData, executionsData, timeRange])

  // Recent executions (last 10)
  const recentExecutions = useMemo(() => {
    return (executionsData?.executions || []).slice(0, 10)
  }, [executionsData])

  // Failed jobs that need attention
  const failedJobIds = useMemo(() => {
    const failed = new Set<string>()
    stats.recentFailed.forEach(e => failed.add(e.job_id))
    return failed
  }, [stats.recentFailed])

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'success': return icons.success
      case 'failed': return icons.failed
      case 'running': return icons.pending
      default: return icons.pending
    }
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'success': return 'text-green-600 bg-green-100 dark:bg-green-900/30 dark:text-green-400'
      case 'failed': return 'text-red-600 bg-red-100 dark:bg-red-900/30 dark:text-red-400'
      case 'running': return 'text-blue-600 bg-blue-100 dark:bg-blue-900/30 dark:text-blue-400'
      default: return 'text-gray-600 bg-gray-100 dark:bg-gray-700 dark:text-gray-400'
    }
  }

  return (
    <div className="space-y-6">
      {/* Header with controls */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Dashboard</h2>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            Last updated {formatDistanceToNow(lastRefresh, { addSuffix: true })}
          </p>
        </div>
        <div className="flex items-center gap-4">
          {/* Time Range Selector */}
          <div className="flex items-center gap-2 bg-white dark:bg-gray-800 rounded-lg p-1 shadow-sm border border-gray-200 dark:border-gray-700">
            {(['1h', '24h', '7d', '30d'] as TimeRange[]).map((range) => (
              <button
                key={range}
                onClick={() => setTimeRange(range)}
                className={`px-3 py-1 text-sm rounded-md transition-colors ${
                  timeRange === range
                    ? 'bg-indigo-600 text-white'
                    : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`}
              >
                {range}
              </button>
            ))}
          </div>
          {/* Auto-refresh toggle */}
          <button
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors ${
              autoRefresh
                ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
            }`}
          >
            <span className={`w-2 h-2 rounded-full ${autoRefresh ? 'bg-green-500 animate-pulse' : 'bg-gray-400'}`} />
            {autoRefresh ? 'Live' : 'Paused'}
          </button>
        </div>
      </div>

      {/* Failed Jobs Alert Banner */}
      {failedJobIds.size > 0 && (
        <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4">
          <div className="flex items-start">
            <div className="flex-shrink-0">
              <svg className="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
                <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
              </svg>
            </div>
            <div className="ml-3 flex-1">
              <h3 className="text-sm font-medium text-red-800 dark:text-red-200">
                {failedJobIds.size} job{failedJobIds.size > 1 ? 's' : ''} failed recently
              </h3>
              <div className="mt-2 text-sm text-red-700 dark:text-red-300">
                <div className="flex flex-wrap gap-2">
                  {stats.recentFailed.slice(0, 5).map((exec) => {
                    const job = jobsData?.jobs.find(j => j.id === exec.job_id)
                    return (
                      <Link
                        key={exec.id}
                        to={`/jobs/${exec.job_id}`}
                        className="inline-flex items-center px-2 py-1 rounded bg-red-100 dark:bg-red-900/40 hover:bg-red-200 dark:hover:bg-red-900/60"
                      >
                        {job?.name || exec.job_id.slice(0, 8)}
                      </Link>
                    )
                  })}
                </div>
              </div>
            </div>
            <Link
              to="/executions"
              className="ml-4 text-sm font-medium text-red-600 dark:text-red-400 hover:text-red-500"
            >
              View all →
            </Link>
          </div>
        </div>
      )}

      {/* Stats Grid */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Total Jobs"
          value={jobsLoading ? '...' : stats.totalJobs}
          icon={icons.jobs}
          color="indigo"
          subtitle={`${stats.enabledJobs} enabled`}
        />
        <StatCard
          title="Success Rate"
          value={executionsLoading ? '...' : `${stats.successRate}%`}
          icon={icons.success}
          color="green"
          subtitle={`${stats.successCount} of ${stats.totalExecutions} executions`}
        />
        <StatCard
          title="Currently Running"
          value={statusLoading ? '...' : status?.running || 0}
          icon={icons.running}
          color="blue"
          subtitle="Active executions"
          pulse={status?.running ? status.running > 0 : false}
        />
        <StatCard
          title="Cluster Status"
          value={statusLoading ? '...' : status?.is_leader ? 'Leader' : 'Follower'}
          icon={icons.cluster}
          color={status?.is_leader ? 'green' : 'yellow'}
          subtitle="Node role"
        />
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Execution Success Rate Chart */}
        <div className="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
            Execution Status ({timeRange})
          </h3>
          <div className="flex items-center justify-center h-48">
            <SuccessRateDonut
              success={stats.successCount}
              failed={stats.failedCount}
              running={stats.runningCount}
            />
          </div>
          <div className="mt-4 flex justify-center gap-6 text-sm">
            <div className="flex items-center gap-2">
              <span className="w-3 h-3 rounded-full bg-green-500" />
              <span className="text-gray-600 dark:text-gray-400">Success ({stats.successCount})</span>
            </div>
            <div className="flex items-center gap-2">
              <span className="w-3 h-3 rounded-full bg-red-500" />
              <span className="text-gray-600 dark:text-gray-400">Failed ({stats.failedCount})</span>
            </div>
            <div className="flex items-center gap-2">
              <span className="w-3 h-3 rounded-full bg-blue-500" />
              <span className="text-gray-600 dark:text-gray-400">Running ({stats.runningCount})</span>
            </div>
          </div>
        </div>

        {/* Job Status Distribution */}
        <div className="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
            Job Status Distribution
          </h3>
          <div className="space-y-4">
            <StatusBar
              label="Enabled"
              value={stats.enabledJobs}
              total={stats.totalJobs}
              color="bg-green-500"
            />
            <StatusBar
              label="Disabled"
              value={stats.disabledJobs}
              total={stats.totalJobs}
              color="bg-gray-400"
            />
          </div>
          <div className="mt-6">
            <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Quick Toggle</h4>
            <div className="flex flex-wrap gap-2 max-h-32 overflow-auto">
              {jobsData?.jobs.slice(0, 10).map((job) => (
                <button
                  key={job.id}
                  onClick={() => toggleMutation.mutate(job)}
                  disabled={toggleMutation.isPending}
                  className={`inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium transition-colors ${
                    job.enabled
                      ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400 hover:bg-green-200'
                      : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400 hover:bg-gray-200'
                  }`}
                >
                  <span className={`w-2 h-2 rounded-full ${job.enabled ? 'bg-green-500' : 'bg-gray-400'}`} />
                  {job.name.slice(0, 15)}{job.name.length > 15 ? '...' : ''}
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Two-column layout for lists */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Recent Executions */}
        <div className="bg-white dark:bg-gray-800 shadow rounded-lg">
          <div className="px-4 py-5 sm:px-6 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
            <h3 className="text-lg font-medium text-gray-900 dark:text-white">Recent Executions</h3>
            <Link to="/executions" className="text-sm text-indigo-600 dark:text-indigo-400 hover:text-indigo-800">
              View all →
            </Link>
          </div>
          <div className="divide-y divide-gray-200 dark:divide-gray-700 max-h-80 overflow-auto">
            {executionsLoading ? (
              <div className="p-4 text-gray-500 dark:text-gray-400">Loading...</div>
            ) : recentExecutions.length > 0 ? (
              recentExecutions.map((exec) => {
                const job = jobsData?.jobs.find(j => j.id === exec.job_id)
                return (
                  <div key={exec.id} className="px-4 py-3 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-gray-700/50">
                    <div className="flex items-center gap-3">
                      {getStatusIcon(exec.status)}
                      <div>
                        <Link
                          to={`/jobs/${exec.job_id}`}
                          className="text-sm font-medium text-gray-900 dark:text-white hover:text-indigo-600 dark:hover:text-indigo-400"
                        >
                          {job?.name || exec.job_id.slice(0, 8)}
                        </Link>
                        <p className="text-xs text-gray-500 dark:text-gray-400">
                          {formatDistanceToNow(new Date(exec.started_at), { addSuffix: true })}
                        </p>
                      </div>
                    </div>
                    <span className={`px-2 py-1 text-xs rounded-full ${getStatusColor(exec.status)}`}>
                      {exec.status}
                    </span>
                  </div>
                )
              })
            ) : (
              <div className="p-4 text-gray-500 dark:text-gray-400">No recent executions</div>
            )}
          </div>
        </div>

        {/* Upcoming Runs */}
        <div className="bg-white dark:bg-gray-800 shadow rounded-lg">
          <div className="px-4 py-5 sm:px-6 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
            <h3 className="text-lg font-medium text-gray-900 dark:text-white">Upcoming Runs</h3>
            <Link to="/jobs" className="text-sm text-indigo-600 dark:text-indigo-400 hover:text-indigo-800">
              View jobs →
            </Link>
          </div>
          <div className="divide-y divide-gray-200 dark:divide-gray-700 max-h-80 overflow-auto">
            {statusLoading ? (
              <div className="p-4 text-gray-500 dark:text-gray-400">Loading...</div>
            ) : status?.next_runs && status.next_runs.length > 0 ? (
              status.next_runs.slice(0, 10).map((run) => {
                const job = jobsData?.jobs.find((j) => j.id === run.job_id)
                return (
                  <div key={`${run.job_id}-${run.next_run}`} className="px-4 py-3 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-gray-700/50">
                    <div className="flex items-center gap-3">
                      <svg className="w-5 h-5 text-indigo-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                      </svg>
                      <div>
                        <Link
                          to={`/jobs/${run.job_id}`}
                          className="text-sm font-medium text-gray-900 dark:text-white hover:text-indigo-600 dark:hover:text-indigo-400"
                        >
                          {job?.name || run.job_id.slice(0, 8)}
                        </Link>
                        <p className="text-xs text-gray-500 dark:text-gray-400 font-mono">
                          {job?.schedule}
                        </p>
                      </div>
                    </div>
                    <div className="text-right">
                      <p className="text-sm text-gray-900 dark:text-white">
                        {format(new Date(run.next_run), 'HH:mm:ss')}
                      </p>
                      <p className="text-xs text-gray-500 dark:text-gray-400">
                        {format(new Date(run.next_run), 'MMM d')}
                      </p>
                    </div>
                  </div>
                )
              })
            ) : (
              <div className="p-4 text-gray-500 dark:text-gray-400">No upcoming runs scheduled</div>
            )}
          </div>
        </div>
      </div>

      {/* Job Dependency Graph */}
      {jobsData?.jobs && jobsData.jobs.length > 0 && (
        <JobDependencyGraph
          jobs={jobsData.jobs}
          dependencies={{}}
          onJobClick={(jobId) => navigate(`/jobs/${jobId}`)}
        />
      )}

      {/* Quick Actions */}
      <div className="flex flex-wrap gap-4">
        <Link
          to="/jobs/new"
          className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700 dark:bg-indigo-500 dark:hover:bg-indigo-600"
        >
          <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
          Create New Job
        </Link>
        <Link
          to="/templates"
          className="inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 text-sm font-medium rounded-md shadow-sm text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700"
        >
          <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
          </svg>
          Use Template
        </Link>
        <Link
          to="/jobs"
          className="inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 text-sm font-medium rounded-md shadow-sm text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700"
        >
          View All Jobs
        </Link>
        <Link
          to="/executions"
          className="inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 text-sm font-medium rounded-md shadow-sm text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700"
        >
          View Executions
        </Link>
      </div>
    </div>
  )
}

// Stat Card Component
interface StatCardProps {
  title: string
  value: string | number
  icon: React.ReactNode
  color: 'indigo' | 'green' | 'blue' | 'yellow' | 'red'
  subtitle?: string
  pulse?: boolean
}

function StatCard({ title, value, icon, color, subtitle, pulse }: StatCardProps) {
  const colorClasses = {
    indigo: 'bg-indigo-500',
    green: 'bg-green-500',
    blue: 'bg-blue-500',
    yellow: 'bg-yellow-500',
    red: 'bg-red-500',
  }

  return (
    <div className="bg-white dark:bg-gray-800 overflow-hidden shadow rounded-lg hover:shadow-md transition-shadow">
      <div className="p-5">
        <div className="flex items-center">
          <div className={`flex-shrink-0 w-12 h-12 ${colorClasses[color]} rounded-lg flex items-center justify-center text-white ${pulse ? 'animate-pulse' : ''}`}>
            {icon}
          </div>
          <div className="ml-5 w-0 flex-1">
            <dl>
              <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">{title}</dt>
              <dd className="text-2xl font-semibold text-gray-900 dark:text-white">{value}</dd>
              {subtitle && (
                <dd className="text-xs text-gray-400 dark:text-gray-500 mt-1">{subtitle}</dd>
              )}
            </dl>
          </div>
        </div>
      </div>
    </div>
  )
}

// Success Rate Donut Chart
function SuccessRateDonut({ success, failed, running }: { success: number; failed: number; running: number }) {
  const total = success + failed + running
  if (total === 0) {
    return (
      <div className="w-48 h-48 flex items-center justify-center text-gray-400 dark:text-gray-500">
        No data
      </div>
    )
  }

  const successPct = (success / total) * 100
  const failedPct = (failed / total) * 100
  const runningPct = (running / total) * 100
  
  // SVG donut chart
  const radius = 70
  const circumference = 2 * Math.PI * radius

  return (
    <div className="relative w-48 h-48">
      <svg className="w-full h-full transform -rotate-90" viewBox="0 0 160 160">
        {/* Background circle */}
        <circle cx="80" cy="80" r={radius} fill="none" stroke="#e5e7eb" strokeWidth="20" className="dark:stroke-gray-700" />
        
        {/* Success arc */}
        {success > 0 && (
          <circle
            cx="80" cy="80" r={radius}
            fill="none"
            stroke="#22c55e"
            strokeWidth="20"
            strokeDasharray={circumference}
            strokeDashoffset={circumference - (successPct / 100) * circumference}
            className="transition-all duration-500"
          />
        )}
        
        {/* Failed arc */}
        {failed > 0 && (
          <circle
            cx="80" cy="80" r={radius}
            fill="none"
            stroke="#ef4444"
            strokeWidth="20"
            strokeDasharray={circumference}
            strokeDashoffset={circumference - (failedPct / 100) * circumference}
            style={{ transform: `rotate(${(successPct / 100) * 360}deg)`, transformOrigin: '80px 80px' }}
            className="transition-all duration-500"
          />
        )}
        
        {/* Running arc */}
        {running > 0 && (
          <circle
            cx="80" cy="80" r={radius}
            fill="none"
            stroke="#3b82f6"
            strokeWidth="20"
            strokeDasharray={circumference}
            strokeDashoffset={circumference - (runningPct / 100) * circumference}
            style={{ transform: `rotate(${((successPct + failedPct) / 100) * 360}deg)`, transformOrigin: '80px 80px' }}
            className="transition-all duration-500"
          />
        )}
      </svg>
      <div className="absolute inset-0 flex items-center justify-center">
        <div className="text-center">
          <div className="text-3xl font-bold text-gray-900 dark:text-white">
            {Math.round(successPct)}%
          </div>
          <div className="text-sm text-gray-500 dark:text-gray-400">success</div>
        </div>
      </div>
    </div>
  )
}

// Status Bar Component
function StatusBar({ label, value, total, color }: { label: string; value: number; total: number; color: string }) {
  const percentage = total > 0 ? (value / total) * 100 : 0
  
  return (
    <div>
      <div className="flex justify-between mb-1">
        <span className="text-sm font-medium text-gray-700 dark:text-gray-300">{label}</span>
        <span className="text-sm text-gray-500 dark:text-gray-400">{value} / {total}</span>
      </div>
      <div className="w-full h-3 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
        <div
          className={`h-full ${color} rounded-full transition-all duration-500`}
          style={{ width: `${percentage}%` }}
        />
      </div>
    </div>
  )
}
