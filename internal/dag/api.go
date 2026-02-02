// Package dag provides DAG API handlers for the Chronos REST API.
package dag

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// APIHandler handles DAG/workflow API requests.
type APIHandler struct {
	store  Store
	engine *Engine
	logger zerolog.Logger
}

// NewAPIHandler creates a new DAG API handler.
func NewAPIHandler(store Store, engine *Engine, logger zerolog.Logger) *APIHandler {
	return &APIHandler{
		store:  store,
		engine: engine,
		logger: logger.With().Str("component", "dag-api").Logger(),
	}
}

// Response types

// APIResponse is a generic API response.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError contains error details.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WorkflowListResponse is the response for listing workflows.
type WorkflowListResponse struct {
	Workflows []*Workflow `json:"workflows"`
	Total     int         `json:"total"`
	Offset    int         `json:"offset"`
	Limit     int         `json:"limit"`
}

// RunListResponse is the response for listing runs.
type RunListResponse struct {
	Runs   []*Run `json:"runs"`
	Total  int    `json:"total"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

// CreateWorkflowRequest is the request body for creating a workflow.
type CreateWorkflowRequest struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description,omitempty"`
	Settings    *WorkflowSettings            `json:"settings,omitempty"`
	Variables   map[string]*Variable         `json:"variables,omitempty"`
	Nodes       map[string]*WorkflowNodeSpec `json:"nodes,omitempty"`
	Edges       []*EdgeSpec                  `json:"edges,omitempty"`
}

// WorkflowNodeSpec is the spec for creating a node.
type WorkflowNodeSpec struct {
	Type        NodeType               `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config"`
	Position    Position               `json:"position"`
}

// EdgeSpec is the spec for creating an edge.
type EdgeSpec struct {
	SourceNode string `json:"source_node"`
	SourcePort string `json:"source_port"`
	TargetNode string `json:"target_node"`
	TargetPort string `json:"target_port"`
	Condition  string `json:"condition,omitempty"`
}

// TriggerRunRequest is the request body for triggering a workflow run.
type TriggerRunRequest struct {
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// Routes registers the DAG API routes.
func (h *APIHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Workflow CRUD
	r.Get("/", h.ListWorkflows)
	r.Post("/", h.CreateWorkflow)
	r.Get("/{workflowID}", h.GetWorkflow)
	r.Put("/{workflowID}", h.UpdateWorkflow)
	r.Delete("/{workflowID}", h.DeleteWorkflow)

	// Workflow operations
	r.Post("/{workflowID}/trigger", h.TriggerWorkflow)
	r.Post("/{workflowID}/validate", h.ValidateWorkflow)
	r.Get("/{workflowID}/export", h.ExportWorkflow)
	r.Post("/import", h.ImportWorkflow)

	// Node operations
	r.Post("/{workflowID}/nodes", h.AddNode)
	r.Put("/{workflowID}/nodes/{nodeID}", h.UpdateNode)
	r.Delete("/{workflowID}/nodes/{nodeID}", h.DeleteNode)

	// Edge operations
	r.Post("/{workflowID}/edges", h.AddEdge)
	r.Delete("/{workflowID}/edges/{edgeID}", h.DeleteEdge)

	// Run operations
	r.Get("/{workflowID}/runs", h.ListRuns)
	r.Get("/{workflowID}/runs/{runID}", h.GetRun)
	r.Post("/{workflowID}/runs/{runID}/cancel", h.CancelRun)
	r.Get("/{workflowID}/runs/{runID}/visualization", h.GetRunVisualization)

	// DAG operations (legacy compatibility)
	r.Get("/dags", h.ListDAGs)
	r.Get("/dags/{dagID}", h.GetDAG)

	return r
}

// ListWorkflows handles GET /workflows.
func (h *APIHandler) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	opts := h.parseListOptions(r)

	workflows, total, err := h.store.ListWorkflows(r.Context(), opts)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: WorkflowListResponse{
			Workflows: workflows,
			Total:     total,
			Offset:    opts.Offset,
			Limit:     opts.Limit,
		},
	})
}

// CreateWorkflow handles POST /workflows.
func (h *APIHandler) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var req CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "workflow name is required")
		return
	}

	builder := NewWorkflowBuilder(req.Name)

	if req.Description != "" {
		builder.workflow.Description = req.Description
	}
	if req.Settings != nil {
		builder.SetSettings(req.Settings)
	}
	for name, v := range req.Variables {
		_ = builder.AddVariable(name, v.Type, v.Value, v.Required)
	}

	// Add nodes
	nodeIDMap := make(map[string]string) // original ID -> generated ID
	for origID, spec := range req.Nodes {
		node, err := builder.AddNode(spec.Type, spec.Name, spec.Position, spec.Config)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "NODE_ERROR", err.Error())
			return
		}
		node.Description = spec.Description
		nodeIDMap[origID] = node.ID
	}

	// Add edges (map original IDs to generated IDs)
	for _, edge := range req.Edges {
		sourceID := nodeIDMap[edge.SourceNode]
		targetID := nodeIDMap[edge.TargetNode]
		if sourceID == "" || targetID == "" {
			h.writeError(w, http.StatusBadRequest, "EDGE_ERROR", "invalid node reference in edge")
			return
		}

		_, err := builder.AddEdge(sourceID, edge.SourcePort, targetID, edge.TargetPort)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "EDGE_ERROR", err.Error())
			return
		}
	}

	workflow, err := builder.Build()
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	if err := h.store.CreateWorkflow(r.Context(), workflow); err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.logger.Info().Str("workflow_id", workflow.ID).Str("name", workflow.Name).Msg("Workflow created")

	h.writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    workflow,
	})
}

// GetWorkflow handles GET /workflows/{workflowID}.
func (h *APIHandler) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err == ErrWorkflowNotFound {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    workflow,
	})
}

// UpdateWorkflow handles PUT /workflows/{workflowID}.
func (h *APIHandler) UpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	existing, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err == ErrWorkflowNotFound {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	var req CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	// Update fields
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if req.Settings != nil {
		existing.Settings = req.Settings
	}

	existing.Version++
	existing.UpdatedAt = time.Now().UTC()

	if err := h.store.UpdateWorkflow(r.Context(), existing); err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    existing,
	})
}

// DeleteWorkflow handles DELETE /workflows/{workflowID}.
func (h *APIHandler) DeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	if err := h.store.DeleteWorkflow(r.Context(), workflowID); err == ErrWorkflowNotFound {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	} else if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.logger.Info().Str("workflow_id", workflowID).Msg("Workflow deleted")

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"deleted": workflowID},
	})
}

// TriggerWorkflow handles POST /workflows/{workflowID}/trigger.
func (h *APIHandler) TriggerWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err == ErrWorkflowNotFound {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	// Convert workflow to DAG for execution
	builder := LoadWorkflow(workflow)
	dag, err := builder.ToDAG()
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "CONVERSION_ERROR", err.Error())
		return
	}

	// Register DAG with engine if not already registered
	if _, err := h.engine.GetDAG(dag.ID); err != nil {
		if err := h.engine.RegisterDAG(dag); err != nil {
			h.writeError(w, http.StatusInternalServerError, "ENGINE_ERROR", err.Error())
			return
		}
	}

	// Start the run
	runID := uuid.New().String()
	run, err := h.engine.StartRun(r.Context(), dag.ID, runID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "EXECUTION_ERROR", err.Error())
		return
	}

	// Persist the run
	if err := h.store.CreateRun(r.Context(), run); err != nil {
		h.logger.Warn().Err(err).Str("run_id", runID).Msg("Failed to persist run")
	}

	h.logger.Info().
		Str("workflow_id", workflowID).
		Str("run_id", runID).
		Msg("Workflow triggered")

	h.writeJSON(w, http.StatusAccepted, APIResponse{
		Success: true,
		Data:    run,
	})
}

// ValidateWorkflow handles POST /workflows/{workflowID}/validate.
func (h *APIHandler) ValidateWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err == ErrWorkflowNotFound {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	builder := LoadWorkflow(workflow)
	validationResult := &ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Validate workflow
	if err := builder.validate(); err != nil {
		validationResult.Valid = false
		validationResult.Errors = append(validationResult.Errors, err.Error())
	}

	// Check for orphaned nodes
	connectedNodes := make(map[string]bool)
	for _, edge := range workflow.Edges {
		connectedNodes[edge.SourceNode] = true
		connectedNodes[edge.TargetNode] = true
	}
	for nodeID, node := range workflow.Nodes {
		if node.Type != NodeTypeTrigger && !connectedNodes[nodeID] {
			validationResult.Warnings = append(validationResult.Warnings,
				"Node '"+node.Name+"' is not connected to any other node")
		}
	}

	// Check for missing configurations
	for _, node := range workflow.Nodes {
		if len(node.Config) == 0 || string(node.Config) == "{}" {
			validationResult.Warnings = append(validationResult.Warnings,
				"Node '"+node.Name+"' has no configuration")
		}
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    validationResult,
	})
}

// ValidationResult contains workflow validation results.
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// ExportWorkflow handles GET /workflows/{workflowID}/export.
func (h *APIHandler) ExportWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err == ErrWorkflowNotFound {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	builder := LoadWorkflow(workflow)
	data, err := builder.Export()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "EXPORT_ERROR", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename="+workflow.Name+".json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// ImportWorkflow handles POST /workflows/import.
func (h *APIHandler) ImportWorkflow(w http.ResponseWriter, r *http.Request) {
	var workflow Workflow
	if err := json.NewDecoder(r.Body).Decode(&workflow); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	// Assign new ID and reset timestamps
	workflow.ID = uuid.New().String()
	workflow.CreatedAt = time.Now().UTC()
	workflow.UpdatedAt = workflow.CreatedAt
	workflow.Version = 1

	// Validate imported workflow
	builder := LoadWorkflow(&workflow)
	if err := builder.validate(); err != nil {
		h.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	if err := h.store.CreateWorkflow(r.Context(), &workflow); err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.logger.Info().Str("workflow_id", workflow.ID).Str("name", workflow.Name).Msg("Workflow imported")

	h.writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    workflow,
	})
}

// AddNode handles POST /workflows/{workflowID}/nodes.
func (h *APIHandler) AddNode(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err == ErrWorkflowNotFound {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	var spec WorkflowNodeSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	builder := LoadWorkflow(workflow)
	node, err := builder.AddNode(spec.Type, spec.Name, spec.Position, spec.Config)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "NODE_ERROR", err.Error())
		return
	}
	node.Description = spec.Description

	workflow.Version++
	if err := h.store.UpdateWorkflow(r.Context(), workflow); err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    node,
	})
}

// UpdateNode handles PUT /workflows/{workflowID}/nodes/{nodeID}.
func (h *APIHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	nodeID := chi.URLParam(r, "nodeID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err == ErrWorkflowNotFound {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	var update NodeUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	builder := LoadWorkflow(workflow)
	if err := builder.UpdateNode(nodeID, &update); err != nil {
		h.writeError(w, http.StatusBadRequest, "NODE_ERROR", err.Error())
		return
	}

	workflow.Version++
	if err := h.store.UpdateWorkflow(r.Context(), workflow); err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    workflow.Nodes[nodeID],
	})
}

// DeleteNode handles DELETE /workflows/{workflowID}/nodes/{nodeID}.
func (h *APIHandler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	nodeID := chi.URLParam(r, "nodeID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err == ErrWorkflowNotFound {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	builder := LoadWorkflow(workflow)
	if err := builder.RemoveNode(nodeID); err != nil {
		h.writeError(w, http.StatusBadRequest, "NODE_ERROR", err.Error())
		return
	}

	workflow.Version++
	if err := h.store.UpdateWorkflow(r.Context(), workflow); err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"deleted": nodeID},
	})
}

// AddEdge handles POST /workflows/{workflowID}/edges.
func (h *APIHandler) AddEdge(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err == ErrWorkflowNotFound {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	var spec EdgeSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	builder := LoadWorkflow(workflow)
	edge, err := builder.AddEdge(spec.SourceNode, spec.SourcePort, spec.TargetNode, spec.TargetPort)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "EDGE_ERROR", err.Error())
		return
	}

	workflow.Version++
	if err := h.store.UpdateWorkflow(r.Context(), workflow); err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    edge,
	})
}

// DeleteEdge handles DELETE /workflows/{workflowID}/edges/{edgeID}.
func (h *APIHandler) DeleteEdge(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	edgeID := chi.URLParam(r, "edgeID")

	workflow, err := h.store.GetWorkflow(r.Context(), workflowID)
	if err == ErrWorkflowNotFound {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "workflow not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	builder := LoadWorkflow(workflow)
	if err := builder.RemoveEdge(edgeID); err != nil {
		h.writeError(w, http.StatusBadRequest, "EDGE_ERROR", err.Error())
		return
	}

	workflow.Version++
	if err := h.store.UpdateWorkflow(r.Context(), workflow); err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"deleted": edgeID},
	})
}

// ListRuns handles GET /workflows/{workflowID}/runs.
func (h *APIHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	opts := h.parseListOptions(r)

	runs, total, err := h.store.ListRuns(r.Context(), workflowID, opts)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: RunListResponse{
			Runs:   runs,
			Total:  total,
			Offset: opts.Offset,
			Limit:  opts.Limit,
		},
	})
}

// GetRun handles GET /workflows/{workflowID}/runs/{runID}.
func (h *APIHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")

	run, err := h.engine.GetRun(runID)
	if err != nil {
		// Try store as fallback
		run, err = h.store.GetRun(r.Context(), runID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "run not found")
			return
		}
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    run,
	})
}

// CancelRun handles POST /workflows/{workflowID}/runs/{runID}/cancel.
func (h *APIHandler) CancelRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")

	if err := h.engine.CancelRun(runID); err != nil {
		h.writeError(w, http.StatusBadRequest, "CANCEL_ERROR", err.Error())
		return
	}

	h.logger.Info().Str("run_id", runID).Msg("Run cancelled")

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"cancelled": runID},
	})
}

// GetRunVisualization handles GET /workflows/{workflowID}/runs/{runID}/visualization.
func (h *APIHandler) GetRunVisualization(w http.ResponseWriter, r *http.Request) {
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

	visualization := &RunVisualization{
		WorkflowID:   workflowID,
		WorkflowName: workflow.Name,
		RunID:        runID,
		Status:       run.Status,
		StartedAt:    run.StartedAt,
		EndedAt:      run.EndedAt,
		Nodes:        make([]*VisualNode, 0, len(workflow.Nodes)),
		Edges:        make([]*VisualEdge, 0, len(workflow.Edges)),
	}

	// Build node visualization with status
	for _, node := range workflow.Nodes {
		vNode := &VisualNode{
			ID:       node.ID,
			Name:     node.Name,
			Type:     node.Type,
			Position: node.Position,
			Status:   "pending",
		}

		if state, ok := run.NodeStates[node.ID]; ok {
			vNode.Status = string(state.Status)
			vNode.StartedAt = state.StartedAt
			vNode.EndedAt = state.EndedAt
			vNode.Duration = state.Duration
			vNode.Error = state.Error
		}

		visualization.Nodes = append(visualization.Nodes, vNode)
	}

	// Build edge visualization
	for _, edge := range workflow.Edges {
		sourceStatus := "pending"
		if state, ok := run.NodeStates[edge.SourceNode]; ok {
			sourceStatus = string(state.Status)
		}

		visualization.Edges = append(visualization.Edges, &VisualEdge{
			ID:         edge.ID,
			SourceNode: edge.SourceNode,
			TargetNode: edge.TargetNode,
			Status:     sourceStatus,
		})
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    visualization,
	})
}

// RunVisualization contains data for visualizing a run.
type RunVisualization struct {
	WorkflowID   string        `json:"workflow_id"`
	WorkflowName string        `json:"workflow_name"`
	RunID        string        `json:"run_id"`
	Status       RunStatus     `json:"status"`
	StartedAt    time.Time     `json:"started_at"`
	EndedAt      time.Time     `json:"ended_at,omitempty"`
	Nodes        []*VisualNode `json:"nodes"`
	Edges        []*VisualEdge `json:"edges"`
}

// VisualNode contains node visualization data.
type VisualNode struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Type      NodeType      `json:"type"`
	Position  Position      `json:"position"`
	Status    string        `json:"status"`
	StartedAt time.Time     `json:"started_at,omitempty"`
	EndedAt   time.Time     `json:"ended_at,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
	Error     string        `json:"error,omitempty"`
}

// VisualEdge contains edge visualization data.
type VisualEdge struct {
	ID         string `json:"id"`
	SourceNode string `json:"source_node"`
	TargetNode string `json:"target_node"`
	Status     string `json:"status"`
}

// ListDAGs handles GET /dags (legacy).
func (h *APIHandler) ListDAGs(w http.ResponseWriter, r *http.Request) {
	opts := h.parseListOptions(r)

	dags, total, err := h.store.ListDAGs(r.Context(), opts)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"dags":   dags,
			"total":  total,
			"offset": opts.Offset,
			"limit":  opts.Limit,
		},
	})
}

// GetDAG handles GET /dags/{dagID} (legacy).
func (h *APIHandler) GetDAG(w http.ResponseWriter, r *http.Request) {
	dagID := chi.URLParam(r, "dagID")

	dag, err := h.store.GetDAG(r.Context(), dagID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "DAG not found")
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    dag,
	})
}

// Helper methods

func (h *APIHandler) parseListOptions(r *http.Request) ListOptions {
	opts := DefaultListOptions()

	if offset := r.URL.Query().Get("offset"); offset != "" {
		if v, err := strconv.Atoi(offset); err == nil && v >= 0 {
			opts.Offset = v
		}
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if v, err := strconv.Atoi(limit); err == nil && v > 0 && v <= 100 {
			opts.Limit = v
		}
	}
	if sortBy := r.URL.Query().Get("sort_by"); sortBy != "" {
		opts.SortBy = sortBy
	}
	if sortOrder := r.URL.Query().Get("sort_order"); sortOrder == "asc" || sortOrder == "desc" {
		opts.SortOrder = sortOrder
	}

	return opts
}

func (h *APIHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *APIHandler) writeError(w http.ResponseWriter, status int, code, message string) {
	h.writeJSON(w, status, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}

// Ensure context is used
var _ context.Context = context.Background()
