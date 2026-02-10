import { useEffect, useRef, useState, useCallback, useMemo } from 'react'

/**
 * DAGVisualization - Interactive DAG visualization with zoom, pan, and real-time updates.
 * 
 * Features:
 * - Zoom and pan via mouse wheel and drag
 * - Node selection with details panel
 * - Critical path highlighting
 * - Real-time status updates via WebSocket
 * - Responsive layout with dagre-like algorithm
 * - Timeline view for execution history
 */

// Types
interface NodePosition {
  x: number
  y: number
}

interface NodeMetrics {
  average_runtime: number
  last_runtime?: number
  success_rate: number
  total_runs: number
}

interface GraphNode {
  id: string
  name: string
  type: string
  description?: string
  position?: NodePosition
  status?: string
  dependencies: string[]
  dependents: string[]
  level: number
  metrics?: NodeMetrics
  on_critical_path: boolean
}

interface GraphEdge {
  id: string
  source: string
  target: string
  type: string
  status?: string
  on_critical_path: boolean
}

interface GraphMetadata {
  total_nodes: number
  total_edges: number
  max_depth: number
  parallel_branches: number
  critical_path_length: number
  estimated_runtime: number
}

interface LayoutConfig {
  algorithm: string
  direction: 'TB' | 'BT' | 'LR' | 'RL'
  node_spacing: number
  rank_spacing: number
}

interface GraphData {
  id: string
  name: string
  description?: string
  nodes: GraphNode[]
  edges: GraphEdge[]
  metadata: GraphMetadata
  layout?: LayoutConfig
}

interface NodeExecution {
  node_id: string
  status: string
  started_at?: string
  ended_at?: string
  duration?: number
  attempts: number
  error?: string
}

interface RunGraphData extends GraphData {
  run_id: string
  run_status: string
  started_at: string
  ended_at?: string
  progress: number
  current_nodes: string[]
  completed_nodes: string[]
  failed_nodes: string[]
  pending_nodes: string[]
  node_executions: NodeExecution[]
}

interface DAGVisualizationProps {
  workflowId: string
  runId?: string
  onNodeSelect?: (nodeId: string) => void
  onRefresh?: () => void
  className?: string
}

interface ViewState {
  scale: number
  translateX: number
  translateY: number
}

// Node status colors
const STATUS_COLORS: Record<string, { bg: string; border: string; text: string }> = {
  pending: { bg: '#F3F4F6', border: '#D1D5DB', text: '#6B7280' },
  ready: { bg: '#FEF3C7', border: '#F59E0B', text: '#92400E' },
  running: { bg: '#DBEAFE', border: '#3B82F6', text: '#1E40AF' },
  success: { bg: '#D1FAE5', border: '#10B981', text: '#065F46' },
  failed: { bg: '#FEE2E2', border: '#EF4444', text: '#991B1B' },
  skipped: { bg: '#E5E7EB', border: '#9CA3AF', text: '#4B5563' },
  cancelled: { bg: '#FED7AA', border: '#F97316', text: '#9A3412' },
}

// Node type icons (using simple shapes for now)
const NODE_TYPE_SHAPES: Record<string, string> = {
  trigger: 'polygon', // Diamond
  action: 'rect',     // Rectangle
  condition: 'polygon', // Diamond
  transform: 'rect',  // Rounded rect
  output: 'circle',   // Circle
}

export default function DAGVisualization({
  workflowId,
  runId,
  onNodeSelect,
  onRefresh,
  className = '',
}: DAGVisualizationProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const svgRef = useRef<SVGSVGElement>(null)
  
  const [graphData, setGraphData] = useState<GraphData | RunGraphData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedNode, setSelectedNode] = useState<string | null>(null)
  const [hoveredNode, setHoveredNode] = useState<string | null>(null)
  const [showCriticalPath, setShowCriticalPath] = useState(true)
  const [showTimeline, setShowTimeline] = useState(false)
  
  // View transformation state
  const [view, setView] = useState<ViewState>({
    scale: 1,
    translateX: 0,
    translateY: 0,
  })
  
  // Drag state
  const [isDragging, setIsDragging] = useState(false)
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 })
  
  // WebSocket connection for real-time updates
  const wsRef = useRef<WebSocket | null>(null)
  
  const [dimensions, setDimensions] = useState({ width: 800, height: 600 })

  // Fetch graph data
  const fetchGraphData = useCallback(async () => {
    try {
      setLoading(true)
      const endpoint = runId 
        ? `/api/v1/workflows/${workflowId}/runs/${runId}/graph`
        : `/api/v1/workflows/${workflowId}/graph`
      
      const response = await fetch(endpoint)
      const result = await response.json()
      
      if (result.success) {
        setGraphData(result.data)
        setError(null)
      } else {
        setError(result.error?.message || 'Failed to fetch graph data')
      }
    } catch (err) {
      setError('Network error fetching graph data')
    } finally {
      setLoading(false)
    }
  }, [workflowId, runId])

  // Calculate layout positions
  const layoutNodes = useMemo(() => {
    if (!graphData) return new Map<string, NodePosition>()
    
    const positions = new Map<string, NodePosition>()
    const nodesByLevel = new Map<number, GraphNode[]>()
    
    // Group nodes by level
    graphData.nodes.forEach(node => {
      const level = node.level || 0
      if (!nodesByLevel.has(level)) {
        nodesByLevel.set(level, [])
      }
      nodesByLevel.get(level)!.push(node)
    })
    
    const config = graphData.layout || {
      direction: 'TB',
      node_spacing: 100,
      rank_spacing: 120,
    }
    
    const isHorizontal = config.direction === 'LR' || config.direction === 'RL'
    const padding = 80
    
    // Sort levels
    const levels = Array.from(nodesByLevel.keys()).sort((a, b) => a - b)
    
    levels.forEach((level, levelIndex) => {
      const nodesAtLevel = nodesByLevel.get(level)!
      const levelSize = nodesAtLevel.length
      
      nodesAtLevel.forEach((node, nodeIndex) => {
        let x: number, y: number
        
        if (isHorizontal) {
          x = padding + levelIndex * config.rank_spacing
          y = padding + (nodeIndex - (levelSize - 1) / 2) * config.node_spacing + dimensions.height / 2
        } else {
          x = padding + (nodeIndex - (levelSize - 1) / 2) * config.node_spacing + dimensions.width / 2
          y = padding + levelIndex * config.rank_spacing
        }
        
        // Use existing position if available
        if (node.position) {
          x = node.position.x
          y = node.position.y
        }
        
        positions.set(node.id, { x, y })
      })
    })
    
    return positions
  }, [graphData, dimensions])

  // Connect WebSocket for real-time updates
  useEffect(() => {
    if (!runId) return
    
    const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${wsProtocol}//${window.location.host}/api/v1/workflows/${workflowId}/runs/${runId}/ws`
    
    try {
      wsRef.current = new WebSocket(wsUrl)
      
      wsRef.current.onmessage = (event) => {
        const update = JSON.parse(event.data)
        if (update.type === 'node_update') {
          setGraphData(prev => {
            if (!prev) return prev
            return {
              ...prev,
              nodes: prev.nodes.map(n => 
                n.id === update.node_id ? { ...n, status: update.status } : n
              ),
            }
          })
        }
      }
      
      wsRef.current.onerror = () => {
        // Fallback to polling if WebSocket fails
        const pollInterval = setInterval(fetchGraphData, 5000)
        return () => clearInterval(pollInterval)
      }
    } catch {
      // WebSocket not supported or blocked
    }
    
    return () => {
      wsRef.current?.close()
    }
  }, [workflowId, runId, fetchGraphData])

  // Initial data fetch
  useEffect(() => {
    fetchGraphData()
  }, [fetchGraphData])

  // Resize observer
  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    const resizeObserver = new ResizeObserver(entries => {
      const { width, height } = entries[0].contentRect
      setDimensions({ 
        width: Math.max(width, 400), 
        height: Math.max(height, 300) 
      })
    })

    resizeObserver.observe(container)
    return () => resizeObserver.disconnect()
  }, [])

  // Mouse wheel zoom
  const handleWheel = useCallback((e: React.WheelEvent) => {
    e.preventDefault()
    const scaleFactor = e.deltaY > 0 ? 0.9 : 1.1
    const newScale = Math.min(Math.max(view.scale * scaleFactor, 0.25), 4)
    
    // Zoom toward mouse position
    const rect = svgRef.current?.getBoundingClientRect()
    if (rect) {
      const mouseX = e.clientX - rect.left
      const mouseY = e.clientY - rect.top
      
      const newTranslateX = mouseX - (mouseX - view.translateX) * (newScale / view.scale)
      const newTranslateY = mouseY - (mouseY - view.translateY) * (newScale / view.scale)
      
      setView({
        scale: newScale,
        translateX: newTranslateX,
        translateY: newTranslateY,
      })
    }
  }, [view])

  // Pan handlers
  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    if (e.button !== 0) return // Only left click
    setIsDragging(true)
    setDragStart({ x: e.clientX - view.translateX, y: e.clientY - view.translateY })
  }, [view])

  const handleMouseMove = useCallback((e: React.MouseEvent) => {
    if (!isDragging) return
    setView(prev => ({
      ...prev,
      translateX: e.clientX - dragStart.x,
      translateY: e.clientY - dragStart.y,
    }))
  }, [isDragging, dragStart])

  const handleMouseUp = useCallback(() => {
    setIsDragging(false)
  }, [])

  // Reset view
  const resetView = useCallback(() => {
    setView({ scale: 1, translateX: 0, translateY: 0 })
  }, [])

  // Fit to view
  const fitToView = useCallback(() => {
    if (!graphData || !containerRef.current) return
    
    const positions = Array.from(layoutNodes.values())
    if (positions.length === 0) return
    
    const minX = Math.min(...positions.map(p => p.x)) - 100
    const maxX = Math.max(...positions.map(p => p.x)) + 100
    const minY = Math.min(...positions.map(p => p.y)) - 100
    const maxY = Math.max(...positions.map(p => p.y)) + 100
    
    const graphWidth = maxX - minX
    const graphHeight = maxY - minY
    
    const scaleX = dimensions.width / graphWidth
    const scaleY = dimensions.height / graphHeight
    const newScale = Math.min(scaleX, scaleY, 1.5)
    
    setView({
      scale: newScale,
      translateX: (dimensions.width - graphWidth * newScale) / 2 - minX * newScale,
      translateY: (dimensions.height - graphHeight * newScale) / 2 - minY * newScale,
    })
  }, [graphData, layoutNodes, dimensions])

  // Node click handler
  const handleNodeClick = useCallback((nodeId: string, e: React.MouseEvent) => {
    e.stopPropagation()
    setSelectedNode(nodeId)
    onNodeSelect?.(nodeId)
  }, [onNodeSelect])

  // Get edge path
  const getEdgePath = useCallback((edge: GraphEdge) => {
    const sourcePos = layoutNodes.get(edge.source)
    const targetPos = layoutNodes.get(edge.target)
    
    if (!sourcePos || !targetPos) return ''
    
    const nodeWidth = 140
    const nodeHeight = 50
    
    // Calculate connection points
    const startX = sourcePos.x + nodeWidth / 2
    const startY = sourcePos.y + nodeHeight
    const endX = targetPos.x + nodeWidth / 2
    const endY = targetPos.y
    
    // Bezier curve for smooth edges
    const midY = (startY + endY) / 2
    
    return `M ${startX} ${startY} C ${startX} ${midY}, ${endX} ${midY}, ${endX} ${endY}`
  }, [layoutNodes])

  // Get node style based on status and selection
  const getNodeStyle = useCallback((node: GraphNode) => {
    const colors = STATUS_COLORS[node.status || 'pending']
    const isSelected = selectedNode === node.id
    const isHovered = hoveredNode === node.id
    const isOnCriticalPath = showCriticalPath && node.on_critical_path
    
    return {
      fill: colors.bg,
      stroke: isOnCriticalPath ? '#DC2626' : colors.border,
      strokeWidth: isSelected ? 3 : isHovered ? 2 : isOnCriticalPath ? 2.5 : 1.5,
      filter: isHovered ? 'url(#shadow)' : undefined,
    }
  }, [selectedNode, hoveredNode, showCriticalPath])

  // Render loading state
  if (loading && !graphData) {
    return (
      <div className={`flex items-center justify-center h-96 bg-gray-50 rounded-lg ${className}`}>
        <div className="flex items-center gap-3">
          <div className="animate-spin w-6 h-6 border-2 border-indigo-600 border-t-transparent rounded-full" />
          <span className="text-gray-600">Loading workflow graph...</span>
        </div>
      </div>
    )
  }

  // Render error state
  if (error) {
    return (
      <div className={`flex items-center justify-center h-96 bg-red-50 rounded-lg ${className}`}>
        <div className="text-center">
          <p className="text-red-600 font-medium">{error}</p>
          <button 
            onClick={fetchGraphData}
            className="mt-3 px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700"
          >
            Retry
          </button>
        </div>
      </div>
    )
  }

  if (!graphData) return null

  const isRunData = 'run_id' in graphData

  return (
    <div className={`bg-white rounded-lg shadow-sm border border-gray-200 ${className}`}>
      {/* Toolbar */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200">
        <div className="flex items-center gap-4">
          <h3 className="font-semibold text-gray-900">{graphData.name}</h3>
          {isRunData && (
            <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${
              (graphData as RunGraphData).run_status === 'success' 
                ? 'bg-green-100 text-green-800'
                : (graphData as RunGraphData).run_status === 'failed'
                ? 'bg-red-100 text-red-800'
                : (graphData as RunGraphData).run_status === 'running'
                ? 'bg-blue-100 text-blue-800'
                : 'bg-gray-100 text-gray-800'
            }`}>
              {(graphData as RunGraphData).run_status}
            </span>
          )}
        </div>
        
        <div className="flex items-center gap-2">
          {/* View controls */}
          <button 
            onClick={fitToView}
            className="p-2 text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded"
            title="Fit to view"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
            </svg>
          </button>
          <button 
            onClick={resetView}
            className="p-2 text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded"
            title="Reset view"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          </button>
          <div className="w-px h-6 bg-gray-300 mx-1" />
          <button 
            onClick={() => setShowCriticalPath(!showCriticalPath)}
            className={`px-3 py-1.5 text-sm rounded ${
              showCriticalPath ? 'bg-red-100 text-red-700' : 'bg-gray-100 text-gray-600'
            }`}
            title="Toggle critical path"
          >
            Critical Path
          </button>
          {isRunData && (
            <button 
              onClick={() => setShowTimeline(!showTimeline)}
              className={`px-3 py-1.5 text-sm rounded ${
                showTimeline ? 'bg-indigo-100 text-indigo-700' : 'bg-gray-100 text-gray-600'
              }`}
            >
              Timeline
            </button>
          )}
          <div className="w-px h-6 bg-gray-300 mx-1" />
          <button 
            onClick={() => { fetchGraphData(); onRefresh?.() }}
            className="p-2 text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded"
            title="Refresh"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          </button>
        </div>
      </div>

      {/* Progress bar for running workflows */}
      {isRunData && (graphData as RunGraphData).run_status === 'running' && (
        <div className="h-1 bg-gray-100">
          <div 
            className="h-full bg-indigo-600 transition-all duration-500"
            style={{ width: `${(graphData as RunGraphData).progress * 100}%` }}
          />
        </div>
      )}

      {/* Main visualization area */}
      <div 
        ref={containerRef}
        className="relative overflow-hidden"
        style={{ height: showTimeline ? '400px' : '500px' }}
      >
        <svg
          ref={svgRef}
          width="100%"
          height="100%"
          className={`${isDragging ? 'cursor-grabbing' : 'cursor-grab'}`}
          onWheel={handleWheel}
          onMouseDown={handleMouseDown}
          onMouseMove={handleMouseMove}
          onMouseUp={handleMouseUp}
          onMouseLeave={handleMouseUp}
        >
          {/* Definitions */}
          <defs>
            <marker
              id="arrowhead"
              markerWidth="10"
              markerHeight="7"
              refX="9"
              refY="3.5"
              orient="auto"
            >
              <polygon points="0 0, 10 3.5, 0 7" fill="#9CA3AF" />
            </marker>
            <marker
              id="arrowhead-critical"
              markerWidth="10"
              markerHeight="7"
              refX="9"
              refY="3.5"
              orient="auto"
            >
              <polygon points="0 0, 10 3.5, 0 7" fill="#DC2626" />
            </marker>
            <filter id="shadow" x="-50%" y="-50%" width="200%" height="200%">
              <feDropShadow dx="0" dy="2" stdDeviation="3" floodOpacity="0.3" />
            </filter>
          </defs>

          {/* Transformed group */}
          <g transform={`translate(${view.translateX}, ${view.translateY}) scale(${view.scale})`}>
            {/* Edges */}
            {graphData.edges.map(edge => {
              const isOnCriticalPath = showCriticalPath && edge.on_critical_path
              const isHighlighted = hoveredNode === edge.source || hoveredNode === edge.target
              
              return (
                <path
                  key={edge.id}
                  d={getEdgePath(edge)}
                  fill="none"
                  stroke={isOnCriticalPath ? '#DC2626' : isHighlighted ? '#6366F1' : '#D1D5DB'}
                  strokeWidth={isOnCriticalPath ? 2.5 : isHighlighted ? 2 : 1.5}
                  strokeDasharray={edge.type === 'conditional' ? '5,5' : undefined}
                  markerEnd={isOnCriticalPath ? 'url(#arrowhead-critical)' : 'url(#arrowhead)'}
                  className="transition-all duration-200"
                />
              )
            })}

            {/* Nodes */}
            {graphData.nodes.map(node => {
              const pos = layoutNodes.get(node.id)
              if (!pos) return null
              
              const style = getNodeStyle(node)
              const isSelected = selectedNode === node.id
              const nodeWidth = 140
              const nodeHeight = 50
              
              return (
                <g
                  key={node.id}
                  transform={`translate(${pos.x}, ${pos.y})`}
                  className="cursor-pointer"
                  onMouseEnter={() => setHoveredNode(node.id)}
                  onMouseLeave={() => setHoveredNode(null)}
                  onClick={(e) => handleNodeClick(node.id, e)}
                >
                  {/* Node shape */}
                  <rect
                    x={0}
                    y={0}
                    width={nodeWidth}
                    height={nodeHeight}
                    rx={8}
                    {...style}
                    className="transition-all duration-200"
                  />
                  
                  {/* Critical path indicator */}
                  {showCriticalPath && node.on_critical_path && (
                    <rect
                      x={-3}
                      y={-3}
                      width={nodeWidth + 6}
                      height={nodeHeight + 6}
                      rx={10}
                      fill="none"
                      stroke="#DC2626"
                      strokeWidth={2}
                      strokeDasharray="4,2"
                      opacity={0.6}
                    />
                  )}
                  
                  {/* Type icon */}
                  <circle
                    cx={20}
                    cy={nodeHeight / 2}
                    r={12}
                    fill={STATUS_COLORS[node.status || 'pending'].border}
                    opacity={0.2}
                  />
                  
                  {/* Node name */}
                  <text
                    x={40}
                    y={nodeHeight / 2 - 4}
                    fontSize={12}
                    fontWeight={600}
                    fill={STATUS_COLORS[node.status || 'pending'].text}
                    className="select-none"
                  >
                    {node.name.length > 14 ? node.name.slice(0, 14) + '...' : node.name}
                  </text>
                  
                  {/* Node type */}
                  <text
                    x={40}
                    y={nodeHeight / 2 + 12}
                    fontSize={10}
                    fill="#9CA3AF"
                    className="select-none"
                  >
                    {node.type}
                  </text>
                  
                  {/* Status indicator */}
                  {node.status === 'running' && (
                    <circle
                      cx={nodeWidth - 15}
                      cy={15}
                      r={5}
                      fill="#3B82F6"
                      className="animate-pulse"
                    />
                  )}
                  
                  {/* Selection indicator */}
                  {isSelected && (
                    <rect
                      x={-4}
                      y={-4}
                      width={nodeWidth + 8}
                      height={nodeHeight + 8}
                      rx={12}
                      fill="none"
                      stroke="#4F46E5"
                      strokeWidth={2}
                    />
                  )}
                </g>
              )
            })}
          </g>
        </svg>

        {/* Zoom level indicator */}
        <div className="absolute bottom-4 right-4 bg-white/90 px-2 py-1 rounded text-xs text-gray-500 border">
          {Math.round(view.scale * 100)}%
        </div>
      </div>

      {/* Timeline view (for runs) */}
      {showTimeline && isRunData && (
        <div className="border-t border-gray-200 p-4">
          <h4 className="font-medium text-gray-900 mb-3">Execution Timeline</h4>
          <TimelineView 
            runData={graphData as RunGraphData} 
            onNodeSelect={handleNodeClick}
          />
        </div>
      )}

      {/* Selected node details */}
      {selectedNode && (
        <NodeDetailsPanel
          node={graphData.nodes.find(n => n.id === selectedNode)!}
          execution={isRunData 
            ? (graphData as RunGraphData).node_executions.find(e => e.node_id === selectedNode)
            : undefined
          }
          onClose={() => setSelectedNode(null)}
        />
      )}

      {/* Legend */}
      <div className="px-4 py-3 border-t border-gray-200 bg-gray-50 flex items-center justify-between">
        <div className="flex items-center gap-4 text-xs">
          <span className="text-gray-500">Status:</span>
          {Object.entries(STATUS_COLORS).slice(0, 5).map(([status, colors]) => (
            <div key={status} className="flex items-center gap-1.5">
              <div 
                className="w-3 h-3 rounded border"
                style={{ backgroundColor: colors.bg, borderColor: colors.border }}
              />
              <span className="text-gray-600 capitalize">{status}</span>
            </div>
          ))}
          {showCriticalPath && (
            <div className="flex items-center gap-1.5 ml-2">
              <div className="w-6 h-0.5 bg-red-600" />
              <span className="text-gray-600">Critical Path</span>
            </div>
          )}
        </div>
        
        {/* Metadata */}
        <div className="text-xs text-gray-500">
          {graphData.metadata.total_nodes} nodes • {graphData.metadata.total_edges} edges • 
          Depth: {graphData.metadata.max_depth} • Max parallel: {graphData.metadata.parallel_branches}
        </div>
      </div>
    </div>
  )
}

// Timeline component for run visualization
interface TimelineViewProps {
  runData: RunGraphData
  onNodeSelect: (nodeId: string, e: React.MouseEvent) => void
}

function TimelineView({ runData, onNodeSelect }: TimelineViewProps) {
  const startTime = new Date(runData.started_at).getTime()
  const endTime = runData.ended_at 
    ? new Date(runData.ended_at).getTime() 
    : Date.now()
  const totalDuration = endTime - startTime
  
  // Sort executions by start time
  const sortedExecutions = [...runData.node_executions]
    .filter(e => e.started_at)
    .sort((a, b) => new Date(a.started_at!).getTime() - new Date(b.started_at!).getTime())

  if (sortedExecutions.length === 0) {
    return <div className="text-gray-500 text-sm">No execution data available</div>
  }

  return (
    <div className="space-y-2">
      {sortedExecutions.map(exec => {
        const execStart = new Date(exec.started_at!).getTime()
        const execEnd = exec.ended_at ? new Date(exec.ended_at).getTime() : Date.now()
        const left = ((execStart - startTime) / totalDuration) * 100
        const width = ((execEnd - execStart) / totalDuration) * 100
        
        const node = runData.nodes.find(n => n.id === exec.node_id)
        const colors = STATUS_COLORS[exec.status] || STATUS_COLORS.pending
        
        return (
          <div key={exec.node_id} className="flex items-center gap-3">
            <div className="w-28 text-sm text-gray-600 truncate" title={node?.name}>
              {node?.name || exec.node_id}
            </div>
            <div className="flex-1 h-6 bg-gray-100 rounded relative">
              <button
                className="absolute h-full rounded transition-opacity hover:opacity-80"
                style={{
                  left: `${Math.max(0, left)}%`,
                  width: `${Math.max(1, Math.min(width, 100 - left))}%`,
                  backgroundColor: colors.border,
                }}
                onClick={(e) => onNodeSelect(exec.node_id, e)}
                title={`${exec.status}: ${exec.duration ? Math.round(exec.duration / 1000000) + 'ms' : 'running'}`}
              />
            </div>
            <div className="w-16 text-xs text-gray-500 text-right">
              {exec.duration ? `${Math.round(exec.duration / 1000000)}ms` : '...'}
            </div>
          </div>
        )
      })}
    </div>
  )
}

// Node details side panel
interface NodeDetailsPanelProps {
  node: GraphNode
  execution?: NodeExecution
  onClose: () => void
}

function NodeDetailsPanel({ node, execution, onClose }: NodeDetailsPanelProps) {
  return (
    <div className="absolute top-0 right-0 w-80 h-full bg-white border-l border-gray-200 shadow-lg overflow-auto">
      <div className="sticky top-0 bg-white border-b border-gray-200 px-4 py-3 flex items-center justify-between">
        <h4 className="font-semibold text-gray-900">{node.name}</h4>
        <button 
          onClick={onClose}
          className="p-1 hover:bg-gray-100 rounded"
        >
          <svg className="w-5 h-5 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      
      <div className="p-4 space-y-4">
        {/* Status */}
        <div>
          <label className="text-xs font-medium text-gray-500 uppercase">Status</label>
          <div className="mt-1">
            <span className={`px-2 py-1 rounded text-sm font-medium ${
              node.status === 'success' ? 'bg-green-100 text-green-800' :
              node.status === 'failed' ? 'bg-red-100 text-red-800' :
              node.status === 'running' ? 'bg-blue-100 text-blue-800' :
              'bg-gray-100 text-gray-800'
            }`}>
              {node.status || 'pending'}
            </span>
          </div>
        </div>
        
        {/* Type */}
        <div>
          <label className="text-xs font-medium text-gray-500 uppercase">Type</label>
          <p className="mt-1 text-sm text-gray-900">{node.type}</p>
        </div>
        
        {/* Description */}
        {node.description && (
          <div>
            <label className="text-xs font-medium text-gray-500 uppercase">Description</label>
            <p className="mt-1 text-sm text-gray-700">{node.description}</p>
          </div>
        )}
        
        {/* Dependencies */}
        {node.dependencies.length > 0 && (
          <div>
            <label className="text-xs font-medium text-gray-500 uppercase">Dependencies</label>
            <div className="mt-1 flex flex-wrap gap-1">
              {node.dependencies.map(dep => (
                <span key={dep} className="px-2 py-0.5 bg-gray-100 rounded text-xs text-gray-600">
                  {dep}
                </span>
              ))}
            </div>
          </div>
        )}
        
        {/* Critical path indicator */}
        {node.on_critical_path && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-3">
            <div className="flex items-center gap-2 text-red-800">
              <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                <path fillRule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
              </svg>
              <span className="font-medium text-sm">On Critical Path</span>
            </div>
            <p className="mt-1 text-xs text-red-700">
              This node is on the critical path. Delays here will impact overall runtime.
            </p>
          </div>
        )}
        
        {/* Execution details */}
        {execution && (
          <>
            <div className="border-t border-gray-200 pt-4">
              <label className="text-xs font-medium text-gray-500 uppercase">Execution Details</label>
            </div>
            
            {execution.started_at && (
              <div>
                <label className="text-xs text-gray-500">Started</label>
                <p className="text-sm text-gray-900">
                  {new Date(execution.started_at).toLocaleTimeString()}
                </p>
              </div>
            )}
            
            {execution.duration && (
              <div>
                <label className="text-xs text-gray-500">Duration</label>
                <p className="text-sm text-gray-900">
                  {Math.round(execution.duration / 1000000)}ms
                </p>
              </div>
            )}
            
            {execution.attempts > 1 && (
              <div>
                <label className="text-xs text-gray-500">Attempts</label>
                <p className="text-sm text-gray-900">{execution.attempts}</p>
              </div>
            )}
            
            {execution.error && (
              <div>
                <label className="text-xs text-gray-500">Error</label>
                <p className="mt-1 text-sm text-red-600 bg-red-50 p-2 rounded font-mono">
                  {execution.error}
                </p>
              </div>
            )}
          </>
        )}
        
        {/* Metrics */}
        {node.metrics && (
          <>
            <div className="border-t border-gray-200 pt-4">
              <label className="text-xs font-medium text-gray-500 uppercase">Historical Metrics</label>
            </div>
            
            <div className="grid grid-cols-2 gap-3">
              <div className="bg-gray-50 rounded p-2">
                <div className="text-lg font-semibold text-gray-900">
                  {node.metrics.total_runs}
                </div>
                <div className="text-xs text-gray-500">Total Runs</div>
              </div>
              <div className="bg-gray-50 rounded p-2">
                <div className="text-lg font-semibold text-gray-900">
                  {Math.round(node.metrics.success_rate * 100)}%
                </div>
                <div className="text-xs text-gray-500">Success Rate</div>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
