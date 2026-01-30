import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { getJobs, deleteJob, enableJob, disableJob, triggerJob } from '../api/client'
import { format } from 'date-fns'
import type { Job } from '../types'

export default function JobList() {
  const queryClient = useQueryClient()

  const { data, isLoading, error } = useQuery({
    queryKey: ['jobs'],
    queryFn: getJobs,
  })

  const deleteMutation = useMutation({
    mutationFn: deleteJob,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['jobs'] }),
  })

  const toggleMutation = useMutation({
    mutationFn: (job: Job) => (job.enabled ? disableJob(job.id) : enableJob(job.id)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['jobs'] }),
  })

  const triggerMutation = useMutation({
    mutationFn: triggerJob,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['jobs'] }),
  })

  if (isLoading) {
    return <div className="text-center py-12">Loading jobs...</div>
  }

  if (error) {
    return <div className="text-center py-12 text-red-600">Error loading jobs</div>
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-2xl font-bold text-gray-900">Jobs</h2>
        <Link
          to="/jobs/new"
          className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700"
        >
          Create Job
        </Link>
      </div>

      <div className="bg-white shadow overflow-hidden sm:rounded-md">
        {data?.jobs.length === 0 ? (
          <div className="text-center py-12">
            <p className="text-gray-500">No jobs yet</p>
            <Link to="/jobs/new" className="text-indigo-600 hover:text-indigo-800 mt-2 inline-block">
              Create your first job
            </Link>
          </div>
        ) : (
          <ul className="divide-y divide-gray-200">
            {data?.jobs.map((job) => (
              <li key={job.id}>
                <div className="px-4 py-4 flex items-center sm:px-6">
                  <div className="min-w-0 flex-1 sm:flex sm:items-center sm:justify-between">
                    <div>
                      <div className="flex text-sm">
                        <Link to={`/jobs/${job.id}`} className="font-medium text-indigo-600 truncate hover:text-indigo-800">
                          {job.name}
                        </Link>
                        <span className={`ml-2 px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${
                          job.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                        }`}>
                          {job.enabled ? 'Enabled' : 'Disabled'}
                        </span>
                      </div>
                      <div className="mt-2 flex">
                        <div className="flex items-center text-sm text-gray-500">
                          <span className="font-mono">{job.schedule}</span>
                          {job.timezone && <span className="ml-2">({job.timezone})</span>}
                        </div>
                      </div>
                      {job.next_run && job.enabled && (
                        <div className="mt-1 text-sm text-gray-500">
                          Next run: {format(new Date(job.next_run), 'MMM d, yyyy HH:mm:ss')}
                        </div>
                      )}
                    </div>
                  </div>
                  <div className="ml-5 flex-shrink-0 flex space-x-2">
                    <button
                      onClick={() => triggerMutation.mutate(job.id)}
                      disabled={triggerMutation.isPending}
                      className="px-3 py-1 text-sm text-indigo-600 hover:text-indigo-800 border border-indigo-300 rounded hover:bg-indigo-50"
                    >
                      Trigger
                    </button>
                    <button
                      onClick={() => toggleMutation.mutate(job)}
                      disabled={toggleMutation.isPending}
                      className={`px-3 py-1 text-sm rounded ${
                        job.enabled
                          ? 'text-yellow-600 hover:text-yellow-800 border border-yellow-300 hover:bg-yellow-50'
                          : 'text-green-600 hover:text-green-800 border border-green-300 hover:bg-green-50'
                      }`}
                    >
                      {job.enabled ? 'Disable' : 'Enable'}
                    </button>
                    <button
                      onClick={() => {
                        if (confirm('Are you sure you want to delete this job?')) {
                          deleteMutation.mutate(job.id)
                        }
                      }}
                      disabled={deleteMutation.isPending}
                      className="px-3 py-1 text-sm text-red-600 hover:text-red-800 border border-red-300 rounded hover:bg-red-50"
                    >
                      Delete
                    </button>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
