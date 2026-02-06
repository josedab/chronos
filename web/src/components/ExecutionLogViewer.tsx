import { useState, useEffect, useRef, useCallback } from 'react'
import type { Execution } from '../types'

interface ExecutionLogViewerProps {
  execution: Execution
  onClose?: () => void
}

interface LogEntry {
  timestamp: string
  level: 'info' | 'warn' | 'error' | 'debug'
  message: string
  metadata?: Record<string, unknown>
}

const LEVEL_STYLES: Record<string, string> = {
  info: 'text-blue-400',
  warn: 'text-yellow-400',
  error: 'text-red-400',
  debug: 'text-gray-400',
}

function parseLogContent(content: string): LogEntry[] {
  const lines = content.split('\n').filter(Boolean)
  return lines.map((line) => {
    // Try to parse JSON log format
    try {
      const parsed = JSON.parse(line)
      return {
        timestamp: parsed.timestamp || parsed.time || new Date().toISOString(),
        level: parsed.level || 'info',
        message: parsed.message || parsed.msg || line,
        metadata: parsed,
      }
    } catch {
      // Fall back to plain text
      const timestampMatch = line.match(/^\[?(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}[^\]]*)\]?\s*/)
      const levelMatch = line.match(/\b(INFO|WARN|ERROR|DEBUG|info|warn|error|debug)\b/i)
      
      return {
        timestamp: timestampMatch?.[1] || new Date().toISOString(),
        level: (levelMatch?.[1]?.toLowerCase() as LogEntry['level']) || 'info',
        message: line,
      }
    }
  })
}

export default function ExecutionLogViewer({ execution, onClose }: ExecutionLogViewerProps) {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [isStreaming, setIsStreaming] = useState(execution.status === 'running')
  const [autoScroll, setAutoScroll] = useState(true)
  const [filter, setFilter] = useState<string>('')
  const [levelFilter, setLevelFilter] = useState<'all' | 'info' | 'warn' | 'error'>('all')
  const logContainerRef = useRef<HTMLDivElement>(null)
  const eventSourceRef = useRef<EventSource | null>(null)

  // Fetch initial logs and set up streaming
  useEffect(() => {
    const fetchLogs = async () => {
      try {
        const response = await fetch(`/api/v1/executions/${execution.id}/logs`)
        if (response.ok) {
          const data = await response.json()
          if (data.success && data.data?.logs) {
            setLogs(parseLogContent(data.data.logs))
          }
        }
      } catch (error) {
        console.error('Failed to fetch logs:', error)
      }
    }

    fetchLogs()

    // Set up SSE for streaming logs if execution is running
    if (execution.status === 'running') {
      const eventSource = new EventSource(`/api/v1/executions/${execution.id}/logs/stream`)
      eventSourceRef.current = eventSource

      eventSource.onmessage = (event) => {
        const newLogs = parseLogContent(event.data)
        setLogs((prev) => [...prev, ...newLogs])
      }

      eventSource.onerror = () => {
        setIsStreaming(false)
        eventSource.close()
      }

      return () => {
        eventSource.close()
      }
    }
  }, [execution.id, execution.status])

  // Auto-scroll to bottom
  useEffect(() => {
    if (autoScroll && logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight
    }
  }, [logs, autoScroll])

  const handleScroll = useCallback(() => {
    if (!logContainerRef.current) return
    const { scrollTop, scrollHeight, clientHeight } = logContainerRef.current
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 50
    setAutoScroll(isAtBottom)
  }, [])

  const filteredLogs = logs.filter((log) => {
    if (levelFilter !== 'all' && log.level !== levelFilter) return false
    if (filter && !log.message.toLowerCase().includes(filter.toLowerCase())) return false
    return true
  })

  const downloadLogs = () => {
    const content = logs.map(log => 
      `[${log.timestamp}] [${log.level.toUpperCase()}] ${log.message}`
    ).join('\n')
    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `execution-${execution.id}-logs.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  const copyLogs = () => {
    const content = logs.map(log => 
      `[${log.timestamp}] [${log.level.toUpperCase()}] ${log.message}`
    ).join('\n')
    navigator.clipboard.writeText(content)
  }

  const getStatusIndicator = () => {
    switch (execution.status) {
      case 'running':
        return <span className="flex items-center gap-2 text-blue-400">
          <span className="animate-pulse w-2 h-2 bg-blue-400 rounded-full" />
          Running
        </span>
      case 'success':
        return <span className="text-green-400">✓ Completed</span>
      case 'failed':
        return <span className="text-red-400">✗ Failed</span>
      default:
        return <span className="text-gray-400">{execution.status}</span>
    }
  }

  return (
    <div className="fixed inset-0 bg-black/80 z-50 flex flex-col">
      {/* Header */}
      <div className="bg-gray-900 border-b border-gray-700 px-4 py-3 flex items-center justify-between">
        <div className="flex items-center gap-4">
          <h3 className="text-white font-semibold">
            Execution Logs: {execution.job_name || execution.job_id.slice(0, 8)}
          </h3>
          {getStatusIndicator()}
          {isStreaming && (
            <span className="text-xs bg-green-600 text-white px-2 py-0.5 rounded">
              Live
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={copyLogs}
            className="px-3 py-1.5 text-sm bg-gray-700 text-gray-200 rounded hover:bg-gray-600"
            title="Copy logs"
          >
            📋 Copy
          </button>
          <button
            onClick={downloadLogs}
            className="px-3 py-1.5 text-sm bg-gray-700 text-gray-200 rounded hover:bg-gray-600"
            title="Download logs"
          >
            ⬇️ Download
          </button>
          {onClose && (
            <button
              onClick={onClose}
              className="px-3 py-1.5 text-sm bg-gray-700 text-gray-200 rounded hover:bg-gray-600"
            >
              ✕ Close
            </button>
          )}
        </div>
      </div>

      {/* Filters */}
      <div className="bg-gray-800 px-4 py-2 flex items-center gap-4 border-b border-gray-700">
        <input
          type="text"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Filter logs..."
          className="bg-gray-900 text-gray-200 px-3 py-1.5 rounded border border-gray-600 text-sm w-64 focus:outline-none focus:border-indigo-500"
        />
        <select
          value={levelFilter}
          onChange={(e) => setLevelFilter(e.target.value as typeof levelFilter)}
          className="bg-gray-900 text-gray-200 px-3 py-1.5 rounded border border-gray-600 text-sm focus:outline-none focus:border-indigo-500"
        >
          <option value="all">All Levels</option>
          <option value="info">Info</option>
          <option value="warn">Warnings</option>
          <option value="error">Errors</option>
        </select>
        <span className="text-gray-400 text-sm">
          {filteredLogs.length} / {logs.length} entries
        </span>
        <div className="flex-1" />
        <label className="flex items-center gap-2 text-gray-300 text-sm">
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
            className="rounded border-gray-600"
          />
          Auto-scroll
        </label>
      </div>

      {/* Log Content */}
      <div
        ref={logContainerRef}
        onScroll={handleScroll}
        className="flex-1 overflow-auto bg-gray-950 p-4 font-mono text-sm"
      >
        {filteredLogs.length === 0 ? (
          <div className="text-gray-500 text-center py-8">
            {logs.length === 0 ? 'No logs available yet...' : 'No logs match the current filter'}
          </div>
        ) : (
          <div className="space-y-0.5">
            {filteredLogs.map((log, index) => (
              <div key={index} className="flex hover:bg-gray-900/50 py-0.5 px-1 rounded">
                <span className="text-gray-500 w-48 flex-shrink-0">
                  {new Date(log.timestamp).toLocaleTimeString()}
                </span>
                <span className={`w-16 flex-shrink-0 ${LEVEL_STYLES[log.level]}`}>
                  [{log.level.toUpperCase()}]
                </span>
                <span className="text-gray-200 break-all">{log.message}</span>
              </div>
            ))}
          </div>
        )}
        
        {isStreaming && (
          <div className="flex items-center gap-2 text-gray-500 mt-4">
            <span className="animate-pulse">●</span>
            Waiting for more logs...
          </div>
        )}
      </div>

      {/* Footer with execution details */}
      <div className="bg-gray-900 border-t border-gray-700 px-4 py-2 text-xs text-gray-400 flex items-center gap-6">
        <span>Execution ID: {execution.id}</span>
        <span>Started: {new Date(execution.started_at).toLocaleString()}</span>
        {execution.completed_at && (
          <span>Completed: {new Date(execution.completed_at).toLocaleString()}</span>
        )}
        {execution.duration && (
          <span>Duration: {execution.duration}ms</span>
        )}
        {execution.node_id && (
          <span>Node: {execution.node_id}</span>
        )}
        <span>Attempts: {execution.attempts}</span>
      </div>
    </div>
  )
}
