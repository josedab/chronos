// Package dag provides visual workflow builder API for Chronos.
package dag

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Builder errors.
var (
	ErrInvalidNodeType    = errors.New("invalid node type")
	ErrNodeAlreadyExists  = errors.New("node already exists")
	ErrEdgeCreatesCircle  = errors.New("edge would create a cycle")
	ErrInvalidPosition    = errors.New("invalid node position")
)

// WorkflowBuilder provides a visual workflow builder interface.
type WorkflowBuilder struct {
	workflow *Workflow
}

// Workflow represents a visual workflow definition.
type Workflow struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Version     int                    `json:"version"`
	Nodes       map[string]*WorkflowNode `json:"nodes"`
	Edges       []*WorkflowEdge        `json:"edges"`
	Variables   map[string]*Variable   `json:"variables,omitempty"`
	Settings    *WorkflowSettings      `json:"settings,omitempty"`
	Canvas      *CanvasSettings        `json:"canvas,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CreatedBy   string                 `json:"created_by,omitempty"`
}

// WorkflowNode represents a node in the visual workflow.
type WorkflowNode struct {
	ID          string                 `json:"id"`
	Type        NodeType               `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Config      json.RawMessage        `json:"config"`
	Position    Position               `json:"position"`
	Style       *NodeStyle             `json:"style,omitempty"`
	Inputs      []*NodePort            `json:"inputs,omitempty"`
	Outputs     []*NodePort            `json:"outputs,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NodeType represents the type of workflow node.
type NodeType string

const (
	NodeTypeTrigger     NodeType = "trigger"
	NodeTypeJob         NodeType = "job"
	NodeTypeCondition   NodeType = "condition"
	NodeTypeParallel    NodeType = "parallel"
	NodeTypeJoin        NodeType = "join"
	NodeTypeDelay       NodeType = "delay"
	NodeTypeApproval    NodeType = "approval"
	NodeTypeNotification NodeType = "notification"
	NodeTypeSubWorkflow NodeType = "sub_workflow"
	NodeTypeScript      NodeType = "script"
	NodeTypeHTTP        NodeType = "http"
	NodeTypeTransform   NodeType = "transform"
)

// Position represents a node's position on the canvas.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NodeStyle represents visual styling for a node.
type NodeStyle struct {
	Color      string `json:"color,omitempty"`
	Icon       string `json:"icon,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Collapsed  bool   `json:"collapsed,omitempty"`
}

// NodePort represents an input or output port on a node.
type NodePort struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // "default", "success", "failure", "any"
	Required bool   `json:"required,omitempty"`
}

// WorkflowEdge represents a connection between nodes.
type WorkflowEdge struct {
	ID         string `json:"id"`
	SourceNode string `json:"source_node"`
	SourcePort string `json:"source_port"`
	TargetNode string `json:"target_node"`
	TargetPort string `json:"target_port"`
	Condition  string `json:"condition,omitempty"` // For conditional edges
	Label      string `json:"label,omitempty"`
}

// Variable represents a workflow variable.
type Variable struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // string, number, boolean, secret, expression
	Value       interface{} `json:"value,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required,omitempty"`
}

// WorkflowSettings contains workflow-level settings.
type WorkflowSettings struct {
	Timeout           string `json:"timeout,omitempty"`
	RetryPolicy       *RetryPolicy `json:"retry_policy,omitempty"`
	ConcurrencyPolicy string `json:"concurrency_policy,omitempty"` // allow, forbid, replace
	OnFailure         string `json:"on_failure,omitempty"` // stop, continue, rollback
	EnableLogging     bool   `json:"enable_logging"`
}

// RetryPolicy for workflows.
type RetryPolicy struct {
	MaxAttempts     int    `json:"max_attempts"`
	InitialInterval string `json:"initial_interval"`
	MaxInterval     string `json:"max_interval"`
	Multiplier      float64 `json:"multiplier"`
}

// CanvasSettings contains canvas display settings.
type CanvasSettings struct {
	Zoom     float64  `json:"zoom"`
	OffsetX  float64  `json:"offset_x"`
	OffsetY  float64  `json:"offset_y"`
	GridSize int      `json:"grid_size"`
	SnapToGrid bool   `json:"snap_to_grid"`
}

// TriggerConfig configures a trigger node.
type TriggerConfig struct {
	Type       string `json:"type"` // schedule, webhook, manual, event
	Schedule   string `json:"schedule,omitempty"`
	Timezone   string `json:"timezone,omitempty"`
	WebhookURL string `json:"webhook_url,omitempty"`
	EventType  string `json:"event_type,omitempty"`
}

// JobConfig configures a job execution node.
type JobConfig struct {
	JobID       string            `json:"job_id,omitempty"`
	WebhookURL  string            `json:"webhook_url,omitempty"`
	Method      string            `json:"method,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`
	RetryPolicy *RetryPolicy      `json:"retry_policy,omitempty"`
}

// ConditionConfig configures a condition node.
type ConditionConfig struct {
	Expression string          `json:"expression"`
	Branches   []ConditionBranch `json:"branches"`
}

// ConditionBranch represents a condition branch.
type ConditionBranch struct {
	Name       string `json:"name"`
	Condition  string `json:"condition"`
	OutputPort string `json:"output_port"`
}

// ParallelConfig configures a parallel execution node.
type ParallelConfig struct {
	MaxConcurrency int  `json:"max_concurrency,omitempty"`
	FailFast       bool `json:"fail_fast"` // Stop all if one fails
}

// JoinConfig configures a join node.
type JoinConfig struct {
	Mode    string `json:"mode"` // all, any, n_of_m
	Count   int    `json:"count,omitempty"` // For n_of_m mode
	Timeout string `json:"timeout,omitempty"`
}

// DelayConfig configures a delay node.
type DelayConfig struct {
	Duration string    `json:"duration,omitempty"`
	Until    time.Time `json:"until,omitempty"`
}

// ApprovalConfig configures an approval node.
type ApprovalConfig struct {
	Approvers []string `json:"approvers"` // User IDs or group names
	Timeout   string   `json:"timeout,omitempty"`
	Message   string   `json:"message,omitempty"`
	MinApprovals int   `json:"min_approvals,omitempty"`
}

// NotificationConfig configures a notification node.
type NotificationConfig struct {
	Channel   string            `json:"channel"` // slack, email, webhook, pagerduty
	Target    string            `json:"target"`  // Channel ID, email, URL
	Message   string            `json:"message"`
	Template  string            `json:"template,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
}

// ScriptConfig configures a script node.
type ScriptConfig struct {
	Language string `json:"language"` // javascript, python, bash
	Code     string `json:"code"`
	Timeout  string `json:"timeout,omitempty"`
}

// TransformConfig configures a data transformation node.
type TransformConfig struct {
	Type     string `json:"type"` // jq, jsonpath, template
	Expression string `json:"expression"`
	InputVar  string `json:"input_var,omitempty"`
	OutputVar string `json:"output_var,omitempty"`
}

// NewWorkflowBuilder creates a new workflow builder.
func NewWorkflowBuilder(name string) *WorkflowBuilder {
	return &WorkflowBuilder{
		workflow: &Workflow{
			ID:        uuid.New().String(),
			Name:      name,
			Version:   1,
			Nodes:     make(map[string]*WorkflowNode),
			Edges:     make([]*WorkflowEdge, 0),
			Variables: make(map[string]*Variable),
			Settings:  &WorkflowSettings{EnableLogging: true},
			Canvas: &CanvasSettings{
				Zoom:       1.0,
				GridSize:   20,
				SnapToGrid: true,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

// LoadWorkflow loads an existing workflow for editing.
func LoadWorkflow(workflow *Workflow) *WorkflowBuilder {
	return &WorkflowBuilder{workflow: workflow}
}

// AddNode adds a node to the workflow.
func (b *WorkflowBuilder) AddNode(nodeType NodeType, name string, position Position, config interface{}) (*WorkflowNode, error) {
	if !isValidNodeType(nodeType) {
		return nil, ErrInvalidNodeType
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	node := &WorkflowNode{
		ID:       uuid.New().String(),
		Type:     nodeType,
		Name:     name,
		Config:   configJSON,
		Position: position,
		Style:    getDefaultStyle(nodeType),
		Inputs:   getDefaultInputs(nodeType),
		Outputs:  getDefaultOutputs(nodeType),
		Metadata: make(map[string]interface{}),
	}

	b.workflow.Nodes[node.ID] = node
	b.workflow.UpdatedAt = time.Now()

	return node, nil
}

// UpdateNode updates an existing node.
func (b *WorkflowBuilder) UpdateNode(nodeID string, updates *NodeUpdate) error {
	node, exists := b.workflow.Nodes[nodeID]
	if !exists {
		return ErrNodeNotFound
	}

	if updates.Name != "" {
		node.Name = updates.Name
	}
	if updates.Description != "" {
		node.Description = updates.Description
	}
	if updates.Config != nil {
		configJSON, err := json.Marshal(updates.Config)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}
		node.Config = configJSON
	}
	if updates.Position != nil {
		node.Position = *updates.Position
	}
	if updates.Style != nil {
		node.Style = updates.Style
	}

	b.workflow.UpdatedAt = time.Now()
	return nil
}

// NodeUpdate contains fields to update on a node.
type NodeUpdate struct {
	Name        string       `json:"name,omitempty"`
	Description string       `json:"description,omitempty"`
	Config      interface{}  `json:"config,omitempty"`
	Position    *Position    `json:"position,omitempty"`
	Style       *NodeStyle   `json:"style,omitempty"`
}

// RemoveNode removes a node and its connected edges.
func (b *WorkflowBuilder) RemoveNode(nodeID string) error {
	if _, exists := b.workflow.Nodes[nodeID]; !exists {
		return ErrNodeNotFound
	}

	// Remove connected edges
	newEdges := make([]*WorkflowEdge, 0)
	for _, edge := range b.workflow.Edges {
		if edge.SourceNode != nodeID && edge.TargetNode != nodeID {
			newEdges = append(newEdges, edge)
		}
	}
	b.workflow.Edges = newEdges

	delete(b.workflow.Nodes, nodeID)
	b.workflow.UpdatedAt = time.Now()

	return nil
}

// AddEdge adds an edge between two nodes.
func (b *WorkflowBuilder) AddEdge(sourceNode, sourcePort, targetNode, targetPort string) (*WorkflowEdge, error) {
	// Validate nodes exist
	if _, exists := b.workflow.Nodes[sourceNode]; !exists {
		return nil, fmt.Errorf("source node not found: %s", sourceNode)
	}
	if _, exists := b.workflow.Nodes[targetNode]; !exists {
		return nil, fmt.Errorf("target node not found: %s", targetNode)
	}

	// Check for cycles
	if b.wouldCreateCycle(sourceNode, targetNode) {
		return nil, ErrEdgeCreatesCircle
	}

	edge := &WorkflowEdge{
		ID:         uuid.New().String(),
		SourceNode: sourceNode,
		SourcePort: sourcePort,
		TargetNode: targetNode,
		TargetPort: targetPort,
	}

	b.workflow.Edges = append(b.workflow.Edges, edge)
	b.workflow.UpdatedAt = time.Now()

	return edge, nil
}

// RemoveEdge removes an edge.
func (b *WorkflowBuilder) RemoveEdge(edgeID string) error {
	newEdges := make([]*WorkflowEdge, 0, len(b.workflow.Edges)-1)
	found := false

	for _, edge := range b.workflow.Edges {
		if edge.ID == edgeID {
			found = true
		} else {
			newEdges = append(newEdges, edge)
		}
	}

	if !found {
		return errors.New("edge not found")
	}

	b.workflow.Edges = newEdges
	b.workflow.UpdatedAt = time.Now()

	return nil
}

// AddVariable adds a workflow variable.
func (b *WorkflowBuilder) AddVariable(name, varType string, value interface{}, required bool) error {
	b.workflow.Variables[name] = &Variable{
		Name:     name,
		Type:     varType,
		Value:    value,
		Required: required,
	}
	b.workflow.UpdatedAt = time.Now()
	return nil
}

// SetSettings updates workflow settings.
func (b *WorkflowBuilder) SetSettings(settings *WorkflowSettings) {
	b.workflow.Settings = settings
	b.workflow.UpdatedAt = time.Now()
}

// UpdateCanvas updates canvas settings.
func (b *WorkflowBuilder) UpdateCanvas(canvas *CanvasSettings) {
	b.workflow.Canvas = canvas
}

// Build builds and validates the workflow.
func (b *WorkflowBuilder) Build() (*Workflow, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}

	b.workflow.Version++
	b.workflow.UpdatedAt = time.Now()

	return b.workflow, nil
}

// GetWorkflow returns the current workflow state.
func (b *WorkflowBuilder) GetWorkflow() *Workflow {
	return b.workflow
}

// ToDAG converts the workflow to an executable DAG.
func (b *WorkflowBuilder) ToDAG() (*DAG, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}

	dag := &DAG{
		ID:          b.workflow.ID,
		Name:        b.workflow.Name,
		Description: b.workflow.Description,
		Nodes:       make(map[string]*Node),
		RootNodes:   make([]string, 0),
		Metadata: map[string]string{
			"workflow_version": fmt.Sprintf("%d", b.workflow.Version),
		},
		CreatedAt: b.workflow.CreatedAt,
		UpdatedAt: time.Now(),
	}

	// Build adjacency list for dependencies
	incomingEdges := make(map[string][]string)
	for _, edge := range b.workflow.Edges {
		incomingEdges[edge.TargetNode] = append(incomingEdges[edge.TargetNode], edge.SourceNode)
	}

	// Convert workflow nodes to DAG nodes
	for id, wNode := range b.workflow.Nodes {
		if wNode.Type == NodeTypeTrigger {
			dag.RootNodes = append(dag.RootNodes, id)
			continue
		}

		node := &Node{
			ID:           id,
			Name:         wNode.Name,
			Dependencies: incomingEdges[id],
			Condition:    ConditionAllSuccess,
			Metadata: map[string]string{
				"node_type": string(wNode.Type),
			},
		}

		// Parse timeout from config if applicable
		if wNode.Type == NodeTypeJob {
			var cfg JobConfig
			if err := json.Unmarshal(wNode.Config, &cfg); err == nil {
				node.JobID = cfg.JobID
			}
		}

		dag.Nodes[id] = node
	}

	return dag, nil
}

// validate validates the workflow.
func (b *WorkflowBuilder) validate() error {
	if b.workflow.Name == "" {
		return errors.New("workflow name is required")
	}

	if len(b.workflow.Nodes) == 0 {
		return errors.New("workflow must have at least one node")
	}

	// Check for triggers
	hasTrigger := false
	for _, node := range b.workflow.Nodes {
		if node.Type == NodeTypeTrigger {
			hasTrigger = true
			break
		}
	}

	if !hasTrigger {
		return errors.New("workflow must have at least one trigger")
	}

	// Validate edges reference valid nodes
	for _, edge := range b.workflow.Edges {
		if _, exists := b.workflow.Nodes[edge.SourceNode]; !exists {
			return fmt.Errorf("edge references non-existent source node: %s", edge.SourceNode)
		}
		if _, exists := b.workflow.Nodes[edge.TargetNode]; !exists {
			return fmt.Errorf("edge references non-existent target node: %s", edge.TargetNode)
		}
	}

	// Check for cycles
	if b.hasCycle() {
		return ErrCycleDetected
	}

	return nil
}

// wouldCreateCycle checks if adding an edge would create a cycle.
func (b *WorkflowBuilder) wouldCreateCycle(from, to string) bool {
	// Use DFS to check if there's a path from 'to' back to 'from'
	visited := make(map[string]bool)
	return b.hasPath(to, from, visited)
}

// hasPath checks if there's a path from source to target using DFS.
func (b *WorkflowBuilder) hasPath(source, target string, visited map[string]bool) bool {
	if source == target {
		return true
	}

	visited[source] = true

	// Get outgoing edges from source
	for _, edge := range b.workflow.Edges {
		if edge.SourceNode == source && !visited[edge.TargetNode] {
			if b.hasPath(edge.TargetNode, target, visited) {
				return true
			}
		}
	}

	return false
}

// hasCycle checks if the workflow has any cycles.
func (b *WorkflowBuilder) hasCycle() bool {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for nodeID := range b.workflow.Nodes {
		if b.hasCycleDFS(nodeID, visited, recStack) {
			return true
		}
	}

	return false
}

func (b *WorkflowBuilder) hasCycleDFS(nodeID string, visited, recStack map[string]bool) bool {
	if !visited[nodeID] {
		visited[nodeID] = true
		recStack[nodeID] = true

		for _, edge := range b.workflow.Edges {
			if edge.SourceNode == nodeID {
				neighbor := edge.TargetNode
				if !visited[neighbor] && b.hasCycleDFS(neighbor, visited, recStack) {
					return true
				} else if recStack[neighbor] {
					return true
				}
			}
		}
	}

	recStack[nodeID] = false
	return false
}

// GetTopologicalOrder returns nodes in execution order.
func (b *WorkflowBuilder) GetTopologicalOrder() ([]string, error) {
	if b.hasCycle() {
		return nil, ErrCycleDetected
	}

	// Calculate in-degree for each node
	inDegree := make(map[string]int)
	for id := range b.workflow.Nodes {
		inDegree[id] = 0
	}

	for _, edge := range b.workflow.Edges {
		inDegree[edge.TargetNode]++
	}

	// Queue nodes with no incoming edges
	queue := make([]string, 0)
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	// Sort for deterministic order
	sort.Strings(queue)

	result := make([]string, 0, len(b.workflow.Nodes))

	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		result = append(result, nodeID)

		// Reduce in-degree for neighbors
		for _, edge := range b.workflow.Edges {
			if edge.SourceNode == nodeID {
				inDegree[edge.TargetNode]--
				if inDegree[edge.TargetNode] == 0 {
					queue = append(queue, edge.TargetNode)
				}
			}
		}

		sort.Strings(queue)
	}

	return result, nil
}

// Export exports the workflow to JSON.
func (b *WorkflowBuilder) Export() ([]byte, error) {
	return json.MarshalIndent(b.workflow, "", "  ")
}

// Import imports a workflow from JSON.
func Import(data []byte) (*WorkflowBuilder, error) {
	var workflow Workflow
	if err := json.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow: %w", err)
	}

	return &WorkflowBuilder{workflow: &workflow}, nil
}

// Helper functions

func isValidNodeType(t NodeType) bool {
	validTypes := []NodeType{
		NodeTypeTrigger, NodeTypeJob, NodeTypeCondition, NodeTypeParallel,
		NodeTypeJoin, NodeTypeDelay, NodeTypeApproval, NodeTypeNotification,
		NodeTypeSubWorkflow, NodeTypeScript, NodeTypeHTTP, NodeTypeTransform,
	}
	for _, vt := range validTypes {
		if t == vt {
			return true
		}
	}
	return false
}

func getDefaultStyle(nodeType NodeType) *NodeStyle {
	styles := map[NodeType]*NodeStyle{
		NodeTypeTrigger:     {Color: "#4CAF50", Icon: "play_arrow", Width: 120, Height: 60},
		NodeTypeJob:         {Color: "#2196F3", Icon: "work", Width: 150, Height: 80},
		NodeTypeCondition:   {Color: "#FF9800", Icon: "call_split", Width: 140, Height: 70},
		NodeTypeParallel:    {Color: "#9C27B0", Icon: "call_split", Width: 130, Height: 70},
		NodeTypeJoin:        {Color: "#9C27B0", Icon: "call_merge", Width: 130, Height: 70},
		NodeTypeDelay:       {Color: "#607D8B", Icon: "schedule", Width: 120, Height: 60},
		NodeTypeApproval:    {Color: "#E91E63", Icon: "thumb_up", Width: 140, Height: 80},
		NodeTypeNotification: {Color: "#00BCD4", Icon: "notifications", Width: 140, Height: 70},
		NodeTypeScript:      {Color: "#795548", Icon: "code", Width: 150, Height: 80},
		NodeTypeHTTP:        {Color: "#3F51B5", Icon: "http", Width: 140, Height: 70},
		NodeTypeTransform:   {Color: "#FF5722", Icon: "transform", Width: 140, Height: 70},
	}

	if style, ok := styles[nodeType]; ok {
		return style
	}
	return &NodeStyle{Color: "#9E9E9E", Width: 120, Height: 60}
}

func getDefaultInputs(nodeType NodeType) []*NodePort {
	if nodeType == NodeTypeTrigger {
		return nil // Triggers have no inputs
	}

	inputs := []*NodePort{
		{ID: "input", Name: "Input", Type: "default", Required: true},
	}

	if nodeType == NodeTypeJoin {
		inputs = append(inputs,
			&NodePort{ID: "input_2", Name: "Input 2", Type: "default"},
			&NodePort{ID: "input_3", Name: "Input 3", Type: "default"},
		)
	}

	return inputs
}

func getDefaultOutputs(nodeType NodeType) []*NodePort {
	outputs := []*NodePort{
		{ID: "output", Name: "Output", Type: "default"},
	}

	switch nodeType {
	case NodeTypeCondition:
		outputs = []*NodePort{
			{ID: "true", Name: "True", Type: "success"},
			{ID: "false", Name: "False", Type: "failure"},
		}
	case NodeTypeJob, NodeTypeHTTP, NodeTypeScript:
		outputs = []*NodePort{
			{ID: "success", Name: "Success", Type: "success"},
			{ID: "failure", Name: "Failure", Type: "failure"},
		}
	case NodeTypeApproval:
		outputs = []*NodePort{
			{ID: "approved", Name: "Approved", Type: "success"},
			{ID: "rejected", Name: "Rejected", Type: "failure"},
			{ID: "timeout", Name: "Timeout", Type: "any"},
		}
	case NodeTypeParallel:
		outputs = []*NodePort{
			{ID: "branch_1", Name: "Branch 1", Type: "default"},
			{ID: "branch_2", Name: "Branch 2", Type: "default"},
			{ID: "branch_3", Name: "Branch 3", Type: "default"},
		}
	}

	return outputs
}
