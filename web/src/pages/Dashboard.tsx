import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { getClusterStatus, getJobs } from '../api/client'
import { format } from 'date-fns'

export default function Dashboard() {
  const { data: status, isLoading: statusLoading } = useQuery({
    queryKey: ['cluster-status'],
    queryFn: getClusterStatus,
  })

  const { data: jobsData, isLoading: jobsLoading } = useQuery({
    queryKey: ['jobs'],
    queryFn: getJobs,
  })

  const enabledJobs = jobsData?.jobs.filter((j) => j.enabled).length || 0
  const totalJobs = jobsData?.jobs.length || 0

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold text-gray-900">Dashboard</h2>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="p-5">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <div className="w-8 h-8 bg-indigo-500 rounded-md flex items-center justify-center">
                  <span className="text-white text-sm font-bold">J</span>
                </div>
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 truncate">Total Jobs</dt>
                  <dd className="text-lg font-semibold text-gray-900">
                    {jobsLoading ? '...' : totalJobs}
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="p-5">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <div className="w-8 h-8 bg-green-500 rounded-md flex items-center justify-center">
                  <span className="text-white text-sm font-bold">E</span>
                </div>
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 truncate">Enabled Jobs</dt>
                  <dd className="text-lg font-semibold text-gray-900">
                    {jobsLoading ? '...' : enabledJobs}
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="p-5">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <div className={`w-8 h-8 ${status?.is_leader ? 'bg-green-500' : 'bg-yellow-500'} rounded-md flex items-center justify-center`}>
                  <span className="text-white text-sm font-bold">L</span>
                </div>
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 truncate">Cluster Role</dt>
                  <dd className="text-lg font-semibold text-gray-900">
                    {statusLoading ? '...' : status?.is_leader ? 'Leader' : 'Follower'}
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="p-5">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <div className="w-8 h-8 bg-blue-500 rounded-md flex items-center justify-center">
                  <span className="text-white text-sm font-bold">R</span>
                </div>
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 truncate">Running</dt>
                  <dd className="text-lg font-semibold text-gray-900">
                    {statusLoading ? '...' : status?.running || 0}
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Upcoming Runs */}
      <div className="bg-white shadow rounded-lg">
        <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
          <h3 className="text-lg font-medium text-gray-900">Upcoming Runs</h3>
        </div>
        <div className="px-4 py-5 sm:p-6">
          {statusLoading ? (
            <p className="text-gray-500">Loading...</p>
          ) : status?.next_runs && status.next_runs.length > 0 ? (
            <ul className="divide-y divide-gray-200">
              {status.next_runs.slice(0, 5).map((run) => {
                const job = jobsData?.jobs.find((j) => j.id === run.job_id)
                return (
                  <li key={`${run.job_id}-${run.next_run}`} className="py-3 flex justify-between">
                    <Link to={`/jobs/${run.job_id}`} className="text-indigo-600 hover:text-indigo-800">
                      {job?.name || run.job_id.slice(0, 8)}
                    </Link>
                    <span className="text-gray-500">
                      {format(new Date(run.next_run), 'MMM d, yyyy HH:mm:ss')}
                    </span>
                  </li>
                )
              })}
            </ul>
          ) : (
            <p className="text-gray-500">No upcoming runs scheduled</p>
          )}
        </div>
      </div>

      {/* Quick Actions */}
      <div className="flex space-x-4">
        <Link
          to="/jobs/new"
          className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700"
        >
          Create New Job
        </Link>
        <Link
          to="/jobs"
          className="inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md shadow-sm text-gray-700 bg-white hover:bg-gray-50"
        >
          View All Jobs
        </Link>
      </div>
    </div>
  )
}
