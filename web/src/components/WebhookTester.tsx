import { useState } from 'react'

interface WebhookTestResult {
  success: boolean
  status?: number
  statusText?: string
  responseTime?: number
  error?: string
  responsePreview?: string
}

interface WebhookTesterProps {
  url: string
  method?: string
  headers?: Record<string, string>
  body?: string
  onClose?: () => void
}

export default function WebhookTester({ url, method = 'POST', headers = {}, body, onClose }: WebhookTesterProps) {
  const [testing, setTesting] = useState(false)
  const [result, setResult] = useState<WebhookTestResult | null>(null)
  const [testUrl, setTestUrl] = useState(url)
  const [testMethod, setTestMethod] = useState(method)
  const [testHeaders, setTestHeaders] = useState(JSON.stringify(headers, null, 2) || '{}')
  const [testBody, setTestBody] = useState(body || '{"test": true, "source": "chronos-webhook-tester"}')

  const handleTest = async () => {
    if (!testUrl) {
      setResult({ success: false, error: 'URL is required' })
      return
    }

    setTesting(true)
    setResult(null)

    const startTime = Date.now()

    try {
      // Parse headers
      let parsedHeaders: Record<string, string> = {}
      try {
        parsedHeaders = JSON.parse(testHeaders)
      } catch {
        setResult({ success: false, error: 'Invalid headers JSON' })
        setTesting(false)
        return
      }

      // Make the test request via the backend proxy to avoid CORS
      // In production, this would go through /api/v1/webhooks/test
      const response = await fetch('/api/v1/webhooks/test', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          url: testUrl,
          method: testMethod,
          headers: parsedHeaders,
          body: testMethod !== 'GET' ? testBody : undefined,
        }),
      })

      const responseTime = Date.now() - startTime
      const data = await response.json().catch(() => ({}))

      setResult({
        success: response.ok || data.target_status < 400,
        status: data.target_status || response.status,
        statusText: data.target_status_text || response.statusText,
        responseTime,
        responsePreview: data.target_response 
          ? (typeof data.target_response === 'string' 
              ? data.target_response.substring(0, 500) 
              : JSON.stringify(data.target_response, null, 2).substring(0, 500))
          : undefined,
        error: data.error,
      })
    } catch (error) {
      const responseTime = Date.now() - startTime
      setResult({
        success: false,
        responseTime,
        error: error instanceof Error ? error.message : 'Network error - check if the URL is accessible',
      })
    } finally {
      setTesting(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] overflow-hidden flex flex-col">
        <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
          <h3 className="text-lg font-medium text-gray-900 dark:text-white">Test Webhook</h3>
          {onClose && (
            <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
              ✕
            </button>
          )}
        </div>
        
        <div className="px-6 py-4 space-y-4 overflow-auto flex-1">
          {/* URL and Method */}
          <div className="flex gap-2">
            <select
              value={testMethod}
              onChange={(e) => setTestMethod(e.target.value)}
              className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"
            >
              <option value="GET">GET</option>
              <option value="POST">POST</option>
              <option value="PUT">PUT</option>
              <option value="PATCH">PATCH</option>
              <option value="DELETE">DELETE</option>
            </select>
            <input
              type="url"
              value={testUrl}
              onChange={(e) => setTestUrl(e.target.value)}
              placeholder="https://example.com/webhook"
              className="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"
            />
          </div>

          {/* Headers */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Headers (JSON)
            </label>
            <textarea
              value={testHeaders}
              onChange={(e) => setTestHeaders(e.target.value)}
              rows={3}
              placeholder='{"Authorization": "Bearer token"}'
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm font-mono"
            />
          </div>

          {/* Body */}
          {testMethod !== 'GET' && (
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Request Body (JSON)
              </label>
              <textarea
                value={testBody}
                onChange={(e) => setTestBody(e.target.value)}
                rows={4}
                placeholder='{"key": "value"}'
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm font-mono"
              />
            </div>
          )}

          {/* Test Button */}
          <button
            onClick={handleTest}
            disabled={testing || !testUrl}
            className="w-full px-4 py-2 bg-indigo-600 text-white rounded-md hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
          >
            {testing ? (
              <>
                <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
                Testing...
              </>
            ) : (
              <>
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
                Send Test Request
              </>
            )}
          </button>

          {/* Result */}
          {result && (
            <div className={`p-4 rounded-lg ${
              result.success 
                ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800' 
                : 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800'
            }`}>
              <div className="flex items-center gap-2 mb-2">
                {result.success ? (
                  <svg className="w-5 h-5 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                  </svg>
                ) : (
                  <svg className="w-5 h-5 text-red-600 dark:text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                )}
                <span className={`font-medium ${
                  result.success ? 'text-green-800 dark:text-green-200' : 'text-red-800 dark:text-red-200'
                }`}>
                  {result.success ? 'Success' : 'Failed'}
                </span>
              </div>
              
              <dl className="text-sm space-y-1">
                {result.status && (
                  <div className="flex gap-2">
                    <dt className="text-gray-500 dark:text-gray-400">Status:</dt>
                    <dd className={result.success ? 'text-green-700 dark:text-green-300' : 'text-red-700 dark:text-red-300'}>
                      {result.status} {result.statusText}
                    </dd>
                  </div>
                )}
                {result.responseTime !== undefined && (
                  <div className="flex gap-2">
                    <dt className="text-gray-500 dark:text-gray-400">Response Time:</dt>
                    <dd className="text-gray-700 dark:text-gray-300">{result.responseTime}ms</dd>
                  </div>
                )}
                {result.error && (
                  <div className="flex gap-2">
                    <dt className="text-gray-500 dark:text-gray-400">Error:</dt>
                    <dd className="text-red-700 dark:text-red-300">{result.error}</dd>
                  </div>
                )}
              </dl>

              {result.responsePreview && (
                <div className="mt-3">
                  <span className="text-xs text-gray-500 dark:text-gray-400">Response Preview:</span>
                  <pre className="mt-1 p-2 bg-gray-900 text-gray-100 rounded text-xs overflow-auto max-h-32">
                    {result.responsePreview}
                  </pre>
                </div>
              )}
            </div>
          )}
        </div>

        <div className="px-6 py-3 bg-gray-50 dark:bg-gray-900 border-t border-gray-200 dark:border-gray-700">
          <p className="text-xs text-gray-500 dark:text-gray-400">
            This sends a test request to verify the webhook endpoint is reachable and responds correctly.
          </p>
        </div>
      </div>
    </div>
  )
}
