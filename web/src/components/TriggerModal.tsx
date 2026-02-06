import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { triggerJobWithParams, TriggerParams } from '../api/client'
import { useToast } from './Toast'

interface TriggerModalProps {
  jobId: string
  jobName: string
  onClose: () => void
}

export default function TriggerModal({ jobId, jobName, onClose }: TriggerModalProps) {
  const queryClient = useQueryClient()
  const toast = useToast()
  const [activeTab, setActiveTab] = useState<'simple' | 'advanced'>('simple')
  const [envVars, setEnvVars] = useState<{ key: string; value: string }[]>([
    { key: '', value: '' }
  ])
  const [headers, setHeaders] = useState<{ key: string; value: string }[]>([
    { key: '', value: '' }
  ])
  const [body, setBody] = useState('')

  const mutation = useMutation({
    mutationFn: (params?: TriggerParams) => triggerJobWithParams(jobId, params),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['executions', jobId] })
      queryClient.invalidateQueries({ queryKey: ['executions'] })
      toast.success(`Job "${jobName}" triggered`, 'Execution started successfully')
      onClose()
    },
    onError: (error: Error) => {
      toast.error('Failed to trigger job', error.message)
    },
  })

  const handleTrigger = () => {
    if (activeTab === 'simple') {
      mutation.mutate(undefined)
    } else {
      const env: Record<string, string> = {}
      const hdrs: Record<string, string> = {}
      
      envVars.forEach(({ key, value }) => {
        if (key.trim()) env[key.trim()] = value
      })
      headers.forEach(({ key, value }) => {
        if (key.trim()) hdrs[key.trim()] = value
      })

      const params: TriggerParams = {}
      if (Object.keys(env).length > 0) params.env = env
      if (Object.keys(hdrs).length > 0) params.headers = hdrs
      if (body.trim()) params.body = body

      mutation.mutate(Object.keys(params).length > 0 ? params : undefined)
    }
  }

  const addEnvVar = () => setEnvVars([...envVars, { key: '', value: '' }])
  const removeEnvVar = (index: number) => setEnvVars(envVars.filter((_, i) => i !== index))
  const updateEnvVar = (index: number, field: 'key' | 'value', val: string) => {
    const updated = [...envVars]
    updated[index][field] = val
    setEnvVars(updated)
  }

  const addHeader = () => setHeaders([...headers, { key: '', value: '' }])
  const removeHeader = (index: number) => setHeaders(headers.filter((_, i) => i !== index))
  const updateHeader = (index: number, field: 'key' | 'value', val: string) => {
    const updated = [...headers]
    updated[index][field] = val
    setHeaders(updated)
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div 
        className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-lg mx-4" 
        onClick={e => e.stopPropagation()}
      >
        <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <svg className="w-5 h-5 text-indigo-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            Trigger Job: {jobName}
          </h3>
        </div>

        <div className="px-6 py-4">
          {/* Tabs */}
          <div className="flex border-b border-gray-200 dark:border-gray-700 mb-4">
            <button
              onClick={() => setActiveTab('simple')}
              className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px ${
                activeTab === 'simple'
                  ? 'border-indigo-600 text-indigo-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'
              }`}
            >
              Simple
            </button>
            <button
              onClick={() => setActiveTab('advanced')}
              className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px ${
                activeTab === 'advanced'
                  ? 'border-indigo-600 text-indigo-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'
              }`}
            >
              Advanced
            </button>
          </div>

          {activeTab === 'simple' ? (
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Trigger the job immediately with default parameters.
            </p>
          ) : (
            <div className="space-y-4 max-h-96 overflow-y-auto">
              {/* Environment Variables */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                    Environment Variables
                  </label>
                  <button
                    type="button"
                    onClick={addEnvVar}
                    className="text-xs text-indigo-600 hover:text-indigo-700"
                  >
                    + Add Variable
                  </button>
                </div>
                <div className="space-y-2">
                  {envVars.map((env, i) => (
                    <div key={i} className="flex gap-2">
                      <input
                        type="text"
                        placeholder="KEY"
                        value={env.key}
                        onChange={e => updateEnvVar(i, 'key', e.target.value)}
                        className="flex-1 px-3 py-1.5 text-sm border rounded dark:bg-gray-700 dark:border-gray-600"
                      />
                      <input
                        type="text"
                        placeholder="value"
                        value={env.value}
                        onChange={e => updateEnvVar(i, 'value', e.target.value)}
                        className="flex-1 px-3 py-1.5 text-sm border rounded dark:bg-gray-700 dark:border-gray-600"
                      />
                      {envVars.length > 1 && (
                        <button
                          type="button"
                          onClick={() => removeEnvVar(i)}
                          className="text-red-500 hover:text-red-700 px-2"
                        >
                          ×
                        </button>
                      )}
                    </div>
                  ))}
                </div>
              </div>

              {/* Custom Headers */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                    Custom Headers
                  </label>
                  <button
                    type="button"
                    onClick={addHeader}
                    className="text-xs text-indigo-600 hover:text-indigo-700"
                  >
                    + Add Header
                  </button>
                </div>
                <div className="space-y-2">
                  {headers.map((header, i) => (
                    <div key={i} className="flex gap-2">
                      <input
                        type="text"
                        placeholder="Header-Name"
                        value={header.key}
                        onChange={e => updateHeader(i, 'key', e.target.value)}
                        className="flex-1 px-3 py-1.5 text-sm border rounded dark:bg-gray-700 dark:border-gray-600"
                      />
                      <input
                        type="text"
                        placeholder="value"
                        value={header.value}
                        onChange={e => updateHeader(i, 'value', e.target.value)}
                        className="flex-1 px-3 py-1.5 text-sm border rounded dark:bg-gray-700 dark:border-gray-600"
                      />
                      {headers.length > 1 && (
                        <button
                          type="button"
                          onClick={() => removeHeader(i)}
                          className="text-red-500 hover:text-red-700 px-2"
                        >
                          ×
                        </button>
                      )}
                    </div>
                  ))}
                </div>
              </div>

              {/* Request Body */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Request Body (JSON)
                </label>
                <textarea
                  value={body}
                  onChange={e => setBody(e.target.value)}
                  rows={4}
                  placeholder='{"key": "value"}'
                  className="w-full px-3 py-2 text-sm font-mono border rounded dark:bg-gray-700 dark:border-gray-600"
                />
              </div>
            </div>
          )}

          {mutation.isError && (
            <div className="mt-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-sm text-red-700 dark:text-red-400">
              {(mutation.error as Error).message}
            </div>
          )}
        </div>

        <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
          >
            Cancel
          </button>
          <button
            onClick={handleTrigger}
            disabled={mutation.isPending}
            className="px-4 py-2 text-sm text-white bg-indigo-600 hover:bg-indigo-700 rounded disabled:opacity-50 flex items-center gap-2"
          >
            {mutation.isPending ? (
              <>
                <svg className="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
                Triggering...
              </>
            ) : (
              <>
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                </svg>
                Trigger Now
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  )
}
