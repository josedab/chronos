// Package realtime provides WebSocket-based real-time collaboration features.
package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors.
var (
	ErrClientNotFound   = errors.New("client not found")
	ErrRoomNotFound     = errors.New("room not found")
	ErrRoomFull         = errors.New("room is full")
	ErrInvalidMessage   = errors.New("invalid message")
	ErrConnectionClosed = errors.New("connection closed")
)

// MessageType represents the type of WebSocket message.
type MessageType string

const (
	// Client -> Server
	MessageTypeJoinRoom     MessageType = "join_room"
	MessageTypeLeaveRoom    MessageType = "leave_room"
	MessageTypePresence     MessageType = "presence"
	MessageTypeCursorMove   MessageType = "cursor_move"
	MessageTypeSelection    MessageType = "selection"
	MessageTypeEdit         MessageType = "edit"
	MessageTypeComment      MessageType = "comment"
	MessageTypeSubscribe    MessageType = "subscribe"
	MessageTypeUnsubscribe  MessageType = "unsubscribe"
	MessageTypePing         MessageType = "ping"

	// Server -> Client
	MessageTypeWelcome      MessageType = "welcome"
	MessageTypeRoomJoined   MessageType = "room_joined"
	MessageTypeRoomLeft     MessageType = "room_left"
	MessageTypeUserJoined   MessageType = "user_joined"
	MessageTypeUserLeft     MessageType = "user_left"
	MessageTypePresenceList MessageType = "presence_list"
	MessageTypeCursorUpdate MessageType = "cursor_update"
	MessageTypeSelectionUpdate MessageType = "selection_update"
	MessageTypeEditBroadcast MessageType = "edit_broadcast"
	MessageTypeCommentAdded MessageType = "comment_added"
	MessageTypeJobUpdated   MessageType = "job_updated"
	MessageTypeExecStarted  MessageType = "execution_started"
	MessageTypeExecCompleted MessageType = "execution_completed"
	MessageTypePong         MessageType = "pong"
	MessageTypeError        MessageType = "error"
)

// Message represents a WebSocket message.
type Message struct {
	Type      MessageType     `json:"type"`
	ID        string          `json:"id,omitempty"`
	Room      string          `json:"room,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Sender    *UserInfo       `json:"sender,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// UserInfo represents a connected user.
type UserInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email,omitempty"`
	Avatar    string `json:"avatar,omitempty"`
	Color     string `json:"color"` // For cursor/selection highlighting
}

// Presence represents user presence state.
type Presence struct {
	UserID    string    `json:"user_id"`
	User      *UserInfo `json:"user"`
	Status    string    `json:"status"` // "active", "idle", "away"
	LastSeen  time.Time `json:"last_seen"`
	Location  string    `json:"location,omitempty"` // Current view/page
	Cursor    *Cursor   `json:"cursor,omitempty"`
	Selection *Selection `json:"selection,omitempty"`
}

// Cursor represents a user's cursor position.
type Cursor struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Target string `json:"target,omitempty"` // Element ID or path
}

// Selection represents a user's current selection.
type Selection struct {
	Type   string `json:"type"` // "job", "field", "text"
	Target string `json:"target"`
	Start  int    `json:"start,omitempty"`
	End    int    `json:"end,omitempty"`
}

// EditOperation represents a collaborative edit.
type EditOperation struct {
	ID        string      `json:"id"`
	JobID     string      `json:"job_id"`
	Field     string      `json:"field"`
	Operation string      `json:"operation"` // "set", "append", "delete"
	Value     interface{} `json:"value"`
	Version   int64       `json:"version"`
}

// Comment represents a comment on a job or execution.
type Comment struct {
	ID         string    `json:"id"`
	JobID      string    `json:"job_id"`
	ExecutionID string   `json:"execution_id,omitempty"`
	Author     *UserInfo `json:"author"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
	ReplyTo    string    `json:"reply_to,omitempty"`
	Resolved   bool      `json:"resolved"`
}

// Room represents a collaboration room (e.g., job page, dashboard).
type Room struct {
	ID        string               `json:"id"`
	Type      RoomType             `json:"type"`
	Name      string               `json:"name"`
	Clients   map[string]*Client   `json:"-"`
	Presence  map[string]*Presence `json:"presence"`
	CreatedAt time.Time            `json:"created_at"`
	mu        sync.RWMutex
}

// RoomType represents the type of room.
type RoomType string

const (
	RoomTypeDashboard RoomType = "dashboard"
	RoomTypeJob       RoomType = "job"
	RoomTypeExecution RoomType = "execution"
	RoomTypeCluster   RoomType = "cluster"
)

// Client represents a connected WebSocket client.
type Client struct {
	ID        string
	User      *UserInfo
	Conn      WebSocketConn
	Rooms     map[string]bool
	Send      chan []byte
	CreatedAt time.Time
	LastPing  time.Time
	mu        sync.RWMutex
}

// WebSocketConn is an interface for WebSocket connections.
type WebSocketConn interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
	Close() error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

// Hub manages WebSocket connections and rooms.
type Hub struct {
	clients    map[string]*Client
	rooms      map[string]*Room
	register   chan *Client
	unregister chan *Client
	broadcast  chan *BroadcastMessage
	mu         sync.RWMutex
	config     HubConfig
}

// BroadcastMessage represents a message to broadcast.
type BroadcastMessage struct {
	Room    string
	Message *Message
	Exclude []string // Client IDs to exclude
}

// HubConfig configures the hub.
type HubConfig struct {
	MaxClientsPerRoom int
	MaxRooms          int
	PingInterval      time.Duration
	WriteWait         time.Duration
	ReadWait          time.Duration
	MaxMessageSize    int64
}

// DefaultHubConfig returns the default hub configuration.
func DefaultHubConfig() HubConfig {
	return HubConfig{
		MaxClientsPerRoom: 50,
		MaxRooms:          1000,
		PingInterval:      30 * time.Second,
		WriteWait:         10 * time.Second,
		ReadWait:          60 * time.Second,
		MaxMessageSize:    65536,
	}
}

// NewHub creates a new Hub.
func NewHub(cfg HubConfig) *Hub {
	if cfg.PingInterval == 0 {
		cfg = DefaultHubConfig()
	}

	return &Hub{
		clients:    make(map[string]*Client),
		rooms:      make(map[string]*Room),
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
		broadcast:  make(chan *BroadcastMessage, 256),
		config:     cfg,
	}
}

// Run starts the hub's main event loop.
func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(h.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.shutdown()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()

			// Send welcome message
			h.sendToClient(client, &Message{
				Type:      MessageTypeWelcome,
				Timestamp: time.Now(),
				Payload:   mustJSON(map[string]interface{}{"client_id": client.ID}),
			})

		case client := <-h.unregister:
			h.removeClient(client)

		case msg := <-h.broadcast:
			h.broadcastToRoom(msg)

		case <-ticker.C:
			h.pingClients()
		}
	}
}

// RegisterClient registers a new client.
func (h *Hub) RegisterClient(conn WebSocketConn, user *UserInfo) *Client {
	clientID := uuid.New().String()
	client := &Client{
		ID:        clientID,
		User:      user,
		Conn:      conn,
		Rooms:     make(map[string]bool),
		Send:      make(chan []byte, 256),
		CreatedAt: time.Now(),
		LastPing:  time.Now(),
	}

	// Assign a unique color to the user
	if user.Color == "" {
		user.Color = generateUserColor(clientID)
	}

	h.register <- client
	return client
}

// UnregisterClient unregisters a client.
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// removeClient removes a client and cleans up.
func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client.ID]; !ok {
		return
	}

	// Leave all rooms
	for roomID := range client.Rooms {
		h.leaveRoomLocked(client, roomID)
	}

	delete(h.clients, client.ID)
	close(client.Send)
	client.Conn.Close()
}

// JoinRoom adds a client to a room.
func (h *Hub) JoinRoom(client *Client, roomID string, roomType RoomType, roomName string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, exists := h.rooms[roomID]
	if !exists {
		// Create the room
		if len(h.rooms) >= h.config.MaxRooms {
			return errors.New("max rooms reached")
		}
		room = &Room{
			ID:        roomID,
			Type:      roomType,
			Name:      roomName,
			Clients:   make(map[string]*Client),
			Presence:  make(map[string]*Presence),
			CreatedAt: time.Now(),
		}
		h.rooms[roomID] = room
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	if len(room.Clients) >= h.config.MaxClientsPerRoom {
		return ErrRoomFull
	}

	room.Clients[client.ID] = client
	room.Presence[client.ID] = &Presence{
		UserID:   client.ID,
		User:     client.User,
		Status:   "active",
		LastSeen: time.Now(),
	}

	client.mu.Lock()
	client.Rooms[roomID] = true
	client.mu.Unlock()

	// Notify others
	h.broadcastToRoomLocked(room, &Message{
		Type:      MessageTypeUserJoined,
		Room:      roomID,
		Timestamp: time.Now(),
		Sender:    client.User,
	}, []string{client.ID})

	// Send room state to joining client
	presenceList := make([]*Presence, 0, len(room.Presence))
	for _, p := range room.Presence {
		presenceList = append(presenceList, p)
	}

	h.sendToClient(client, &Message{
		Type:      MessageTypeRoomJoined,
		Room:      roomID,
		Timestamp: time.Now(),
		Payload:   mustJSON(map[string]interface{}{"presence": presenceList}),
	})

	return nil
}

// LeaveRoom removes a client from a room.
func (h *Hub) LeaveRoom(client *Client, roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.leaveRoomLocked(client, roomID)
}

func (h *Hub) leaveRoomLocked(client *Client, roomID string) {
	room, exists := h.rooms[roomID]
	if !exists {
		return
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	delete(room.Clients, client.ID)
	delete(room.Presence, client.ID)

	client.mu.Lock()
	delete(client.Rooms, roomID)
	client.mu.Unlock()

	// Notify others
	h.broadcastToRoomLocked(room, &Message{
		Type:      MessageTypeUserLeft,
		Room:      roomID,
		Timestamp: time.Now(),
		Sender:    client.User,
	}, nil)

	// Clean up empty rooms
	if len(room.Clients) == 0 {
		delete(h.rooms, roomID)
	}
}

// UpdatePresence updates a client's presence in a room.
func (h *Hub) UpdatePresence(client *Client, roomID string, status string, location string, cursor *Cursor, selection *Selection) {
	h.mu.RLock()
	room, exists := h.rooms[roomID]
	h.mu.RUnlock()

	if !exists {
		return
	}

	room.mu.Lock()
	presence, ok := room.Presence[client.ID]
	if !ok {
		room.mu.Unlock()
		return
	}

	presence.Status = status
	presence.LastSeen = time.Now()
	presence.Location = location
	if cursor != nil {
		presence.Cursor = cursor
	}
	if selection != nil {
		presence.Selection = selection
	}
	room.mu.Unlock()

	// Broadcast presence update
	h.BroadcastToRoom(roomID, &Message{
		Type:      MessageTypePresence,
		Room:      roomID,
		Timestamp: time.Now(),
		Sender:    client.User,
		Payload:   mustJSON(presence),
	}, []string{client.ID})
}

// BroadcastToRoom sends a message to all clients in a room.
func (h *Hub) BroadcastToRoom(roomID string, msg *Message, exclude []string) {
	h.broadcast <- &BroadcastMessage{
		Room:    roomID,
		Message: msg,
		Exclude: exclude,
	}
}

func (h *Hub) broadcastToRoom(bm *BroadcastMessage) {
	h.mu.RLock()
	room, exists := h.rooms[bm.Room]
	h.mu.RUnlock()

	if !exists {
		return
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	h.broadcastToRoomLocked(room, bm.Message, bm.Exclude)
}

func (h *Hub) broadcastToRoomLocked(room *Room, msg *Message, exclude []string) {
	excludeMap := make(map[string]bool)
	for _, id := range exclude {
		excludeMap[id] = true
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for clientID, client := range room.Clients {
		if excludeMap[clientID] {
			continue
		}

		select {
		case client.Send <- data:
		default:
			// Client buffer full, skip
		}
	}
}

func (h *Hub) sendToClient(client *Client, msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	select {
	case client.Send <- data:
	default:
		// Client buffer full
	}
}

// BroadcastJobUpdate broadcasts a job update to relevant rooms.
func (h *Hub) BroadcastJobUpdate(jobID string, update interface{}) {
	msg := &Message{
		Type:      MessageTypeJobUpdated,
		Room:      "job:" + jobID,
		Timestamp: time.Now(),
		Payload:   mustJSON(update),
	}

	// Broadcast to job room
	h.BroadcastToRoom("job:"+jobID, msg, nil)

	// Also broadcast to dashboard
	h.BroadcastToRoom("dashboard", msg, nil)
}

// BroadcastExecutionEvent broadcasts execution events.
func (h *Hub) BroadcastExecutionEvent(jobID string, executionID string, eventType MessageType, data interface{}) {
	msg := &Message{
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   mustJSON(map[string]interface{}{
			"job_id":       jobID,
			"execution_id": executionID,
			"data":         data,
		}),
	}

	// Broadcast to job room and execution room
	h.BroadcastToRoom("job:"+jobID, msg, nil)
	h.BroadcastToRoom("execution:"+executionID, msg, nil)
	h.BroadcastToRoom("dashboard", msg, nil)
}

func (h *Hub) pingClients() {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for _, c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	now := time.Now()
	for _, client := range clients {
		// Check if client has timed out
		if now.Sub(client.LastPing) > h.config.ReadWait {
			h.unregister <- client
			continue
		}

		// Send ping
		h.sendToClient(client, &Message{
			Type:      MessageTypePong,
			Timestamp: now,
		})
	}
}

func (h *Hub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, client := range h.clients {
		close(client.Send)
		client.Conn.Close()
	}
}

// GetRoomStats returns statistics for a room.
func (h *Hub) GetRoomStats(roomID string) (int, []*Presence) {
	h.mu.RLock()
	room, exists := h.rooms[roomID]
	h.mu.RUnlock()

	if !exists {
		return 0, nil
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	presence := make([]*Presence, 0, len(room.Presence))
	for _, p := range room.Presence {
		presence = append(presence, p)
	}

	return len(room.Clients), presence
}

// GetStats returns overall hub statistics.
func (h *Hub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return map[string]interface{}{
		"clients":     len(h.clients),
		"rooms":       len(h.rooms),
		"max_clients": h.config.MaxClientsPerRoom * h.config.MaxRooms,
		"max_rooms":   h.config.MaxRooms,
	}
}

// HandleMessage processes an incoming message from a client.
func (h *Hub) HandleMessage(client *Client, data []byte) error {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return ErrInvalidMessage
	}

	msg.Sender = client.User
	msg.Timestamp = time.Now()

	switch msg.Type {
	case MessageTypePing:
		client.LastPing = time.Now()
		h.sendToClient(client, &Message{Type: MessageTypePong, Timestamp: time.Now()})

	case MessageTypeJoinRoom:
		var payload struct {
			RoomID   string   `json:"room_id"`
			RoomType RoomType `json:"room_type"`
			RoomName string   `json:"room_name"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return ErrInvalidMessage
		}
		return h.JoinRoom(client, payload.RoomID, payload.RoomType, payload.RoomName)

	case MessageTypeLeaveRoom:
		h.LeaveRoom(client, msg.Room)

	case MessageTypePresence:
		var payload struct {
			Status    string     `json:"status"`
			Location  string     `json:"location"`
			Cursor    *Cursor    `json:"cursor"`
			Selection *Selection `json:"selection"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return ErrInvalidMessage
		}
		h.UpdatePresence(client, msg.Room, payload.Status, payload.Location, payload.Cursor, payload.Selection)

	case MessageTypeCursorMove:
		h.BroadcastToRoom(msg.Room, &Message{
			Type:      MessageTypeCursorUpdate,
			Room:      msg.Room,
			Timestamp: time.Now(),
			Sender:    client.User,
			Payload:   msg.Payload,
		}, []string{client.ID})

	case MessageTypeSelection:
		h.BroadcastToRoom(msg.Room, &Message{
			Type:      MessageTypeSelectionUpdate,
			Room:      msg.Room,
			Timestamp: time.Now(),
			Sender:    client.User,
			Payload:   msg.Payload,
		}, []string{client.ID})

	case MessageTypeEdit:
		h.BroadcastToRoom(msg.Room, &Message{
			Type:      MessageTypeEditBroadcast,
			Room:      msg.Room,
			Timestamp: time.Now(),
			Sender:    client.User,
			Payload:   msg.Payload,
		}, []string{client.ID})

	case MessageTypeComment:
		h.BroadcastToRoom(msg.Room, &Message{
			Type:      MessageTypeCommentAdded,
			Room:      msg.Room,
			Timestamp: time.Now(),
			Sender:    client.User,
			Payload:   msg.Payload,
		}, nil) // Include sender for comments
	}

	return nil
}

// ServeWS handles WebSocket connections (for use with http.Handler).
func (h *Hub) ServeWS(upgrader interface{}, w http.ResponseWriter, r *http.Request, user *UserInfo) {
	// This would be implemented with gorilla/websocket or similar
	// For now, this is a placeholder showing the interface
}

func mustJSON(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func generateUserColor(seed string) string {
	colors := []string{
		"#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4", "#FFEAA7",
		"#DDA0DD", "#98D8C8", "#F7DC6F", "#BB8FCE", "#85C1E9",
	}
	hash := 0
	for _, c := range seed {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return colors[hash%len(colors)]
}
