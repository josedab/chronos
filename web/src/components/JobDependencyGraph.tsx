import { useMemo, useRef, useEffect, useState } from 'react'
import type { Job } from '../types'

interface JobDependencyGraphProps {
  jobs: Job[]
  dependencies: Record<string, string[]> // jobId -> dependsOn jobIds
  onJobClick?: (jobId: string) => void
}

interface GraphNode {
  id: string
  name: string
  enabled: boolean
  x: number
  y: number
  level: number
}

interface GraphEdge {
  from: string
  to: string
}

export default function JobDependencyGraph({ jobs, dependencies, onJobClick }: JobDependencyGraphProps) {
  const svgRef = useRef<SVGSVGElement>(null)
  const [hoveredNode, setHoveredNode] = useState<string | null>(null)
  const [dimensions, setDimensions] = useState({ width: 800, height: 400 })

  // Calculate graph layout
  const { nodes, edges } = useMemo(() => {
    const nodeMap = new Map<string, GraphNode>()
    const edgeList: GraphEdge[] = []

    // Build dependency edges and calculate levels
    const levels = new Map<string, number>()
    
    // Calculate level for each node (topological order)
    const getLevel = (jobId: string, visited = new Set<string>()): number => {
      if (visited.has(jobId)) return 0 // Circular dependency protection
      if (levels.has(jobId)) return levels.get(jobId)!
      
      visited.add(jobId)
      const deps = dependencies[jobId] || []
      const maxDepLevel = deps.length > 0
        ? Math.max(...deps.map(d => getLevel(d, visited)))
        : -1
      
      const level = maxDepLevel + 1
      levels.set(jobId, level)
      return level
    }

    jobs.forEach(job => getLevel(job.id))

    // Group jobs by level
    const jobsByLevel = new Map<number, Job[]>()
    jobs.forEach(job => {
      const level = levels.get(job.id) || 0
      if (!jobsByLevel.has(level)) {
        jobsByLevel.set(level, [])
      }
      jobsByLevel.get(level)!.push(job)
    })

    // Position nodes
    const maxLevel = Math.max(...Array.from(levels.values()), 0)
    const levelWidth = dimensions.width / (maxLevel + 2)
    
    jobsByLevel.forEach((levelJobs, level) => {
      const levelHeight = dimensions.height / (levelJobs.length + 1)
      levelJobs.forEach((job, index) => {
        nodeMap.set(job.id, {
          id: job.id,
          name: job.name,
          enabled: job.enabled,
          x: (level + 1) * levelWidth,
          y: (index + 1) * levelHeight,
          level,
        })
      })
    })

    // Create edges
    Object.entries(dependencies).forEach(([jobId, deps]) => {
      deps.forEach(depId => {
        if (nodeMap.has(jobId) && nodeMap.has(depId)) {
          edgeList.push({ from: depId, to: jobId })
        }
      })
    })

    return {
      nodes: Array.from(nodeMap.values()),
      edges: edgeList,
    }
  }, [jobs, dependencies, dimensions])

  // Resize observer
  useEffect(() => {
    const container = svgRef.current?.parentElement
    if (!container) return

    const resizeObserver = new ResizeObserver(entries => {
      const { width, height } = entries[0].contentRect
      setDimensions({ width: Math.max(width, 400), height: Math.max(height, 300) })
    })

    resizeObserver.observe(container)
    return () => resizeObserver.disconnect()
  }, [])

  const getNodeColor = (node: GraphNode) => {
    if (hoveredNode === node.id) return '#4F46E5' // indigo-600
    if (!node.enabled) return '#9CA3AF' // gray-400
    return '#10B981' // green-500
  }

  // Calculate edge path with curve
  const getEdgePath = (edge: GraphEdge) => {
    const fromNode = nodes.find(n => n.id === edge.from)
    const toNode = nodes.find(n => n.id === edge.to)
    if (!fromNode || !toNode) return ''

    const startX = fromNode.x + 60 // Right side of node
    const startY = fromNode.y
    const endX = toNode.x - 60 // Left side of node
    const endY = toNode.y

    // Bezier curve control points
    const midX = (startX + endX) / 2

    return `M ${startX} ${startY} C ${midX} ${startY}, ${midX} ${endY}, ${endX} ${endY}`
  }

  if (jobs.length === 0) {
    return (
      <div className="flex items-center justify-center h-64 bg-gray-50 rounded-lg border-2 border-dashed border-gray-300">
        <p className="text-gray-500">No jobs to display</p>
      </div>
    )
  }

  const hasNoDependencies = Object.keys(dependencies).length === 0 || 
    Object.values(dependencies).every(deps => deps.length === 0)

  if (hasNoDependencies) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h3 className="text-lg font-medium text-gray-900 mb-4">Job Dependencies</h3>
        <div className="flex items-center justify-center h-48 bg-gray-50 rounded-lg border border-gray-200">
          <div className="text-center">
            <p className="text-gray-500 mb-2">No dependencies configured</p>
            <p className="text-sm text-gray-400">
              Jobs will run independently based on their schedules
            </p>
          </div>
        </div>
        {/* Simple grid view when no dependencies */}
        <div className="mt-4 grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
          {jobs.map(job => (
            <button
              key={job.id}
              onClick={() => onJobClick?.(job.id)}
              className={`p-3 rounded-lg border text-left transition-colors ${
                job.enabled
                  ? 'border-green-200 bg-green-50 hover:bg-green-100'
                  : 'border-gray-200 bg-gray-50 hover:bg-gray-100'
              }`}
            >
              <div className="font-medium text-sm text-gray-900 truncate">{job.name}</div>
              <div className="text-xs text-gray-500 font-mono mt-1">{job.schedule}</div>
            </button>
          ))}
        </div>
      </div>
    )
  }

  return (
    <div className="bg-white rounded-lg shadow">
      <div className="px-4 py-3 border-b border-gray-200">
        <h3 className="text-lg font-medium text-gray-900">Job Dependency Graph</h3>
        <p className="text-sm text-gray-500 mt-1">
          Visualizes job execution order and dependencies
        </p>
      </div>
      
      <div className="p-4 overflow-auto" style={{ minHeight: '400px' }}>
        <svg
          ref={svgRef}
          width={dimensions.width}
          height={dimensions.height}
          className="w-full"
        >
          {/* Arrow marker definition */}
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
          </defs>

          {/* Edges */}
          {edges.map((edge, i) => (
            <path
              key={i}
              d={getEdgePath(edge)}
              fill="none"
              stroke="#D1D5DB"
              strokeWidth={2}
              markerEnd="url(#arrowhead)"
              className="transition-colors"
              style={{
                stroke: hoveredNode === edge.from || hoveredNode === edge.to ? '#6366F1' : '#D1D5DB'
              }}
            />
          ))}

          {/* Nodes */}
          {nodes.map(node => (
            <g
              key={node.id}
              transform={`translate(${node.x - 50}, ${node.y - 20})`}
              className="cursor-pointer"
              onMouseEnter={() => setHoveredNode(node.id)}
              onMouseLeave={() => setHoveredNode(null)}
              onClick={() => onJobClick?.(node.id)}
            >
              <rect
                width={100}
                height={40}
                rx={6}
                fill={getNodeColor(node)}
                className="transition-colors"
              />
              <text
                x={50}
                y={24}
                textAnchor="middle"
                fill="white"
                fontSize={12}
                fontWeight={500}
                className="select-none"
              >
                {node.name.length > 12 ? node.name.slice(0, 12) + '...' : node.name}
              </text>
              {/* Status indicator */}
              <circle
                cx={90}
                cy={10}
                r={4}
                fill={node.enabled ? '#22C55E' : '#EF4444'}
              />
            </g>
          ))}
        </svg>
      </div>

      {/* Legend */}
      <div className="px-4 py-3 border-t border-gray-200 bg-gray-50 flex items-center gap-6">
        <span className="text-xs text-gray-500">Legend:</span>
        <div className="flex items-center gap-1.5">
          <div className="w-3 h-3 rounded bg-green-500" />
          <span className="text-xs text-gray-600">Enabled</span>
        </div>
        <div className="flex items-center gap-1.5">
          <div className="w-3 h-3 rounded bg-gray-400" />
          <span className="text-xs text-gray-600">Disabled</span>
        </div>
        <div className="flex items-center gap-1.5">
          <div className="w-8 h-0.5 bg-gray-300" />
          <span className="text-xs text-gray-600">Depends on</span>
        </div>
      </div>
    </div>
  )
}
