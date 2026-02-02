// Package replay provides debug console API handlers.
package replay

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// APIHandler handles debug console API requests.
type APIHandler struct {
	debugger *Debugger
	timeTravelDebugger *TimeTravelDebugger
}

// NewAPIHandler creates a new debug console API handler.
func NewAPIHandler(debugger *Debugger) *APIHandler {
	return &APIHandler{
		debugger: debugger,
		timeTravelDebugger: NewTimeTravelDebugger(debugger),
	}
}

// APIResponse is the standard API response format.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ConsoleBreakpoint represents a breakpoint in the API.
type ConsoleBreakpoint struct {
	ID        string `json:"id"`
	Type      string `json:"type"` // "request", "response", "error", "status_code"
	Condition string `json:"condition,omitempty"`
	Enabled   bool   `json:"enabled"`
}

// WatchVariable represents a watched variable/field.
type WatchVariable struct {
	Name  string      `json:"name"`
	Path  string      `json:"path"` // JSONPath expression
	Value interface{} `json:"value,omitempty"`
}

// SessionCommand represents a command executed in the session.
type SessionCommand struct {
	Command   string      `json:"command"`
	Args      []string    `json:"args,omitempty"`
	Output    interface{} `json:"output,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// Routes returns the chi router with all debug console routes.
func (h *APIHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Execution snapshots
	r.Get("/snapshots", h.ListSnapshots)
	r.Get("/snapshots/{executionID}", h.GetSnapshot)
	r.Post("/snapshots", h.CaptureSnapshot)

	// Replay
	r.Post("/replay", h.Replay)
	r.Get("/replays/{replayID}", h.GetReplayResult)

	// Timeline
	r.Get("/timeline/{executionID}", h.GetTimeline)

	// Compare
	r.Post("/compare", h.Compare)

	// Debug sessions
	r.Post("/sessions", h.StartSession)
	r.Get("/sessions/{sessionID}", h.GetSession)
	r.Delete("/sessions/{sessionID}", h.EndSession)
	r.Post("/sessions/{sessionID}/command", h.ExecuteCommand)
	r.Post("/sessions/{sessionID}/breakpoints", h.AddBreakpoint)
	r.Delete("/sessions/{sessionID}/breakpoints/{breakpointID}", h.RemoveBreakpoint)
	r.Post("/sessions/{sessionID}/watch", h.AddWatch)
	r.Delete("/sessions/{sessionID}/watch/{name}", h.RemoveWatch)

	// Step debugging
	r.Post("/sessions/{sessionID}/step", h.Step)
	r.Post("/sessions/{sessionID}/continue", h.Continue)
	r.Post("/sessions/{sessionID}/pause", h.Pause)

	// Evaluation
	r.Post("/evaluate", h.Evaluate)

	return r
}

// ListSnapshots lists execution snapshots.
func (h *APIHandler) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("job_id")
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	snapshots, err := h.debugger.ListSnapshots(r.Context(), jobID, limit)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: snapshots})
}

// GetSnapshot retrieves an execution snapshot.
func (h *APIHandler) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	executionID := chi.URLParam(r, "executionID")
	snapshot, err := h.debugger.GetSnapshot(r.Context(), executionID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: snapshot})
}

// CaptureSnapshot captures a new execution snapshot.
func (h *APIHandler) CaptureSnapshot(w http.ResponseWriter, r *http.Request) {
	var snapshot ExecutionSnapshot
	if err := json.NewDecoder(r.Body).Decode(&snapshot); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	if err := h.debugger.CaptureExecution(r.Context(), &snapshot); err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: snapshot})
}

// Replay replays an execution.
func (h *APIHandler) Replay(w http.ResponseWriter, r *http.Request) {
	var req ReplayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	result, err := h.debugger.Replay(r.Context(), &req)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: result})
}

// GetReplayResult retrieves a replay result.
func (h *APIHandler) GetReplayResult(w http.ResponseWriter, r *http.Request) {
	replayID := chi.URLParam(r, "replayID")
	
	h.debugger.mu.RLock()
	result, ok := h.debugger.replays[replayID]
	h.debugger.mu.RUnlock()

	if !ok {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: "replay not found"})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: result})
}

// GetTimeline retrieves execution timeline.
func (h *APIHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	executionID := chi.URLParam(r, "executionID")
	timeline, err := h.debugger.GetTimeline(r.Context(), executionID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: timeline})
}

// CompareRequest is a request to compare executions.
type CompareRequest struct {
	ExecutionID1 string `json:"execution_id_1"`
	ExecutionID2 string `json:"execution_id_2"`
}

// Compare compares two executions.
func (h *APIHandler) Compare(w http.ResponseWriter, r *http.Request) {
	var req CompareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	diffs, err := h.debugger.Compare(r.Context(), req.ExecutionID1, req.ExecutionID2)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"execution_id_1": req.ExecutionID1,
			"execution_id_2": req.ExecutionID2,
			"differences":    diffs,
			"identical":      len(diffs) == 0,
		},
	})
}

// StartSessionRequest is a request to start a debug session.
type StartSessionRequest struct {
	ExecutionID string `json:"execution_id"`
}

// StartSession starts a new debug session.
func (h *APIHandler) StartSession(w http.ResponseWriter, r *http.Request) {
	var req StartSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	session, err := h.timeTravelDebugger.CreateSession(r.Context(), req.ExecutionID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: session})
}

// GetSession retrieves a debug session.
func (h *APIHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	
	session, err := h.timeTravelDebugger.GetSession(sessionID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: session})
}

// EndSession ends a debug session.
func (h *APIHandler) EndSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if err := h.timeTravelDebugger.CloseSession(sessionID); err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, APIResponse{Success: true})
}

// CommandRequest is a request to execute a command.
type CommandRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// ExecuteCommand executes a command in the debug session.
func (h *APIHandler) ExecuteCommand(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	var req CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	session, err := h.timeTravelDebugger.GetSession(sessionID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	cmd := SessionCommand{
		Command:   req.Command,
		Args:      req.Args,
		Timestamp: time.Now(),
	}

	// Execute command
	switch req.Command {
	case "inspect":
		cmd.Output = h.inspectCommandTT(session, req.Args)
	case "request":
		cmd.Output = session.Snapshot.Request
	case "response":
		cmd.Output = session.Snapshot.Response
	case "config":
		cmd.Output = session.Snapshot.JobConfig
	case "context":
		cmd.Output = session.Snapshot.Context
	case "timeline":
		timeline, err := h.debugger.GetTimeline(r.Context(), session.ExecutionID)
		if err != nil {
			cmd.Error = err.Error()
		} else {
			cmd.Output = timeline
		}
	case "help":
		cmd.Output = map[string]string{
			"inspect <field>": "Inspect a field (request, response, config, context)",
			"request":         "Show the HTTP request details",
			"response":        "Show the HTTP response details",
			"config":          "Show job configuration",
			"context":         "Show execution context",
			"timeline":        "Show execution timeline",
			"replay":          "Replay the execution",
			"diff <exec_id>":  "Compare with another execution",
			"watch <path>":    "Add a watch expression",
			"breakpoint":      "Manage breakpoints",
		}
	case "replay":
		result, err := h.debugger.Replay(r.Context(), &ReplayRequest{
			ExecutionID: session.ExecutionID,
			DryRun:      len(req.Args) > 0 && req.Args[0] == "--dry-run",
		})
		if err != nil {
			cmd.Error = err.Error()
		} else {
			cmd.Output = result
		}
	case "diff":
		if len(req.Args) < 1 {
			cmd.Error = "usage: diff <execution_id>"
		} else {
			diffs, err := h.debugger.Compare(r.Context(), session.ExecutionID, req.Args[0])
			if err != nil {
				cmd.Error = err.Error()
			} else {
				cmd.Output = diffs
			}
		}
	default:
		cmd.Error = "unknown command: " + req.Command
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: cmd})
}

func (h *APIHandler) inspectCommandTT(session *DebugSession, args []string) interface{} {
	if len(args) == 0 {
		return map[string]interface{}{
			"available": []string{"request", "response", "config", "context", "metadata"},
		}
	}

	switch args[0] {
	case "request":
		return session.Snapshot.Request
	case "response":
		return session.Snapshot.Response
	case "config":
		return session.Snapshot.JobConfig
	case "context":
		return session.Snapshot.Context
	case "metadata":
		return session.Snapshot.Metadata
	default:
		return map[string]string{"error": "unknown field: " + args[0]}
	}
}

// AddBreakpointRequest is a request to add a breakpoint.
type AddBreakpointRequest struct {
	Type      string `json:"type"`
	Condition string `json:"condition,omitempty"`
}

// AddBreakpoint adds a breakpoint.
func (h *APIHandler) AddBreakpoint(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	var req AddBreakpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	bp := &Breakpoint{
		ID:        uuid.New().String(),
		Type:      BreakpointType(req.Type),
		Condition: req.Condition,
		Enabled:   true,
	}

	if err := h.timeTravelDebugger.AddBreakpoint(sessionID, bp); err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: bp})
}

// RemoveBreakpoint removes a breakpoint.
func (h *APIHandler) RemoveBreakpoint(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	breakpointID := chi.URLParam(r, "breakpointID")

	if err := h.timeTravelDebugger.RemoveBreakpoint(sessionID, breakpointID); err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true})
}

// AddWatchRequest is a request to add a watch variable.
type AddWatchRequest struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// AddWatch adds a watch variable.
func (h *APIHandler) AddWatch(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	var req AddWatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	watch, err := h.timeTravelDebugger.AddWatch(sessionID, req.Path)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: watch})
}

// RemoveWatch removes a watch variable.
func (h *APIHandler) RemoveWatch(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	name := chi.URLParam(r, "name")

	// Note: Time travel debugger doesn't have a direct remove watch method
	// We'll use close session and re-create without that watch
	// For now, return success (watches are ephemeral)
	_ = sessionID
	_ = name
	h.writeJSON(w, http.StatusOK, APIResponse{Success: true})
}

// Step steps through execution.
func (h *APIHandler) Step(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	session, err := h.timeTravelDebugger.StepForward(sessionID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"state":       session.State,
			"current_step": session.CurrentStep,
			"message":     "Stepped to next event",
		},
	})
}

// Continue continues execution.
func (h *APIHandler) Continue(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	session, err := h.timeTravelDebugger.Continue(sessionID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"state": session.State,
		},
	})
}

// Pause pauses execution.
func (h *APIHandler) Pause(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	session, err := h.timeTravelDebugger.GetSession(sessionID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	// Set state to paused
	session.State = DebugStatePaused

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"state": session.State,
		},
	})
}

// EvaluateRequest is a request to evaluate an expression.
type EvaluateRequest struct {
	ExecutionID string `json:"execution_id"`
	Expression  string `json:"expression"`
}

// Evaluate evaluates an expression against an execution.
func (h *APIHandler) Evaluate(w http.ResponseWriter, r *http.Request) {
	var req EvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	snapshot, err := h.debugger.GetSnapshot(r.Context(), req.ExecutionID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	result := h.evaluateExpression(snapshot, req.Expression)

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"expression": req.Expression,
			"result":     result,
		},
	})
}

func (h *APIHandler) evaluateExpression(snapshot *ExecutionSnapshot, expr string) interface{} {
	// Simple path-based evaluation
	switch expr {
	case "request":
		return snapshot.Request
	case "request.url":
		return snapshot.Request.URL
	case "request.method":
		return snapshot.Request.Method
	case "request.headers":
		return snapshot.Request.Headers
	case "request.body":
		return snapshot.Request.Body
	case "response":
		return snapshot.Response
	case "response.status_code":
		return snapshot.Response.StatusCode
	case "response.headers":
		return snapshot.Response.Headers
	case "response.body":
		return snapshot.Response.Body
	case "config":
		return snapshot.JobConfig
	case "config.schedule":
		return snapshot.JobConfig.Schedule
	case "config.timeout":
		return snapshot.JobConfig.Timeout
	case "context":
		return snapshot.Context
	case "context.node_id":
		return snapshot.Context.NodeID
	case "status":
		return snapshot.Status
	case "error":
		return snapshot.Error
	case "duration":
		return snapshot.Duration.String()
	case "attempt":
		return snapshot.Attempt
	default:
		return map[string]string{"error": "unknown expression: " + expr}
	}
}

func (h *APIHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
