// Package dag provides graph visualization API for DAG workflows.
package dag

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// GraphAPIHandler handles DAG graph visualization API requests.
type GraphAPIHandler struct {
	store     Store
	engine    *Engine
	wsManager *WebSocketManager
	logger    zerolog.Logger
}

// NewGraphAPIHandler creates a new graph API handler.
func NewGraphAPIHandler(store Store, engine *Engine, logger zerolog.Logger) *GraphAPIHandler {
	return &GraphAPIHandler{
		store:     store,
		engine:    engine,
		wsManager: NewWebSocketManager(),
		logger:    logger.With().Str("component", "graph-api").Logger(),
	}
}

// GraphRoutes returns graph-specific API routes.
func (h *GraphAPIHandler) GraphRoutes() chi.Router {
	r := chi.NewRouter()

	// Graph data endpoints
	r.Get("/{workflowID}/graph", h.GetGraphData)
	r.Get("/{workflowID}/graph/layout", h.GetGraphLayout)
	r.Get("/{workflowID}/graph/critical-path", h.GetCriticalPath)
	r.Get("/{workflowID}/graph/insights", h.GetGraphInsights)

	// Run-specific graph data
	r.Get("/{workflowID}/runs/{runID}/graph", h.GetRunGraphData)
	r.Get("/{workflowID}/runs/{runID}/graph/timeline", h.GetRunTimeline)

	// WebSocket for real-time updates
	r.Get("/{workflowID}/runs/{runID}/ws", h.WebSocketHandler)

	return r
}

// GraphData represents the complete graph structure for visualization.
type GraphData struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Nodes       []*GraphNode      `json:"nodes"`
	Edges       []*GraphEdge      `json:"edges"`
	Metadata    *GraphMetadata    `json:"metadata"`
	Layout      *LayoutConfig     `json:"layout,omitempty"`
}

// GraphNode represents a node in the visualization graph.
type GraphNode struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Type         string              `json:"type"`
	Description  string              `json:"description,omitempty"`
	Position     *NodePosition       `json:"position,omitempty"`
	Status       string              `json:"status,omitempty"`
	Dependencies []string            `json:"dependencies"`
	Dependents   []string            `json:"dependents"`
	Level        int                 `json:"level"`
	Metrics      *NodeMetrics        `json:"metrics,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	OnCriticalPath bool              `json:"on_critical_path"`
}

// NodePosition represents x,y coordinates for rendering.
type NodePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NodeMetrics contains execution metrics for a node.
type NodeMetrics struct {
	AverageRuntime time.Duration `json:"average_runtime"`
	LastRuntime    time.Duration `json:"last_runtime,omitempty"`
	SuccessRate    float64       `json:"success_rate"`
	TotalRuns      int           `json:"total_runs"`
	LastRun        time.Time     `json:"last_run,omitempty"`
}

// GraphEdge represents an edge in the visualization graph.
type GraphEdge struct {
	ID         string `json:"id"`
	Source     string `json:"source"`
	Target     string `json:"target"`
	SourcePort string `json:"source_port,omitempty"`
	TargetPort string `json:"target_port,omitempty"`
	Type       string `json:"type"` // "dependency", "conditional", "data"
	Status     string `json:"status,omitempty"`
	OnCriticalPath bool `json:"on_critical_path"`
}

// GraphMetadata contains graph-level statistics.
type GraphMetadata struct {
	TotalNodes           int           `json:"total_nodes"`
	TotalEdges           int           `json:"total_edges"`
	MaxDepth             int           `json:"max_depth"`
	ParallelBranches     int           `json:"parallel_branches"`
	CriticalPathLength   int           `json:"critical_path_length"`
	EstimatedRuntime     time.Duration `json:"estimated_runtime"`
	AverageNodeFanout    float64       `json:"average_node_fanout"`
	HasCycles            bool          `json:"has_cycles"`
	LastUpdated          time.Time     `json:"last_updated"`
}

// LayoutConfig contains layout algorithm configuration.
type LayoutConfig struct {
	Algorithm     string  `json:"algorithm"` // "dagre", "elk", "hierarchical"
	Direction     string  `json:"direction"` // "TB", "BT", "LR", "RL"
	NodeSpacing   float64 `json:"node_spacing"`
	RankSpacing   float64 `json:"rank_spacing"`
	EdgeSpacing   float64 `json:"edge_spacing"`
	Padding       float64 `json:"padding"`
}

// DefaultLayoutConfig returns default layout settings.
func DefaultLayoutConfig() *LayoutConfig {
	return &LayoutConfig{
		Algorithm:   "dagre",
		Direction:   "TB",
		NodeSpacing: 50,
		RankSpacing: 80,
		EdgeSpacing: 20,
		Padding:     40,
	}
}

// CriticalPath represents the critical path through the DAG.
type CriticalPath struct {
	Nodes              []string      `json:"nodes"`
	Edges              []string      `json:"edges"`
	TotalDuration      time.Duration `json:"total_duration"`
	EstimatedRemaining time.Duration `json:"estimated_remaining,omitempty"`
	Bottlenecks        []*Bottleneck `json:"bottlenecks,omitempty"`
}

// Bottleneck identifies a bottleneck in the graph.
type Bottleneck struct {
	NodeID      string        `json:"node_id"`
	NodeName    string        `json:"node_name"`
	Reason      string        `json:"reason"`
	Impact      time.Duration `json:"impact"`
	Suggestions []string      `json:"suggestions,omitempty"`
}

// GraphInsights contains analytical insights about the graph.
type GraphInsights struct {
	Summary          string              `json:"summary"`
	CriticalPath     *CriticalPath       `json:"critical_path"`
	ParallelGroups   [][]string          `json:"parallel_groups"`
	BottleneckNodes  []string            `json:"bottleneck_nodes"`
	Recommendations  []*Recommendation   `json:"recommendations"`
	Statistics       *GraphStatistics    `json:"statistics"`
}

// Recommendation is an optimization recommendation.
type Recommendation struct {
	Type        string   `json:"type"` // "performance", "reliability", "structure"
	Priority    string   `json:"priority"` // "high", "medium", "low"
	Title       string   `json:"title"`
	Description string   `json:"description"`
	NodeIDs     []string `json:"node_ids,omitempty"`
}

// GraphStatistics contains detailed graph statistics.
type GraphStatistics struct {
	TotalRuns        int           `json:"total_runs"`
	SuccessfulRuns   int           `json:"successful_runs"`
	FailedRuns       int           `json:"failed_runs"`
	AverageRuntime   time.Duration `json:"average_runtime"`
	MinRuntime       time.Duration `json:"min_runtime"`
	MaxRuntime       time.Duration `json:"max_runtime"`
	SuccessRate      float64       `json:"success_rate"`
	MostFailedNode   string        `json:"most_failed_node,omitempty"`
	SlowestNode      string        `json:"slowest_node,omitempty"`
}

// RunGraphData contains graph data for a specific run.
type RunGraphData struct {
	*GraphData
	RunID           string           `json:"run_id"`
	RunStatus       string           `json:"run_status"`
	StartedAt       time.Time        `json:"started_at"`
	EndedAt         time.Time        `json:"ended_at,omitempty"`
	Progress        float64          `json:"progress"`
	CurrentNodes    []string         `json:"current_nodes"`
	CompletedNodes  []string         `json:"completed_nodes"`
	FailedNodes     []string         `json:"failed_nodes"`
	PendingNodes    []string         `json:"pending_nodes"`
	NodeExecutions  []*NodeExecution `json:"node_executions"`
}

// NodeExecution contains execution details for a node.
type NodeExecution struct {
	NodeID      string        `json:"node_id"`
	Status      string        `json:"status"`
	StartedAt   time.Time     `json:"started_at,omitempty"`
	EndedAt     time.Time     `json:"ended_at,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	Attempts    int           `json:"attempts"`
	Error       string        `json:"error,omitempty"`
	Logs        []string      `json:"logs,omitempty"`
}

// Timeline represents execution timeline for a run.
type Timeline struct {
	RunID     string           `json:"run_id"`
	StartTime time.Time        `json:"start_time"`
	EndTime   time.Time        `json:"end_time,omitempty"`
	Duration  time.Duration    `json:"duration,omitempty"`
	Events    []*TimelineEvent `json:"events"`
	Lanes     []*TimelineLane  `json:"lanes"`
}

// TimelineEvent represents an event in the timeline.
type TimelineEvent struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"node_id"`
	NodeName  string    `json:"node_name"`
	Type      string    `json:"type"` // "start", "end", "error", "retry"
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
	LaneIndex int       `json:"lane_index"`
}

// TimelineLane represents a parallel execution lane.
type TimelineLane struct {
	Index   int            `json:"index"`
	NodeIDs []string       `json:"node_ids"`
	Spans   []*TimelineSpan `json:"spans"`
}

// TimelineSpan represents a time span in a lane.
type TimelineSpan struct {
	NodeID    string        `json:"node_id"`
	NodeName  string        `json:"node_name"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
	Status    string        `json:"status"`
}

// GetGraphData handles GET /workflows/{workflowID}/graph.
func (h *GraphAPIHandler) GetGraphData(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}

	graphData := h.buildGraphData(workflow, nil)
	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: graphData})
}

// GetGraphLayout handles GET /workflows/{workflowID}/graph/layout.
func (h *GraphAPIHandler) GetGraphLayout(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}

	// Calculate automatic layout
	layout := h.calculateLayout(workflow)
	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: layout})
}

// GetCriticalPath handles GET /workflows/{workflowID}/graph/critical-path.
func (h *GraphAPIHandler) GetCriticalPath(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}

	criticalPath := h.calculateCriticalPath(workflow, nil)
	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: criticalPath})
}

// GetGraphInsights handles GET /workflows/{workflowID}/graph/insights.
func (h *GraphAPIHandler) GetGraphInsights(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}

	insights := h.analyzeGraph(workflow)
	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: insights})
}

// GetRunGraphData handles GET /workflows/{workflowID}/runs/{runID}/graph.
func (h *GraphAPIHandler) GetRunGraphData(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	runID := chi.URLParam(r, "runID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}

	run, err := h.engine.GetRun(runID)
	if err != nil {
		run, err = h.store.GetRun(r.Context(), runID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "run not found")
			return
		}
	}

	runGraphData := h.buildRunGraphData(workflow, run)
	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: runGraphData})
}

// GetRunTimeline handles GET /workflows/{workflowID}/runs/{runID}/graph/timeline.
func (h *GraphAPIHandler) GetRunTimeline(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	runID := chi.URLParam(r, "runID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}

	run, err := h.engine.GetRun(runID)
	if err != nil {
		run, err = h.store.GetRun(r.Context(), runID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "run not found")
			return
		}
	}

	timeline := h.buildTimeline(workflow, run)
	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: timeline})
}

// buildGraphData constructs the graph data structure.
func (h *GraphAPIHandler) buildGraphData(workflow *Workflow, run *Run) *GraphData {
	// Build dependency maps
	dependents := make(map[string][]string)
	for _, edge := range workflow.Edges {
		dependents[edge.SourceNode] = append(dependents[edge.SourceNode], edge.TargetNode)
	}

	// Calculate node levels
	levels := h.calculateNodeLevels(workflow)

	// Build nodes
	nodes := make([]*GraphNode, 0, len(workflow.Nodes))
	for nodeID, node := range workflow.Nodes {
		deps := []string{}
		for _, edge := range workflow.Edges {
			if edge.TargetNode == nodeID {
				deps = append(deps, edge.SourceNode)
			}
		}

		config := make(map[string]interface{})
		if node.Config != nil {
			json.Unmarshal(node.Config, &config)
		}

		gNode := &GraphNode{
			ID:           nodeID,
			Name:         node.Name,
			Type:         string(node.Type),
			Description:  node.Description,
			Position:     &NodePosition{X: node.Position.X, Y: node.Position.Y},
			Dependencies: deps,
			Dependents:   dependents[nodeID],
			Level:        levels[nodeID],
			Config:       config,
		}

		// Add run status if available
		if run != nil {
			if state, ok := run.NodeStates[nodeID]; ok {
				gNode.Status = string(state.Status)
			}
		}

		nodes = append(nodes, gNode)
	}

	// Build edges
	edges := make([]*GraphEdge, 0, len(workflow.Edges))
	for _, edge := range workflow.Edges {
		gEdge := &GraphEdge{
			ID:         edge.ID,
			Source:     edge.SourceNode,
			Target:     edge.TargetNode,
			SourcePort: edge.SourcePort,
			TargetPort: edge.TargetPort,
			Type:       "dependency",
		}

		if run != nil {
			if state, ok := run.NodeStates[edge.SourceNode]; ok {
				gEdge.Status = string(state.Status)
			}
		}

		edges = append(edges, gEdge)
	}

	// Calculate metadata
	metadata := h.calculateMetadata(workflow, nodes, edges)

	return &GraphData{
		ID:          workflow.ID,
		Name:        workflow.Name,
		Description: workflow.Description,
		Nodes:       nodes,
		Edges:       edges,
		Metadata:    metadata,
		Layout:      DefaultLayoutConfig(),
	}
}

// calculateNodeLevels computes the level (depth) of each node.
func (h *GraphAPIHandler) calculateNodeLevels(workflow *Workflow) map[string]int {
	levels := make(map[string]int)
	
	// Build adjacency list
	deps := make(map[string][]string)
	for _, edge := range workflow.Edges {
		deps[edge.TargetNode] = append(deps[edge.TargetNode], edge.SourceNode)
	}

	// Find root nodes (no dependencies)
	roots := []string{}
	for nodeID := range workflow.Nodes {
		if len(deps[nodeID]) == 0 {
			roots = append(roots, nodeID)
			levels[nodeID] = 0
		}
	}

	// BFS to assign levels
	queue := roots
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		currentLevel := levels[current]

		// Find dependent nodes
		for nodeID := range workflow.Nodes {
			for _, edge := range workflow.Edges {
				if edge.SourceNode == current && edge.TargetNode == nodeID {
					newLevel := currentLevel + 1
					if existingLevel, exists := levels[nodeID]; !exists || newLevel > existingLevel {
						levels[nodeID] = newLevel
						queue = append(queue, nodeID)
					}
				}
			}
		}
	}

	return levels
}

// calculateMetadata computes graph metadata.
func (h *GraphAPIHandler) calculateMetadata(workflow *Workflow, nodes []*GraphNode, edges []*GraphEdge) *GraphMetadata {
	maxDepth := 0
	totalFanout := 0

	for _, node := range nodes {
		if node.Level > maxDepth {
			maxDepth = node.Level
		}
		totalFanout += len(node.Dependents)
	}

	avgFanout := 0.0
	if len(nodes) > 0 {
		avgFanout = float64(totalFanout) / float64(len(nodes))
	}

	// Count parallel branches at each level
	levelCounts := make(map[int]int)
	for _, node := range nodes {
		levelCounts[node.Level]++
	}

	maxParallel := 0
	for _, count := range levelCounts {
		if count > maxParallel {
			maxParallel = count
		}
	}

	return &GraphMetadata{
		TotalNodes:         len(nodes),
		TotalEdges:         len(edges),
		MaxDepth:           maxDepth,
		ParallelBranches:   maxParallel,
		AverageNodeFanout:  avgFanout,
		HasCycles:          false, // Would be caught by DAG validation
		LastUpdated:        workflow.UpdatedAt,
	}
}

// calculateLayout computes automatic node positions.
func (h *GraphAPIHandler) calculateLayout(workflow *Workflow) map[string]*NodePosition {
	levels := h.calculateNodeLevels(workflow)
	positions := make(map[string]*NodePosition)

	// Group nodes by level
	levelNodes := make(map[int][]string)
	for nodeID, level := range levels {
		levelNodes[level] = append(levelNodes[level], nodeID)
	}

	// Sort levels
	sortedLevels := make([]int, 0, len(levelNodes))
	for level := range levelNodes {
		sortedLevels = append(sortedLevels, level)
	}
	sort.Ints(sortedLevels)

	config := DefaultLayoutConfig()
	
	// Assign positions
	for _, level := range sortedLevels {
		nodeIDs := levelNodes[level]
		sort.Strings(nodeIDs) // Consistent ordering
		
		y := config.Padding + float64(level)*config.RankSpacing
		
		totalWidth := float64(len(nodeIDs)-1) * config.NodeSpacing
		startX := config.Padding + (500-totalWidth)/2 // Center in 500px canvas

		for i, nodeID := range nodeIDs {
			positions[nodeID] = &NodePosition{
				X: startX + float64(i)*config.NodeSpacing,
				Y: y,
			}
		}
	}

	return positions
}

// calculateCriticalPath finds the longest path through the DAG.
func (h *GraphAPIHandler) calculateCriticalPath(workflow *Workflow, nodeMetrics map[string]*NodeMetrics) *CriticalPath {
	// Build adjacency and reverse adjacency lists
	outEdges := make(map[string][]string)
	for _, edge := range workflow.Edges {
		outEdges[edge.SourceNode] = append(outEdges[edge.SourceNode], edge.TargetNode)
	}

	// Topological sort
	visited := make(map[string]bool)
	order := []string{}

	var dfs func(nodeID string)
	dfs = func(nodeID string) {
		if visited[nodeID] {
			return
		}
		visited[nodeID] = true
		for _, next := range outEdges[nodeID] {
			dfs(next)
		}
		order = append(order, nodeID)
	}

	for nodeID := range workflow.Nodes {
		dfs(nodeID)
	}

	// Reverse for topological order
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}

	// Calculate longest path using dynamic programming
	dist := make(map[string]time.Duration)
	parent := make(map[string]string)

	getNodeDuration := func(nodeID string) time.Duration {
		if nodeMetrics != nil {
			if m, ok := nodeMetrics[nodeID]; ok {
				return m.AverageRuntime
			}
		}
		return 30 * time.Second // Default estimate
	}

	for _, nodeID := range order {
		nodeDuration := getNodeDuration(nodeID)
		dist[nodeID] = nodeDuration

		for _, next := range outEdges[nodeID] {
			newDist := dist[nodeID] + getNodeDuration(next)
			if newDist > dist[next] {
				dist[next] = newDist
				parent[next] = nodeID
			}
		}
	}

	// Find end node with max distance
	var endNode string
	maxDist := time.Duration(0)
	for nodeID, d := range dist {
		if d > maxDist {
			maxDist = d
			endNode = nodeID
		}
	}

	// Reconstruct path
	path := []string{}
	current := endNode
	for current != "" {
		path = append([]string{current}, path...)
		current = parent[current]
	}

	// Build edge list for critical path
	edgeIDs := []string{}
	for i := 0; i < len(path)-1; i++ {
		for _, edge := range workflow.Edges {
			if edge.SourceNode == path[i] && edge.TargetNode == path[i+1] {
				edgeIDs = append(edgeIDs, edge.ID)
				break
			}
		}
	}

	return &CriticalPath{
		Nodes:         path,
		Edges:         edgeIDs,
		TotalDuration: maxDist,
	}
}

// analyzeGraph provides insights about the graph.
func (h *GraphAPIHandler) analyzeGraph(workflow *Workflow) *GraphInsights {
	criticalPath := h.calculateCriticalPath(workflow, nil)
	parallelGroups := h.findParallelGroups(workflow)

	recommendations := []*Recommendation{}

	// Check for single-threaded execution
	maxParallel := 0
	for _, group := range parallelGroups {
		if len(group) > maxParallel {
			maxParallel = len(group)
		}
	}

	if maxParallel == 1 && len(workflow.Nodes) > 3 {
		recommendations = append(recommendations, &Recommendation{
			Type:        "performance",
			Priority:    "high",
			Title:       "Sequential Execution Pattern",
			Description: "The workflow executes nodes sequentially. Consider parallelizing independent tasks.",
		})
	}

	// Check for fan-out/fan-in patterns
	for _, node := range workflow.Nodes {
		fanout := 0
		for _, edge := range workflow.Edges {
			if edge.SourceNode == node.ID {
				fanout++
			}
		}
		if fanout > 5 {
			recommendations = append(recommendations, &Recommendation{
				Type:        "structure",
				Priority:    "medium",
				Title:       "High Fan-out Node",
				Description: "Node '" + node.Name + "' has " + string(rune(fanout+'0')) + " dependents. Consider grouping related tasks.",
				NodeIDs:     []string{node.ID},
			})
		}
	}

	summary := h.generateSummary(workflow, criticalPath, parallelGroups)

	return &GraphInsights{
		Summary:         summary,
		CriticalPath:    criticalPath,
		ParallelGroups:  parallelGroups,
		Recommendations: recommendations,
		Statistics:      &GraphStatistics{},
	}
}

// findParallelGroups identifies groups of nodes that can run in parallel.
func (h *GraphAPIHandler) findParallelGroups(workflow *Workflow) [][]string {
	levels := h.calculateNodeLevels(workflow)
	
	levelGroups := make(map[int][]string)
	for nodeID, level := range levels {
		levelGroups[level] = append(levelGroups[level], nodeID)
	}

	// Sort by level
	sortedLevels := make([]int, 0, len(levelGroups))
	for level := range levelGroups {
		sortedLevels = append(sortedLevels, level)
	}
	sort.Ints(sortedLevels)

	groups := make([][]string, 0, len(sortedLevels))
	for _, level := range sortedLevels {
		groups = append(groups, levelGroups[level])
	}

	return groups
}

// generateSummary creates a human-readable summary.
func (h *GraphAPIHandler) generateSummary(workflow *Workflow, cp *CriticalPath, groups [][]string) string {
	totalNodes := len(workflow.Nodes)
	totalEdges := len(workflow.Edges)
	criticalLen := len(cp.Nodes)
	maxParallel := 0
	for _, g := range groups {
		if len(g) > maxParallel {
			maxParallel = len(g)
		}
	}

	return "Workflow has " + itoa(totalNodes) + " nodes and " + itoa(totalEdges) + 
		" edges. Critical path contains " + itoa(criticalLen) + 
		" nodes. Maximum parallelism is " + itoa(maxParallel) + " concurrent nodes."
}

// buildRunGraphData builds graph data with run execution information.
func (h *GraphAPIHandler) buildRunGraphData(workflow *Workflow, run *Run) *RunGraphData {
	graphData := h.buildGraphData(workflow, run)
	criticalPath := h.calculateCriticalPath(workflow, nil)

	// Mark critical path nodes and edges
	cpNodes := make(map[string]bool)
	for _, nodeID := range criticalPath.Nodes {
		cpNodes[nodeID] = true
	}
	cpEdges := make(map[string]bool)
	for _, edgeID := range criticalPath.Edges {
		cpEdges[edgeID] = true
	}

	for _, node := range graphData.Nodes {
		node.OnCriticalPath = cpNodes[node.ID]
	}
	for _, edge := range graphData.Edges {
		edge.OnCriticalPath = cpEdges[edge.ID]
	}

	// Categorize nodes by status
	var currentNodes, completedNodes, failedNodes, pendingNodes []string
	for nodeID, state := range run.NodeStates {
		switch state.Status {
		case NodeRunning:
			currentNodes = append(currentNodes, nodeID)
		case NodeSuccess:
			completedNodes = append(completedNodes, nodeID)
		case NodeFailed:
			failedNodes = append(failedNodes, nodeID)
		case NodePending:
			pendingNodes = append(pendingNodes, nodeID)
		}
	}

	// Build node executions
	executions := make([]*NodeExecution, 0, len(run.NodeStates))
	for nodeID, state := range run.NodeStates {
		executions = append(executions, &NodeExecution{
			NodeID:    nodeID,
			Status:    string(state.Status),
			StartedAt: state.StartedAt,
			EndedAt:   state.EndedAt,
			Duration:  state.Duration,
			Attempts:  state.Attempts,
			Error:     state.Error,
		})
	}

	// Calculate progress
	progress := 0.0
	if len(workflow.Nodes) > 0 {
		completed := len(completedNodes) + len(failedNodes)
		progress = float64(completed) / float64(len(workflow.Nodes))
	}

	return &RunGraphData{
		GraphData:      graphData,
		RunID:          run.ID,
		RunStatus:      string(run.Status),
		StartedAt:      run.StartedAt,
		EndedAt:        run.EndedAt,
		Progress:       progress,
		CurrentNodes:   currentNodes,
		CompletedNodes: completedNodes,
		FailedNodes:    failedNodes,
		PendingNodes:   pendingNodes,
		NodeExecutions: executions,
	}
}

// buildTimeline builds an execution timeline.
func (h *GraphAPIHandler) buildTimeline(workflow *Workflow, run *Run) *Timeline {
	events := []*TimelineEvent{}
	
	// Sort node states by start time
	type nodeStart struct {
		nodeID string
		state  *NodeState
	}
	starts := make([]nodeStart, 0, len(run.NodeStates))
	for nodeID, state := range run.NodeStates {
		if !state.StartedAt.IsZero() {
			starts = append(starts, nodeStart{nodeID, state})
		}
	}
	sort.Slice(starts, func(i, j int) bool {
		return starts[i].state.StartedAt.Before(starts[j].state.StartedAt)
	})

	// Assign lanes (parallel execution tracks)
	laneEndTimes := []time.Time{}
	nodeLanes := make(map[string]int)

	for _, ns := range starts {
		// Find available lane
		lane := -1
		for i, endTime := range laneEndTimes {
			if ns.state.StartedAt.After(endTime) || ns.state.StartedAt.Equal(endTime) {
				lane = i
				break
			}
		}
		if lane == -1 {
			lane = len(laneEndTimes)
			laneEndTimes = append(laneEndTimes, time.Time{})
		}

		nodeLanes[ns.nodeID] = lane
		if !ns.state.EndedAt.IsZero() {
			laneEndTimes[lane] = ns.state.EndedAt
		} else {
			laneEndTimes[lane] = time.Now()
		}

		node := workflow.Nodes[ns.nodeID]
		nodeName := ns.nodeID
		if node != nil {
			nodeName = node.Name
		}

		// Add start event
		events = append(events, &TimelineEvent{
			ID:        uuid.New().String(),
			NodeID:    ns.nodeID,
			NodeName:  nodeName,
			Type:      "start",
			Timestamp: ns.state.StartedAt,
			LaneIndex: lane,
		})

		// Add end event
		if !ns.state.EndedAt.IsZero() {
			eventType := "end"
			message := ""
			if ns.state.Status == NodeFailed {
				eventType = "error"
				message = ns.state.Error
			}
			events = append(events, &TimelineEvent{
				ID:        uuid.New().String(),
				NodeID:    ns.nodeID,
				NodeName:  nodeName,
				Type:      eventType,
				Timestamp: ns.state.EndedAt,
				Message:   message,
				LaneIndex: lane,
			})
		}
	}

	// Build lanes
	lanes := make([]*TimelineLane, len(laneEndTimes))
	for i := range lanes {
		lanes[i] = &TimelineLane{
			Index:   i,
			NodeIDs: []string{},
			Spans:   []*TimelineSpan{},
		}
	}

	for nodeID, lane := range nodeLanes {
		state := run.NodeStates[nodeID]
		node := workflow.Nodes[nodeID]
		nodeName := nodeID
		if node != nil {
			nodeName = node.Name
		}

		lanes[lane].NodeIDs = append(lanes[lane].NodeIDs, nodeID)
		lanes[lane].Spans = append(lanes[lane].Spans, &TimelineSpan{
			NodeID:    nodeID,
			NodeName:  nodeName,
			StartTime: state.StartedAt,
			EndTime:   state.EndedAt,
			Duration:  state.Duration,
			Status:    string(state.Status),
		})
	}

	timeline := &Timeline{
		RunID:     run.ID,
		StartTime: run.StartedAt,
		EndTime:   run.EndedAt,
		Events:    events,
		Lanes:     lanes,
	}

	if !run.EndedAt.IsZero() {
		timeline.Duration = run.EndedAt.Sub(run.StartedAt)
	}

	return timeline
}

// WebSocket support for real-time updates

// WebSocketManager manages WebSocket connections.
type WebSocketManager struct {
	mu          sync.RWMutex
	connections map[string]map[string]*WSConnection
}

// WSConnection represents a WebSocket connection.
type WSConnection struct {
	ID       string
	RunID    string
	SendChan chan []byte
}

// NewWebSocketManager creates a new WebSocket manager.
func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		connections: make(map[string]map[string]*WSConnection),
	}
}

// WebSocketHandler handles WebSocket connections.
func (h *GraphAPIHandler) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	// Note: This is a placeholder. Full WebSocket implementation would use gorilla/websocket
	// For now, return upgrade required error for documentation purposes
	h.writeError(w, http.StatusUpgradeRequired, "WEBSOCKET_REQUIRED", 
		"WebSocket connection required. Use ws:// protocol.")
}

// BroadcastRunUpdate sends run update to all connected clients.
func (h *GraphAPIHandler) BroadcastRunUpdate(runID string, data interface{}) {
	h.wsManager.mu.RLock()
	defer h.wsManager.mu.RUnlock()

	conns := h.wsManager.connections[runID]
	if conns == nil {
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	for _, conn := range conns {
		select {
		case conn.SendChan <- jsonData:
		default:
			// Channel full, skip
		}
	}
}

// Helper functions

func (h *GraphAPIHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *GraphAPIHandler) writeError(w http.ResponseWriter, status int, code, message string) {
	h.writeJSON(w, status, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// Ensure imports are used
var (
	_ = context.Background
	_ = math.Max
)
