import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router-dom'
import { getJobs, deleteJob, enableJob, disableJob, triggerJob, createJob } from '../api/client'
import { format } from 'date-fns'
import { useState, useMemo, useRef } from 'react'
import type { Job } from '../types'
import { useListNavigation, useScrollIntoView } from '../hooks/useListNavigation'

type StatusFilter = 'all' | 'enabled' | 'disabled'
type SortField = 'name' | 'schedule' | 'created_at' | 'next_run'
type SortOrder = 'asc' | 'desc'

export default function JobList() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const listContainerRef = useRef<HTMLUListElement>(null)
  const [selectedJobs, setSelectedJobs] = useState<Set<string>>(new Set())
  const [bulkActionPending, setBulkActionPending] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [sortField, setSortField] = useState<SortField>('name')
  const [sortOrder, setSortOrder] = useState<SortOrder>('asc')
  const [showImportModal, setShowImportModal] = useState(false)
  const [importData, setImportData] = useState<string>('')
  const [importError, setImportError] = useState<string | null>(null)

  const { data, isLoading, error } = useQuery({
    queryKey: ['jobs'],
    queryFn: getJobs,
  })

  // Filter and sort jobs
  const filteredJobs = useMemo(() => {
    if (!data?.jobs) return []
    
    let jobs = [...data.jobs]
    
    // Search filter
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase()
      jobs = jobs.filter(job => 
        job.name.toLowerCase().includes(query) ||
        job.description?.toLowerCase().includes(query) ||
        job.schedule.toLowerCase().includes(query) ||
        job.webhook.url.toLowerCase().includes(query)
      )
    }
    
    // Status filter
    if (statusFilter === 'enabled') {
      jobs = jobs.filter(job => job.enabled)
    } else if (statusFilter === 'disabled') {
      jobs = jobs.filter(job => !job.enabled)
    }
    
    // Sort
    jobs.sort((a, b) => {
      let comparison = 0
      switch (sortField) {
        case 'name':
          comparison = a.name.localeCompare(b.name)
          break
        case 'schedule':
          comparison = a.schedule.localeCompare(b.schedule)
          break
        case 'created_at':
          comparison = new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
          break
        case 'next_run':
          const aTime = a.next_run ? new Date(a.next_run).getTime() : Infinity
          const bTime = b.next_run ? new Date(b.next_run).getTime() : Infinity
          comparison = aTime - bTime
          break
      }
      return sortOrder === 'asc' ? comparison : -comparison
    })
    
    return jobs
  }, [data?.jobs, searchQuery, statusFilter, sortField, sortOrder])

  // j/k keyboard navigation
  const { selectedIndex, setSelectedIndex } = useListNavigation({
    items: filteredJobs,
    onEnter: (job) => navigate(`/jobs/${job.id}`),
    enabled: !showImportModal, // Disable when modal is open
  })
  useScrollIntoView(selectedIndex, listContainerRef)

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

  const toggleJobSelection = (jobId: string) => {
    const newSelection = new Set(selectedJobs)
    if (newSelection.has(jobId)) {
      newSelection.delete(jobId)
    } else {
      newSelection.add(jobId)
    }
    setSelectedJobs(newSelection)
  }

  const toggleSelectAll = () => {
    if (selectedJobs.size === filteredJobs.length) {
      setSelectedJobs(new Set())
    } else {
      setSelectedJobs(new Set(filteredJobs.map(j => j.id)))
    }
  }

  const handleBulkEnable = async () => {
    setBulkActionPending(true)
    try {
      await Promise.all(Array.from(selectedJobs).map(id => enableJob(id)))
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      setSelectedJobs(new Set())
    } finally {
      setBulkActionPending(false)
    }
  }

  const handleBulkDisable = async () => {
    setBulkActionPending(true)
    try {
      await Promise.all(Array.from(selectedJobs).map(id => disableJob(id)))
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      setSelectedJobs(new Set())
    } finally {
      setBulkActionPending(false)
    }
  }

  const handleBulkTrigger = async () => {
    setBulkActionPending(true)
    try {
      await Promise.all(Array.from(selectedJobs).map(id => triggerJob(id)))
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      setSelectedJobs(new Set())
    } finally {
      setBulkActionPending(false)
    }
  }

  const handleBulkDelete = async () => {
    if (!confirm(`Are you sure you want to delete ${selectedJobs.size} jobs? This action cannot be undone.`)) {
      return
    }
    setBulkActionPending(true)
    try {
      await Promise.all(Array.from(selectedJobs).map(id => deleteJob(id)))
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      setSelectedJobs(new Set())
    } finally {
      setBulkActionPending(false)
    }
  }

  const handleExportJobs = (jobsToExport: Job[]) => {
    const exportData = jobsToExport.map(job => ({
      name: job.name,
      description: job.description,
      schedule: job.schedule,
      timezone: job.timezone,
      webhook: job.webhook,
      retry_policy: job.retry_policy,
      timeout: job.timeout,
      concurrency: job.concurrency,
      tags: job.tags,
      enabled: job.enabled,
    }))
    
    const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `chronos-jobs-${format(new Date(), 'yyyy-MM-dd-HHmmss')}.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  const handleExportSelected = () => {
    const jobsToExport = filteredJobs.filter(j => selectedJobs.has(j.id))
    handleExportJobs(jobsToExport)
  }

  const handleExportAll = () => {
    handleExportJobs(data?.jobs || [])
  }

  const handleImportFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    
    const reader = new FileReader()
    reader.onload = (event) => {
      const content = event.target?.result as string
      setImportData(content)
      setShowImportModal(true)
      setImportError(null)
    }
    reader.readAsText(file)
    
    // Reset file input
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  const handleImportJobs = async () => {
    try {
      const jobs = JSON.parse(importData)
      if (!Array.isArray(jobs)) {
        throw new Error('Import data must be an array of jobs')
      }
      
      setBulkActionPending(true)
      let successCount = 0
      const errors: string[] = []
      
      for (const job of jobs) {
        try {
          await createJob(job)
          successCount++
        } catch (err) {
          errors.push(`Failed to create "${job.name}": ${(err as Error).message}`)
        }
      }
      
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      setShowImportModal(false)
      setImportData('')
      
      if (errors.length > 0) {
        alert(`Imported ${successCount} jobs.\n\nErrors:\n${errors.join('\n')}`)
      } else {
        alert(`Successfully imported ${successCount} jobs.`)
      }
    } catch (err) {
      setImportError((err as Error).message)
    } finally {
      setBulkActionPending(false)
    }
  }

  if (isLoading) {
    return <div className="text-center py-12">Loading jobs...</div>
  }

  if (error) {
    return <div className="text-center py-12 text-red-600">Error loading jobs</div>
  }

  const hasSelection = selectedJobs.size > 0
  const allSelected = filteredJobs.length > 0 && selectedJobs.size === filteredJobs.length

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-2xl font-bold text-gray-900">Jobs</h2>
        <div className="flex items-center gap-2">
          {/* Import/Export Dropdown */}
          <div className="relative group">
            <button className="inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md shadow-sm text-gray-700 bg-white hover:bg-gray-50">
              Import/Export ▾
            </button>
            <div className="absolute right-0 mt-1 w-48 bg-white rounded-md shadow-lg border border-gray-200 opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all z-10">
              <button
                onClick={handleExportAll}
                disabled={!data?.jobs.length}
                className="block w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 disabled:opacity-50"
              >
                📤 Export All Jobs
              </button>
              {hasSelection && (
                <button
                  onClick={handleExportSelected}
                  className="block w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-100"
                >
                  📤 Export Selected ({selectedJobs.size})
                </button>
              )}
              <hr className="my-1" />
              <label className="block w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 cursor-pointer">
                📥 Import Jobs
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".json"
                  onChange={handleImportFile}
                  className="hidden"
                />
              </label>
            </div>
          </div>
          <Link
            to="/jobs/new"
            className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700"
          >
            Create Job
          </Link>
        </div>
      </div>

      {/* Search and Filters */}
      <div className="bg-white shadow rounded-lg p-4">
        <div className="flex flex-wrap gap-4">
          {/* Search */}
          <div className="flex-1 min-w-64">
            <div className="relative">
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search jobs by name, schedule, or webhook URL..."
                className="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-md focus:ring-indigo-500 focus:border-indigo-500"
              />
              <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <span className="text-gray-400">🔍</span>
              </div>
              {searchQuery && (
                <button
                  onClick={() => setSearchQuery('')}
                  className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-gray-600"
                >
                  ✕
                </button>
              )}
            </div>
          </div>

          {/* Status Filter */}
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as StatusFilter)}
            className="px-3 py-2 border border-gray-300 rounded-md focus:ring-indigo-500 focus:border-indigo-500"
          >
            <option value="all">All Status</option>
            <option value="enabled">Enabled</option>
            <option value="disabled">Disabled</option>
          </select>

          {/* Sort */}
          <select
            value={`${sortField}-${sortOrder}`}
            onChange={(e) => {
              const [field, order] = e.target.value.split('-') as [SortField, SortOrder]
              setSortField(field)
              setSortOrder(order)
            }}
            className="px-3 py-2 border border-gray-300 rounded-md focus:ring-indigo-500 focus:border-indigo-500"
          >
            <option value="name-asc">Name (A-Z)</option>
            <option value="name-desc">Name (Z-A)</option>
            <option value="created_at-desc">Newest First</option>
            <option value="created_at-asc">Oldest First</option>
            <option value="next_run-asc">Next Run (Soonest)</option>
            <option value="schedule-asc">Schedule</option>
          </select>
        </div>

        {/* Results count */}
        <div className="mt-3 text-sm text-gray-500">
          {filteredJobs.length === data?.jobs.length
            ? `${filteredJobs.length} jobs`
            : `${filteredJobs.length} of ${data?.jobs.length} jobs`}
        </div>
      </div>

      {/* Bulk Actions Bar */}
      {hasSelection && (
        <div className="bg-indigo-50 border border-indigo-200 rounded-lg px-4 py-3 flex items-center justify-between">
          <span className="text-sm text-indigo-700 font-medium">
            {selectedJobs.size} job{selectedJobs.size !== 1 ? 's' : ''} selected
          </span>
          <div className="flex items-center gap-2">
            <button
              onClick={handleBulkTrigger}
              disabled={bulkActionPending}
              className="px-3 py-1.5 text-sm bg-indigo-600 text-white rounded-md hover:bg-indigo-700 disabled:opacity-50"
            >
              Trigger All
            </button>
            <button
              onClick={handleBulkEnable}
              disabled={bulkActionPending}
              className="px-3 py-1.5 text-sm bg-green-600 text-white rounded-md hover:bg-green-700 disabled:opacity-50"
            >
              Enable All
            </button>
            <button
              onClick={handleBulkDisable}
              disabled={bulkActionPending}
              className="px-3 py-1.5 text-sm bg-yellow-600 text-white rounded-md hover:bg-yellow-700 disabled:opacity-50"
            >
              Disable All
            </button>
            <button
              onClick={handleBulkDelete}
              disabled={bulkActionPending}
              className="px-3 py-1.5 text-sm bg-red-600 text-white rounded-md hover:bg-red-700 disabled:opacity-50"
            >
              Delete All
            </button>
            <button
              onClick={() => setSelectedJobs(new Set())}
              className="px-3 py-1.5 text-sm text-gray-600 hover:text-gray-800"
            >
              Clear Selection
            </button>
          </div>
        </div>
      )}

      <div className="bg-white shadow overflow-hidden sm:rounded-md">
        {data?.jobs.length === 0 ? (
          <div className="text-center py-12">
            <p className="text-gray-500">No jobs yet</p>
            <Link to="/jobs/new" className="text-indigo-600 hover:text-indigo-800 mt-2 inline-block">
              Create your first job
            </Link>
          </div>
        ) : filteredJobs.length === 0 ? (
          <div className="text-center py-12">
            <p className="text-gray-500">No jobs match your filters</p>
            <button
              onClick={() => { setSearchQuery(''); setStatusFilter('all'); }}
              className="text-indigo-600 hover:text-indigo-800 mt-2 inline-block"
            >
              Clear filters
            </button>
          </div>
        ) : (
          <>
            {/* Select All Header */}
            <div className="px-4 py-3 bg-gray-50 border-b border-gray-200 flex items-center">
              <input
                type="checkbox"
                checked={allSelected}
                onChange={toggleSelectAll}
                className="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
              />
              <span className="ml-3 text-sm text-gray-600">
                {allSelected ? 'Deselect all' : 'Select all'}
              </span>
            </div>
            <ul ref={listContainerRef} className="divide-y divide-gray-200">
              {filteredJobs.map((job, index) => (
                <li 
                  key={job.id} 
                  data-index={index}
                  onClick={() => setSelectedIndex(index)}
                  className={`cursor-pointer transition-colors ${
                    selectedIndex === index 
                      ? 'bg-indigo-100 ring-2 ring-inset ring-indigo-500' 
                      : selectedJobs.has(job.id) 
                        ? 'bg-indigo-50' 
                        : 'hover:bg-gray-50'
                  }`}
                >
                  <div className="px-4 py-4 flex items-center sm:px-6">
                    {/* Checkbox */}
                    <input
                      type="checkbox"
                      checked={selectedJobs.has(job.id)}
                      onChange={() => toggleJobSelection(job.id)}
                      onClick={(e) => e.stopPropagation()}
                      className="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500 mr-4"
                    />
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
          </>
        )}
      </div>

      {/* Import Modal */}
      {showImportModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-lg shadow-xl w-full max-w-2xl">
            <div className="px-6 py-4 border-b border-gray-200 flex justify-between items-center">
              <h3 className="text-lg font-medium text-gray-900">Import Jobs</h3>
              <button
                onClick={() => { setShowImportModal(false); setImportData(''); setImportError(null); }}
                className="text-gray-400 hover:text-gray-600"
              >
                ✕
              </button>
            </div>
            <div className="px-6 py-4">
              <p className="text-sm text-gray-600 mb-4">
                Review the jobs to be imported. Existing jobs with the same name will not be overwritten.
              </p>
              
              {importError && (
                <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-md text-red-700 text-sm">
                  {importError}
                </div>
              )}
              
              <div className="bg-gray-900 text-gray-100 rounded-lg p-4 max-h-64 overflow-auto">
                <pre className="text-sm font-mono whitespace-pre-wrap">{importData}</pre>
              </div>
              
              {!importError && importData && (
                <p className="mt-2 text-sm text-gray-500">
                  {(() => {
                    try {
                      const jobs = JSON.parse(importData)
                      return `${Array.isArray(jobs) ? jobs.length : 0} job(s) will be imported`
                    } catch {
                      return 'Invalid JSON format'
                    }
                  })()}
                </p>
              )}
            </div>
            <div className="px-6 py-4 bg-gray-50 border-t flex justify-end gap-3">
              <button
                onClick={() => { setShowImportModal(false); setImportData(''); setImportError(null); }}
                className="px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 rounded-md"
              >
                Cancel
              </button>
              <button
                onClick={handleImportJobs}
                disabled={bulkActionPending || !importData}
                className="px-4 py-2 text-sm bg-indigo-600 text-white rounded-md hover:bg-indigo-700 disabled:opacity-50"
              >
                {bulkActionPending ? 'Importing...' : 'Import Jobs'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
