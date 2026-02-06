import { useState } from 'react'

interface ApiEndpoint {
  method: 'GET' | 'POST' | 'PUT' | 'DELETE'
  path: string
  description: string
  requestBody?: string
  responseExample?: string
  parameters?: { name: string; type: string; required: boolean; description: string }[]
}

const API_ENDPOINTS: ApiEndpoint[] = [
  // Jobs
  {
    method: 'GET',
    path: '/api/v1/jobs',
    description: 'List all jobs',
    responseExample: `{
  "success": true,
  "data": {
    "jobs": [...],
    "total": 10
  }
}`,
  },
  {
    method: 'POST',
    path: '/api/v1/jobs',
    description: 'Create a new job',
    requestBody: `{
  "name": "my-job",
  "schedule": "0 * * * *",
  "webhook": {
    "url": "https://example.com/webhook",
    "method": "POST"
  },
  "enabled": true
}`,
    responseExample: `{
  "success": true,
  "data": { "id": "..." }
}`,
  },
  {
    method: 'GET',
    path: '/api/v1/jobs/{id}',
    description: 'Get a specific job',
    parameters: [
      { name: 'id', type: 'string', required: true, description: 'Job ID' }
    ],
  },
  {
    method: 'PUT',
    path: '/api/v1/jobs/{id}',
    description: 'Update a job',
    parameters: [
      { name: 'id', type: 'string', required: true, description: 'Job ID' }
    ],
  },
  {
    method: 'DELETE',
    path: '/api/v1/jobs/{id}',
    description: 'Delete a job',
    parameters: [
      { name: 'id', type: 'string', required: true, description: 'Job ID' }
    ],
  },
  {
    method: 'POST',
    path: '/api/v1/jobs/{id}/trigger',
    description: 'Manually trigger a job',
    parameters: [
      { name: 'id', type: 'string', required: true, description: 'Job ID' }
    ],
  },
  {
    method: 'POST',
    path: '/api/v1/jobs/{id}/enable',
    description: 'Enable a job',
  },
  {
    method: 'POST',
    path: '/api/v1/jobs/{id}/disable',
    description: 'Disable a job',
  },
  // Executions
  {
    method: 'GET',
    path: '/api/v1/jobs/{id}/executions',
    description: 'Get executions for a job',
    parameters: [
      { name: 'id', type: 'string', required: true, description: 'Job ID' },
      { name: 'limit', type: 'number', required: false, description: 'Max results (default: 20)' }
    ],
  },
  {
    method: 'GET',
    path: '/api/v1/executions',
    description: 'Get all executions across all jobs',
    parameters: [
      { name: 'limit', type: 'number', required: false, description: 'Max results (default: 50)' }
    ],
  },
  {
    method: 'POST',
    path: '/api/v1/jobs/{id}/executions/{execId}/cancel',
    description: 'Cancel a running execution',
  },
  // Cluster
  {
    method: 'GET',
    path: '/api/v1/cluster/status',
    description: 'Get cluster status overview',
  },
  {
    method: 'GET',
    path: '/api/v1/cluster/details',
    description: 'Get detailed cluster information',
  },
  // Config
  {
    method: 'GET',
    path: '/api/v1/config',
    description: 'Get server configuration',
  },
  // Alerts
  {
    method: 'GET',
    path: '/api/v1/alerts/channels',
    description: 'List alert channels',
  },
  {
    method: 'POST',
    path: '/api/v1/alerts/channels',
    description: 'Create an alert channel',
    requestBody: `{
  "name": "slack-alerts",
  "type": "slack",
  "config": {
    "webhook_url": "https://hooks.slack.com/..."
  },
  "enabled": true
}`,
  },
  {
    method: 'GET',
    path: '/api/v1/alerts/rules',
    description: 'List alert rules',
  },
  {
    method: 'POST',
    path: '/api/v1/alerts/rules',
    description: 'Create an alert rule',
    requestBody: `{
  "name": "job-failure",
  "condition": "job_failed",
  "channels": ["channel-id"],
  "enabled": true
}`,
  },
]

const methodColors = {
  GET: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
  POST: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
  PUT: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
  DELETE: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
}

export default function ApiExplorer() {
  const [search, setSearch] = useState('')
  const [selectedEndpoint, setSelectedEndpoint] = useState<ApiEndpoint | null>(null)
  const [filterMethod, setFilterMethod] = useState<string>('all')

  const filteredEndpoints = API_ENDPOINTS.filter(ep => {
    const matchesSearch = ep.path.toLowerCase().includes(search.toLowerCase()) ||
                         ep.description.toLowerCase().includes(search.toLowerCase())
    const matchesMethod = filterMethod === 'all' || ep.method === filterMethod
    return matchesSearch && matchesMethod
  })

  // Group by resource
  const grouped = filteredEndpoints.reduce((acc, ep) => {
    const resource = ep.path.split('/')[3] || 'other' // e.g., 'jobs', 'cluster', 'alerts'
    if (!acc[resource]) acc[resource] = []
    acc[resource].push(ep)
    return acc
  }, {} as Record<string, ApiEndpoint[]>)

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow">
      <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <h3 className="text-lg font-medium text-gray-900 dark:text-white flex items-center gap-2">
          <svg className="w-5 h-5 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
          </svg>
          API Explorer
        </h3>
        <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
          Explore the Chronos REST API endpoints
        </p>
      </div>

      <div className="p-4 border-b border-gray-200 dark:border-gray-700">
        <div className="flex gap-3">
          <div className="flex-1 relative">
            <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search endpoints..."
              className="w-full pl-10 pr-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm dark:bg-gray-700 dark:text-white"
            />
          </div>
          <select
            value={filterMethod}
            onChange={(e) => setFilterMethod(e.target.value)}
            className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm dark:bg-gray-700 dark:text-white"
          >
            <option value="all">All Methods</option>
            <option value="GET">GET</option>
            <option value="POST">POST</option>
            <option value="PUT">PUT</option>
            <option value="DELETE">DELETE</option>
          </select>
        </div>
      </div>

      <div className="flex">
        {/* Endpoints list */}
        <div className="w-1/2 border-r border-gray-200 dark:border-gray-700 max-h-[500px] overflow-y-auto">
          {Object.entries(grouped).map(([resource, endpoints]) => (
            <div key={resource}>
              <div className="px-4 py-2 bg-gray-50 dark:bg-gray-900 text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">
                {resource}
              </div>
              {endpoints.map((ep, i) => (
                <button
                  key={i}
                  onClick={() => setSelectedEndpoint(ep)}
                  className={`w-full px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-gray-700 flex items-center gap-3 ${
                    selectedEndpoint === ep ? 'bg-indigo-50 dark:bg-indigo-900/20' : ''
                  }`}
                >
                  <span className={`px-2 py-0.5 text-xs font-medium rounded ${methodColors[ep.method]}`}>
                    {ep.method}
                  </span>
                  <span className="text-sm font-mono text-gray-700 dark:text-gray-300 truncate">
                    {ep.path}
                  </span>
                </button>
              ))}
            </div>
          ))}
        </div>

        {/* Endpoint detail */}
        <div className="w-1/2 p-4 max-h-[500px] overflow-y-auto">
          {selectedEndpoint ? (
            <div className="space-y-4">
              <div>
                <div className="flex items-center gap-2 mb-2">
                  <span className={`px-2 py-0.5 text-xs font-medium rounded ${methodColors[selectedEndpoint.method]}`}>
                    {selectedEndpoint.method}
                  </span>
                  <code className="text-sm font-mono text-gray-900 dark:text-white">
                    {selectedEndpoint.path}
                  </code>
                </div>
                <p className="text-sm text-gray-600 dark:text-gray-400">
                  {selectedEndpoint.description}
                </p>
              </div>

              {selectedEndpoint.parameters && selectedEndpoint.parameters.length > 0 && (
                <div>
                  <h4 className="text-sm font-medium text-gray-900 dark:text-white mb-2">Parameters</h4>
                  <div className="space-y-2">
                    {selectedEndpoint.parameters.map((param, i) => (
                      <div key={i} className="flex items-start gap-2 text-sm">
                        <code className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded text-xs">
                          {param.name}
                        </code>
                        <span className="text-gray-500 dark:text-gray-400 text-xs">
                          {param.type}
                          {param.required && <span className="text-red-500 ml-1">*</span>}
                        </span>
                        <span className="text-gray-600 dark:text-gray-300 text-xs">
                          {param.description}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {selectedEndpoint.requestBody && (
                <div>
                  <h4 className="text-sm font-medium text-gray-900 dark:text-white mb-2">Request Body</h4>
                  <pre className="text-xs bg-gray-900 text-gray-100 p-3 rounded overflow-x-auto">
                    {selectedEndpoint.requestBody}
                  </pre>
                </div>
              )}

              {selectedEndpoint.responseExample && (
                <div>
                  <h4 className="text-sm font-medium text-gray-900 dark:text-white mb-2">Response Example</h4>
                  <pre className="text-xs bg-gray-900 text-gray-100 p-3 rounded overflow-x-auto">
                    {selectedEndpoint.responseExample}
                  </pre>
                </div>
              )}

              {/* Copy curl command */}
              <div>
                <h4 className="text-sm font-medium text-gray-900 dark:text-white mb-2">cURL</h4>
                <div className="relative">
                  <pre className="text-xs bg-gray-900 text-gray-100 p-3 rounded overflow-x-auto">
                    {`curl -X ${selectedEndpoint.method} \\
  http://localhost:8080${selectedEndpoint.path.replace('{id}', ':id').replace('{execId}', ':execId')} \\
  -H "Content-Type: application/json"${selectedEndpoint.requestBody ? ` \\
  -d '${selectedEndpoint.requestBody.replace(/\n/g, '').replace(/\s+/g, ' ')}'` : ''}`}
                  </pre>
                  <button
                    onClick={() => {
                      const curl = `curl -X ${selectedEndpoint.method} http://localhost:8080${selectedEndpoint.path} -H "Content-Type: application/json"`
                      navigator.clipboard.writeText(curl)
                    }}
                    className="absolute top-2 right-2 text-gray-400 hover:text-white"
                    title="Copy to clipboard"
                  >
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                    </svg>
                  </button>
                </div>
              </div>
            </div>
          ) : (
            <div className="flex items-center justify-center h-full text-gray-500 dark:text-gray-400">
              Select an endpoint to view details
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
