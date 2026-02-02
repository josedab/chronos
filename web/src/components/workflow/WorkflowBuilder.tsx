import React, { useState, useCallback, useRef, useEffect } from 'react';

// Types for the workflow builder
export interface Position {
  x: number;
  y: number;
}

export interface WorkflowNode {
  id: string;
  type: NodeType;
  name: string;
  position: Position;
  config: Record<string, unknown>;
  status?: NodeStatus;
}

export interface WorkflowEdge {
  id: string;
  source: string;
  sourcePort: string;
  target: string;
  targetPort: string;
  condition?: string;
}

export interface Workflow {
  id: string;
  name: string;
  description?: string;
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
  settings?: WorkflowSettings;
}

export interface WorkflowSettings {
  concurrencyPolicy: 'allow' | 'forbid' | 'replace';
  onFailure: 'stop' | 'continue' | 'retry';
  maxRetries: number;
  timeout: string;
}

export type NodeType =
  | 'trigger'
  | 'job'
  | 'condition'
  | 'delay'
  | 'parallel'
  | 'loop'
  | 'notification'
  | 'http'
  | 'script'
  | 'transform'
  | 'aggregate'
  | 'end';

export type NodeStatus = 'pending' | 'running' | 'success' | 'failed' | 'skipped';

// Node type definitions
export const NODE_TYPES: Record<NodeType, { label: string; color: string; icon: string; ports: { inputs: string[]; outputs: string[] } }> = {
  trigger: { label: 'Trigger', color: '#10B981', icon: '⚡', ports: { inputs: [], outputs: ['output'] } },
  job: { label: 'Job', color: '#3B82F6', icon: '📋', ports: { inputs: ['input'], outputs: ['success', 'failure'] } },
  condition: { label: 'Condition', color: '#F59E0B', icon: '🔀', ports: { inputs: ['input'], outputs: ['true', 'false'] } },
  delay: { label: 'Delay', color: '#8B5CF6', icon: '⏱️', ports: { inputs: ['input'], outputs: ['output'] } },
  parallel: { label: 'Parallel', color: '#EC4899', icon: '⚙️', ports: { inputs: ['input'], outputs: ['output'] } },
  loop: { label: 'Loop', color: '#14B8A6', icon: '🔄', ports: { inputs: ['input', 'items'], outputs: ['item', 'done'] } },
  notification: { label: 'Notify', color: '#EF4444', icon: '🔔', ports: { inputs: ['input'], outputs: ['output'] } },
  http: { label: 'HTTP', color: '#6366F1', icon: '🌐', ports: { inputs: ['input'], outputs: ['success', 'failure'] } },
  script: { label: 'Script', color: '#F97316', icon: '📝', ports: { inputs: ['input'], outputs: ['output', 'error'] } },
  transform: { label: 'Transform', color: '#84CC16', icon: '🔄', ports: { inputs: ['input'], outputs: ['output'] } },
  aggregate: { label: 'Aggregate', color: '#06B6D4', icon: '📊', ports: { inputs: ['input'], outputs: ['output'] } },
  end: { label: 'End', color: '#6B7280', icon: '🏁', ports: { inputs: ['input'], outputs: [] } },
};

interface WorkflowBuilderProps {
  workflow?: Workflow;
  onChange?: (workflow: Workflow) => void;
  onSave?: (workflow: Workflow) => void;
  readOnly?: boolean;
  showStatus?: boolean;
}

export const WorkflowBuilder: React.FC<WorkflowBuilderProps> = ({
  workflow: initialWorkflow,
  onChange,
  onSave,
  readOnly = false,
  showStatus = false,
}) => {
  const [workflow, setWorkflow] = useState<Workflow>(
    initialWorkflow || {
      id: `wf-${Date.now()}`,
      name: 'New Workflow',
      nodes: [],
      edges: [],
    }
  );
  const [selectedNode, setSelectedNode] = useState<string | null>(null);
  const [selectedEdge, setSelectedEdge] = useState<string | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [dragOffset, setDragOffset] = useState<Position>({ x: 0, y: 0 });
  const [connecting, setConnecting] = useState<{ nodeId: string; port: string } | null>(null);
  const [zoom, setZoom] = useState(1);
  const [pan, setPan] = useState<Position>({ x: 0, y: 0 });
  const canvasRef = useRef<HTMLDivElement>(null);

  // Update parent when workflow changes
  useEffect(() => {
    onChange?.(workflow);
  }, [workflow, onChange]);

  // Add a new node
  const addNode = useCallback((type: NodeType, position: Position) => {
    const newNode: WorkflowNode = {
      id: `node-${Date.now()}`,
      type,
      name: NODE_TYPES[type].label,
      position,
      config: getDefaultConfig(type),
    };

    setWorkflow((prev) => ({
      ...prev,
      nodes: [...prev.nodes, newNode],
    }));

    return newNode;
  }, []);

  // Remove a node
  const removeNode = useCallback((nodeId: string) => {
    setWorkflow((prev) => ({
      ...prev,
      nodes: prev.nodes.filter((n) => n.id !== nodeId),
      edges: prev.edges.filter((e) => e.source !== nodeId && e.target !== nodeId),
    }));
    setSelectedNode(null);
  }, []);

  // Update node position
  const updateNodePosition = useCallback((nodeId: string, position: Position) => {
    setWorkflow((prev) => ({
      ...prev,
      nodes: prev.nodes.map((n) => (n.id === nodeId ? { ...n, position } : n)),
    }));
  }, []);

  // Update node config
  const updateNodeConfig = useCallback((nodeId: string, config: Record<string, unknown>) => {
    setWorkflow((prev) => ({
      ...prev,
      nodes: prev.nodes.map((n) => (n.id === nodeId ? { ...n, config: { ...n.config, ...config } } : n)),
    }));
  }, []);

  // Add an edge
  const addEdge = useCallback((source: string, sourcePort: string, target: string, targetPort: string) => {
    // Check for cycles
    if (wouldCreateCycle(workflow, source, target)) {
      alert('Cannot create connection: this would create a cycle');
      return;
    }

    // Check if edge already exists
    const exists = workflow.edges.some(
      (e) => e.source === source && e.sourcePort === sourcePort && e.target === target && e.targetPort === targetPort
    );
    if (exists) return;

    const newEdge: WorkflowEdge = {
      id: `edge-${Date.now()}`,
      source,
      sourcePort,
      target,
      targetPort,
    };

    setWorkflow((prev) => ({
      ...prev,
      edges: [...prev.edges, newEdge],
    }));
  }, [workflow]);

  // Remove an edge
  const removeEdge = useCallback((edgeId: string) => {
    setWorkflow((prev) => ({
      ...prev,
      edges: prev.edges.filter((e) => e.id !== edgeId),
    }));
    setSelectedEdge(null);
  }, []);

  // Handle mouse events for dragging
  const handleMouseDown = useCallback((e: React.MouseEvent, nodeId: string) => {
    if (readOnly) return;
    e.stopPropagation();
    setSelectedNode(nodeId);
    setSelectedEdge(null);
    setIsDragging(true);

    const node = workflow.nodes.find((n) => n.id === nodeId);
    if (node) {
      setDragOffset({
        x: e.clientX - node.position.x * zoom - pan.x,
        y: e.clientY - node.position.y * zoom - pan.y,
      });
    }
  }, [readOnly, workflow.nodes, zoom, pan]);

  const handleMouseMove = useCallback((e: React.MouseEvent) => {
    if (!isDragging || !selectedNode || readOnly) return;

    const newX = (e.clientX - dragOffset.x - pan.x) / zoom;
    const newY = (e.clientY - dragOffset.y - pan.y) / zoom;

    // Snap to grid
    const snappedX = Math.round(newX / 20) * 20;
    const snappedY = Math.round(newY / 20) * 20;

    updateNodePosition(selectedNode, { x: snappedX, y: snappedY });
  }, [isDragging, selectedNode, dragOffset, pan, zoom, updateNodePosition, readOnly]);

  const handleMouseUp = useCallback(() => {
    setIsDragging(false);
  }, []);

  // Handle port click for connections
  const handlePortClick = useCallback((e: React.MouseEvent, nodeId: string, port: string, isOutput: boolean) => {
    if (readOnly) return;
    e.stopPropagation();

    if (connecting) {
      if (isOutput || connecting.nodeId === nodeId) {
        setConnecting(null);
        return;
      }
      // Create connection
      addEdge(connecting.nodeId, connecting.port, nodeId, port);
      setConnecting(null);
    } else if (isOutput) {
      setConnecting({ nodeId, port });
    }
  }, [connecting, addEdge, readOnly]);

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (readOnly) return;

      if (e.key === 'Delete' || e.key === 'Backspace') {
        if (selectedNode) {
          removeNode(selectedNode);
        } else if (selectedEdge) {
          removeEdge(selectedEdge);
        }
      }

      if (e.key === 'Escape') {
        setConnecting(null);
        setSelectedNode(null);
        setSelectedEdge(null);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [selectedNode, selectedEdge, removeNode, removeEdge, readOnly]);

  // Handle drop from toolbar
  const handleDrop = useCallback((e: React.DragEvent) => {
    if (readOnly) return;
    e.preventDefault();

    const type = e.dataTransfer.getData('nodeType') as NodeType;
    if (!type || !NODE_TYPES[type]) return;

    const rect = canvasRef.current?.getBoundingClientRect();
    if (!rect) return;

    const x = (e.clientX - rect.left - pan.x) / zoom;
    const y = (e.clientY - rect.top - pan.y) / zoom;

    addNode(type, { x: Math.round(x / 20) * 20, y: Math.round(y / 20) * 20 });
  }, [addNode, pan, zoom, readOnly]);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
  }, []);

  // Render node
  const renderNode = (node: WorkflowNode) => {
    const nodeType = NODE_TYPES[node.type];
    const isSelected = selectedNode === node.id;
    const statusColor = showStatus && node.status ? getStatusColor(node.status) : null;

    return (
      <div
        key={node.id}
        className={`absolute rounded-lg shadow-lg border-2 transition-all cursor-move select-none ${
          isSelected ? 'ring-2 ring-blue-500' : ''
        }`}
        style={{
          left: node.position.x * zoom + pan.x,
          top: node.position.y * zoom + pan.y,
          backgroundColor: 'white',
          borderColor: statusColor || nodeType.color,
          minWidth: 160 * zoom,
          transform: `scale(${zoom})`,
          transformOrigin: 'top left',
        }}
        onMouseDown={(e) => handleMouseDown(e, node.id)}
        onClick={(e) => {
          e.stopPropagation();
          setSelectedNode(node.id);
          setSelectedEdge(null);
        }}
      >
        {/* Node header */}
        <div
          className="px-3 py-2 rounded-t-lg text-white font-medium flex items-center gap-2"
          style={{ backgroundColor: statusColor || nodeType.color }}
        >
          <span>{nodeType.icon}</span>
          <span className="truncate">{node.name}</span>
          {showStatus && node.status && (
            <span className="ml-auto text-xs bg-white/20 px-2 py-0.5 rounded">
              {node.status}
            </span>
          )}
        </div>

        {/* Node body */}
        <div className="p-2 text-sm text-gray-600">
          {node.type === 'job' && node.config.jobId && (
            <div className="truncate">Job: {String(node.config.jobId)}</div>
          )}
          {node.type === 'trigger' && node.config.schedule && (
            <div className="truncate">Schedule: {String(node.config.schedule)}</div>
          )}
          {node.type === 'delay' && node.config.duration && (
            <div className="truncate">Wait: {String(node.config.duration)}</div>
          )}
          {node.type === 'condition' && node.config.expression && (
            <div className="truncate">If: {String(node.config.expression)}</div>
          )}
        </div>

        {/* Input ports */}
        <div className="absolute -left-3 top-1/2 -translate-y-1/2 flex flex-col gap-2">
          {nodeType.ports.inputs.map((port) => (
            <div
              key={port}
              className={`w-4 h-4 rounded-full border-2 cursor-pointer transition-colors ${
                connecting ? 'bg-green-400 border-green-600' : 'bg-gray-200 border-gray-400'
              } hover:bg-blue-400 hover:border-blue-600`}
              title={port}
              onClick={(e) => handlePortClick(e, node.id, port, false)}
            />
          ))}
        </div>

        {/* Output ports */}
        <div className="absolute -right-3 top-1/2 -translate-y-1/2 flex flex-col gap-2">
          {nodeType.ports.outputs.map((port) => (
            <div
              key={port}
              className={`w-4 h-4 rounded-full border-2 cursor-pointer transition-colors ${
                connecting?.nodeId === node.id && connecting?.port === port
                  ? 'bg-blue-500 border-blue-700'
                  : 'bg-gray-200 border-gray-400'
              } hover:bg-blue-400 hover:border-blue-600`}
              title={port}
              onClick={(e) => handlePortClick(e, node.id, port, true)}
            />
          ))}
        </div>
      </div>
    );
  };

  // Render edge
  const renderEdge = (edge: WorkflowEdge) => {
    const sourceNode = workflow.nodes.find((n) => n.id === edge.source);
    const targetNode = workflow.nodes.find((n) => n.id === edge.target);
    if (!sourceNode || !targetNode) return null;

    const sourceType = NODE_TYPES[sourceNode.type];
    const sourcePortIndex = sourceType.ports.outputs.indexOf(edge.sourcePort);
    const targetType = NODE_TYPES[targetNode.type];
    const targetPortIndex = targetType.ports.inputs.indexOf(edge.targetPort);

    const startX = (sourceNode.position.x + 160) * zoom + pan.x;
    const startY = (sourceNode.position.y + 30 + sourcePortIndex * 20) * zoom + pan.y;
    const endX = targetNode.position.x * zoom + pan.x;
    const endY = (targetNode.position.y + 30 + targetPortIndex * 20) * zoom + pan.y;

    const controlPoint1X = startX + (endX - startX) * 0.4;
    const controlPoint2X = startX + (endX - startX) * 0.6;

    const path = `M ${startX} ${startY} C ${controlPoint1X} ${startY}, ${controlPoint2X} ${endY}, ${endX} ${endY}`;
    const isSelected = selectedEdge === edge.id;

    return (
      <g key={edge.id}>
        <path
          d={path}
          fill="none"
          stroke={isSelected ? '#3B82F6' : '#9CA3AF'}
          strokeWidth={isSelected ? 3 : 2}
          className="cursor-pointer transition-colors hover:stroke-blue-400"
          onClick={(e) => {
            e.stopPropagation();
            setSelectedEdge(edge.id);
            setSelectedNode(null);
          }}
        />
        {/* Arrow marker */}
        <circle
          cx={endX}
          cy={endY}
          r={4}
          fill={isSelected ? '#3B82F6' : '#9CA3AF'}
        />
      </g>
    );
  };

  return (
    <div className="flex h-full">
      {/* Toolbar */}
      {!readOnly && (
        <div className="w-48 bg-gray-100 border-r p-4 overflow-y-auto">
          <h3 className="font-semibold text-gray-700 mb-4">Node Types</h3>
          <div className="space-y-2">
            {Object.entries(NODE_TYPES).map(([type, config]) => (
              <div
                key={type}
                draggable
                onDragStart={(e) => e.dataTransfer.setData('nodeType', type)}
                className="flex items-center gap-2 p-2 bg-white rounded border border-gray-200 cursor-grab hover:border-gray-400 hover:shadow-sm transition-all"
              >
                <span>{config.icon}</span>
                <span className="text-sm">{config.label}</span>
              </div>
            ))}
          </div>

          <h3 className="font-semibold text-gray-700 mt-6 mb-2">Zoom</h3>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setZoom((z) => Math.max(0.5, z - 0.1))}
              className="px-2 py-1 bg-white border rounded hover:bg-gray-50"
            >
              -
            </button>
            <span className="text-sm">{Math.round(zoom * 100)}%</span>
            <button
              onClick={() => setZoom((z) => Math.min(2, z + 0.1))}
              className="px-2 py-1 bg-white border rounded hover:bg-gray-50"
            >
              +
            </button>
          </div>

          {selectedNode && (
            <div className="mt-6">
              <h3 className="font-semibold text-gray-700 mb-2">Selected Node</h3>
              <button
                onClick={() => removeNode(selectedNode)}
                className="w-full px-3 py-2 bg-red-500 text-white rounded hover:bg-red-600"
              >
                Delete Node
              </button>
            </div>
          )}
        </div>
      )}

      {/* Canvas */}
      <div
        ref={canvasRef}
        className="flex-1 relative overflow-hidden bg-gray-50"
        style={{
          backgroundImage:
            'radial-gradient(circle, #d1d5db 1px, transparent 1px)',
          backgroundSize: `${20 * zoom}px ${20 * zoom}px`,
          backgroundPosition: `${pan.x}px ${pan.y}px`,
        }}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onMouseLeave={handleMouseUp}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onClick={() => {
          setSelectedNode(null);
          setSelectedEdge(null);
          setConnecting(null);
        }}
      >
        {/* Edges (SVG layer) */}
        <svg className="absolute inset-0 w-full h-full pointer-events-none">
          <g className="pointer-events-auto">
            {workflow.edges.map(renderEdge)}
          </g>
          {/* Connection preview */}
          {connecting && (
            <line
              x1={0}
              y1={0}
              x2={0}
              y2={0}
              stroke="#3B82F6"
              strokeWidth={2}
              strokeDasharray="5,5"
              className="connecting-line"
            />
          )}
        </svg>

        {/* Nodes */}
        {workflow.nodes.map(renderNode)}

        {/* Empty state */}
        {workflow.nodes.length === 0 && (
          <div className="absolute inset-0 flex items-center justify-center">
            <div className="text-center text-gray-400">
              <p className="text-lg">Drag nodes from the toolbar to get started</p>
              <p className="text-sm mt-2">Connect nodes by clicking output and input ports</p>
            </div>
          </div>
        )}
      </div>

      {/* Properties panel */}
      {selectedNode && !readOnly && (
        <div className="w-64 bg-white border-l p-4 overflow-y-auto">
          <NodePropertiesPanel
            node={workflow.nodes.find((n) => n.id === selectedNode)!}
            onUpdate={(config) => updateNodeConfig(selectedNode, config)}
            onNameChange={(name) => {
              setWorkflow((prev) => ({
                ...prev,
                nodes: prev.nodes.map((n) =>
                  n.id === selectedNode ? { ...n, name } : n
                ),
              }));
            }}
          />
        </div>
      )}
    </div>
  );
};

// Node Properties Panel
interface NodePropertiesPanelProps {
  node: WorkflowNode;
  onUpdate: (config: Record<string, unknown>) => void;
  onNameChange: (name: string) => void;
}

const NodePropertiesPanel: React.FC<NodePropertiesPanelProps> = ({
  node,
  onUpdate,
  onNameChange,
}) => {
  return (
    <div className="space-y-4">
      <h3 className="font-semibold text-gray-700">Properties</h3>

      <div>
        <label className="block text-sm font-medium text-gray-600 mb-1">Name</label>
        <input
          type="text"
          value={node.name}
          onChange={(e) => onNameChange(e.target.value)}
          className="w-full px-3 py-2 border rounded focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
        />
      </div>

      {/* Type-specific fields */}
      {node.type === 'trigger' && (
        <TriggerConfig config={node.config} onChange={onUpdate} />
      )}
      {node.type === 'job' && (
        <JobConfig config={node.config} onChange={onUpdate} />
      )}
      {node.type === 'condition' && (
        <ConditionConfig config={node.config} onChange={onUpdate} />
      )}
      {node.type === 'delay' && (
        <DelayConfig config={node.config} onChange={onUpdate} />
      )}
      {node.type === 'http' && (
        <HttpConfig config={node.config} onChange={onUpdate} />
      )}
      {node.type === 'notification' && (
        <NotificationConfig config={node.config} onChange={onUpdate} />
      )}
    </div>
  );
};

// Config components for different node types
const TriggerConfig: React.FC<{ config: Record<string, unknown>; onChange: (c: Record<string, unknown>) => void }> = ({ config, onChange }) => (
  <div className="space-y-3">
    <div>
      <label className="block text-sm font-medium text-gray-600 mb-1">Type</label>
      <select
        value={String(config.type || 'schedule')}
        onChange={(e) => onChange({ type: e.target.value })}
        className="w-full px-3 py-2 border rounded"
      >
        <option value="schedule">Schedule (Cron)</option>
        <option value="webhook">Webhook</option>
        <option value="event">Event</option>
        <option value="manual">Manual</option>
      </select>
    </div>
    {config.type === 'schedule' && (
      <div>
        <label className="block text-sm font-medium text-gray-600 mb-1">Schedule</label>
        <input
          type="text"
          placeholder="0 * * * *"
          value={String(config.schedule || '')}
          onChange={(e) => onChange({ schedule: e.target.value })}
          className="w-full px-3 py-2 border rounded"
        />
      </div>
    )}
  </div>
);

const JobConfig: React.FC<{ config: Record<string, unknown>; onChange: (c: Record<string, unknown>) => void }> = ({ config, onChange }) => (
  <div className="space-y-3">
    <div>
      <label className="block text-sm font-medium text-gray-600 mb-1">Job ID</label>
      <input
        type="text"
        placeholder="job-id"
        value={String(config.jobId || '')}
        onChange={(e) => onChange({ jobId: e.target.value })}
        className="w-full px-3 py-2 border rounded"
      />
    </div>
    <div>
      <label className="block text-sm font-medium text-gray-600 mb-1">Timeout</label>
      <input
        type="text"
        placeholder="5m"
        value={String(config.timeout || '')}
        onChange={(e) => onChange({ timeout: e.target.value })}
        className="w-full px-3 py-2 border rounded"
      />
    </div>
  </div>
);

const ConditionConfig: React.FC<{ config: Record<string, unknown>; onChange: (c: Record<string, unknown>) => void }> = ({ config, onChange }) => (
  <div className="space-y-3">
    <div>
      <label className="block text-sm font-medium text-gray-600 mb-1">Expression</label>
      <input
        type="text"
        placeholder="$.status == 'success'"
        value={String(config.expression || '')}
        onChange={(e) => onChange({ expression: e.target.value })}
        className="w-full px-3 py-2 border rounded"
      />
    </div>
  </div>
);

const DelayConfig: React.FC<{ config: Record<string, unknown>; onChange: (c: Record<string, unknown>) => void }> = ({ config, onChange }) => (
  <div className="space-y-3">
    <div>
      <label className="block text-sm font-medium text-gray-600 mb-1">Duration</label>
      <input
        type="text"
        placeholder="5m"
        value={String(config.duration || '')}
        onChange={(e) => onChange({ duration: e.target.value })}
        className="w-full px-3 py-2 border rounded"
      />
    </div>
  </div>
);

const HttpConfig: React.FC<{ config: Record<string, unknown>; onChange: (c: Record<string, unknown>) => void }> = ({ config, onChange }) => (
  <div className="space-y-3">
    <div>
      <label className="block text-sm font-medium text-gray-600 mb-1">URL</label>
      <input
        type="text"
        placeholder="https://api.example.com"
        value={String(config.url || '')}
        onChange={(e) => onChange({ url: e.target.value })}
        className="w-full px-3 py-2 border rounded"
      />
    </div>
    <div>
      <label className="block text-sm font-medium text-gray-600 mb-1">Method</label>
      <select
        value={String(config.method || 'GET')}
        onChange={(e) => onChange({ method: e.target.value })}
        className="w-full px-3 py-2 border rounded"
      >
        <option value="GET">GET</option>
        <option value="POST">POST</option>
        <option value="PUT">PUT</option>
        <option value="DELETE">DELETE</option>
      </select>
    </div>
  </div>
);

const NotificationConfig: React.FC<{ config: Record<string, unknown>; onChange: (c: Record<string, unknown>) => void }> = ({ config, onChange }) => (
  <div className="space-y-3">
    <div>
      <label className="block text-sm font-medium text-gray-600 mb-1">Channel</label>
      <select
        value={String(config.channel || 'slack')}
        onChange={(e) => onChange({ channel: e.target.value })}
        className="w-full px-3 py-2 border rounded"
      >
        <option value="slack">Slack</option>
        <option value="email">Email</option>
        <option value="webhook">Webhook</option>
        <option value="pagerduty">PagerDuty</option>
      </select>
    </div>
    <div>
      <label className="block text-sm font-medium text-gray-600 mb-1">Message</label>
      <textarea
        placeholder="Notification message..."
        value={String(config.message || '')}
        onChange={(e) => onChange({ message: e.target.value })}
        className="w-full px-3 py-2 border rounded"
        rows={3}
      />
    </div>
  </div>
);

// Helper functions
function getDefaultConfig(type: NodeType): Record<string, unknown> {
  switch (type) {
    case 'trigger':
      return { type: 'schedule', schedule: '0 * * * *' };
    case 'job':
      return { jobId: '', timeout: '5m' };
    case 'condition':
      return { expression: '' };
    case 'delay':
      return { duration: '1m' };
    case 'http':
      return { url: '', method: 'GET' };
    case 'notification':
      return { channel: 'slack', message: '' };
    default:
      return {};
  }
}

function getStatusColor(status: NodeStatus): string {
  switch (status) {
    case 'running':
      return '#3B82F6';
    case 'success':
      return '#10B981';
    case 'failed':
      return '#EF4444';
    case 'skipped':
      return '#9CA3AF';
    default:
      return '#F59E0B';
  }
}

function wouldCreateCycle(workflow: Workflow, source: string, target: string): boolean {
  // Simple DFS to check for cycles
  const visited = new Set<string>();
  const stack = [target];

  while (stack.length > 0) {
    const node = stack.pop()!;
    if (node === source) return true;
    if (visited.has(node)) continue;
    visited.add(node);

    // Find all edges where this node is the source
    for (const edge of workflow.edges) {
      if (edge.source === node) {
        stack.push(edge.target);
      }
    }
  }

  return false;
}

export default WorkflowBuilder;
