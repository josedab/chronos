package realtime

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestDefaultHubConfig(t *testing.T) {
	cfg := DefaultHubConfig()

	if cfg.MaxClientsPerRoom != 50 {
		t.Errorf("MaxClientsPerRoom = %d, want 50", cfg.MaxClientsPerRoom)
	}
	if cfg.MaxRooms != 1000 {
		t.Errorf("MaxRooms = %d, want 1000", cfg.MaxRooms)
	}
	if cfg.PingInterval != 30*time.Second {
		t.Errorf("PingInterval = %v, want 30s", cfg.PingInterval)
	}
}

func TestNewHub(t *testing.T) {
	hub := NewHub(DefaultHubConfig())

	if hub == nil {
		t.Fatal("NewHub returned nil")
	}
	if hub.clients == nil {
		t.Error("clients map should be initialized")
	}
	if hub.rooms == nil {
		t.Error("rooms map should be initialized")
	}
}

func TestHub_RegisterClient(t *testing.T) {
	hub := NewHub(DefaultHubConfig())
	conn := &mockWebSocketConn{}

	user := &UserInfo{
		ID:    "user-1",
		Name:  "Test User",
		Email: "test@example.com",
	}

	client := hub.RegisterClient(conn, user)

	if client == nil {
		t.Fatal("RegisterClient returned nil")
	}
	if client.ID == "" {
		t.Error("client ID should be set")
	}
	if client.User != user {
		t.Error("client user should be set")
	}
	if user.Color == "" {
		t.Error("user color should be generated")
	}
}

func TestHub_Run(t *testing.T) {
	hub := NewHub(DefaultHubConfig())

	ctx, cancel := context.WithCancel(context.Background())

	go hub.Run(ctx)

	// Register a client
	conn := &mockWebSocketConn{}
	user := &UserInfo{ID: "user-1", Name: "Test"}
	client := hub.RegisterClient(conn, user)

	// Wait for registration
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	_, exists := hub.clients[client.ID]
	hub.mu.RUnlock()

	if !exists {
		t.Error("client should be registered")
	}

	// Unregister
	hub.UnregisterClient(client)
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	_, exists = hub.clients[client.ID]
	hub.mu.RUnlock()

	if exists {
		t.Error("client should be unregistered")
	}

	cancel()
}

func TestHub_JoinRoom(t *testing.T) {
	hub := NewHub(DefaultHubConfig())
	conn := &mockWebSocketConn{}
	user := &UserInfo{ID: "user-1", Name: "Test"}
	client := hub.RegisterClient(conn, user)

	// Process registration
	hub.mu.Lock()
	hub.clients[client.ID] = client
	hub.mu.Unlock()

	err := hub.JoinRoom(client, "room-1", RoomTypeJob, "Test Room")
	if err != nil {
		t.Fatalf("JoinRoom failed: %v", err)
	}

	// Check room exists
	hub.mu.RLock()
	room, exists := hub.rooms["room-1"]
	hub.mu.RUnlock()

	if !exists {
		t.Fatal("room should be created")
	}

	room.mu.RLock()
	_, clientInRoom := room.Clients[client.ID]
	_, presenceExists := room.Presence[client.ID]
	room.mu.RUnlock()

	if !clientInRoom {
		t.Error("client should be in room")
	}
	if !presenceExists {
		t.Error("presence should be set")
	}
}

func TestHub_JoinRoom_RoomFull(t *testing.T) {
	cfg := DefaultHubConfig()
	cfg.MaxClientsPerRoom = 1
	hub := NewHub(cfg)

	// Add first client
	conn1 := &mockWebSocketConn{}
	client1 := hub.RegisterClient(conn1, &UserInfo{ID: "user-1"})
	hub.mu.Lock()
	hub.clients[client1.ID] = client1
	hub.mu.Unlock()

	hub.JoinRoom(client1, "room-1", RoomTypeJob, "Test")

	// Try to add second client
	conn2 := &mockWebSocketConn{}
	client2 := hub.RegisterClient(conn2, &UserInfo{ID: "user-2"})
	hub.mu.Lock()
	hub.clients[client2.ID] = client2
	hub.mu.Unlock()

	err := hub.JoinRoom(client2, "room-1", RoomTypeJob, "Test")
	if err != ErrRoomFull {
		t.Errorf("expected ErrRoomFull, got %v", err)
	}
}

func TestHub_LeaveRoom(t *testing.T) {
	hub := NewHub(DefaultHubConfig())
	conn := &mockWebSocketConn{}
	client := hub.RegisterClient(conn, &UserInfo{ID: "user-1"})

	hub.mu.Lock()
	hub.clients[client.ID] = client
	hub.mu.Unlock()

	hub.JoinRoom(client, "room-1", RoomTypeJob, "Test")
	hub.LeaveRoom(client, "room-1")

	hub.mu.RLock()
	room, exists := hub.rooms["room-1"]
	hub.mu.RUnlock()

	// Room should be deleted when empty
	if exists && len(room.Clients) > 0 {
		t.Error("room should be deleted or empty")
	}
}

func TestHub_UpdatePresence(t *testing.T) {
	hub := NewHub(DefaultHubConfig())
	conn := &mockWebSocketConn{}
	client := hub.RegisterClient(conn, &UserInfo{ID: "user-1"})

	hub.mu.Lock()
	hub.clients[client.ID] = client
	hub.mu.Unlock()

	hub.JoinRoom(client, "room-1", RoomTypeJob, "Test")

	cursor := &Cursor{X: 100, Y: 200}
	selection := &Selection{Type: "text", Start: 0, End: 10}

	hub.UpdatePresence(client, "room-1", "active", "/job/123", cursor, selection)

	hub.mu.RLock()
	room := hub.rooms["room-1"]
	hub.mu.RUnlock()

	room.mu.RLock()
	presence := room.Presence[client.ID]
	room.mu.RUnlock()

	if presence.Status != "active" {
		t.Errorf("status = %s, want active", presence.Status)
	}
	if presence.Location != "/job/123" {
		t.Errorf("location = %s, want /job/123", presence.Location)
	}
	if presence.Cursor.X != 100 {
		t.Errorf("cursor.X = %d, want 100", presence.Cursor.X)
	}
}

func TestHub_GetRoomStats(t *testing.T) {
	hub := NewHub(DefaultHubConfig())

	// Empty room
	count, presence := hub.GetRoomStats("nonexistent")
	if count != 0 || presence != nil {
		t.Error("nonexistent room should return 0 and nil")
	}

	// Create room with client
	conn := &mockWebSocketConn{}
	client := hub.RegisterClient(conn, &UserInfo{ID: "user-1", Name: "Test"})
	hub.mu.Lock()
	hub.clients[client.ID] = client
	hub.mu.Unlock()

	hub.JoinRoom(client, "room-1", RoomTypeJob, "Test")

	count, presence = hub.GetRoomStats("room-1")
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
	if len(presence) != 1 {
		t.Errorf("presence count = %d, want 1", len(presence))
	}
}

func TestHub_GetStats(t *testing.T) {
	hub := NewHub(DefaultHubConfig())

	stats := hub.GetStats()

	if stats["clients"] != 0 {
		t.Errorf("clients = %v, want 0", stats["clients"])
	}
	if stats["rooms"] != 0 {
		t.Errorf("rooms = %v, want 0", stats["rooms"])
	}
}

func TestHub_HandleMessage_Ping(t *testing.T) {
	hub := NewHub(DefaultHubConfig())
	conn := &mockWebSocketConn{}
	client := hub.RegisterClient(conn, &UserInfo{ID: "user-1"})

	msg := Message{Type: MessageTypePing}
	data, _ := json.Marshal(msg)

	oldPing := client.LastPing
	time.Sleep(10 * time.Millisecond)

	err := hub.HandleMessage(client, data)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	if client.LastPing.Equal(oldPing) {
		t.Error("LastPing should be updated")
	}
}

func TestHub_HandleMessage_JoinRoom(t *testing.T) {
	hub := NewHub(DefaultHubConfig())
	conn := &mockWebSocketConn{}
	client := hub.RegisterClient(conn, &UserInfo{ID: "user-1"})

	hub.mu.Lock()
	hub.clients[client.ID] = client
	hub.mu.Unlock()

	payload := map[string]interface{}{
		"room_id":   "room-1",
		"room_type": "job",
		"room_name": "Test Room",
	}
	payloadData, _ := json.Marshal(payload)

	msg := Message{
		Type:    MessageTypeJoinRoom,
		Payload: payloadData,
	}
	data, _ := json.Marshal(msg)

	err := hub.HandleMessage(client, data)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	hub.mu.RLock()
	_, exists := hub.rooms["room-1"]
	hub.mu.RUnlock()

	if !exists {
		t.Error("room should be created")
	}
}

func TestHub_HandleMessage_InvalidJSON(t *testing.T) {
	hub := NewHub(DefaultHubConfig())
	conn := &mockWebSocketConn{}
	client := hub.RegisterClient(conn, &UserInfo{ID: "user-1"})

	err := hub.HandleMessage(client, []byte("invalid json"))
	if err != ErrInvalidMessage {
		t.Errorf("expected ErrInvalidMessage, got %v", err)
	}
}

func TestHub_BroadcastJobUpdate(t *testing.T) {
	hub := NewHub(DefaultHubConfig())
	conn := &mockWebSocketConn{}
	client := hub.RegisterClient(conn, &UserInfo{ID: "user-1"})

	hub.mu.Lock()
	hub.clients[client.ID] = client
	hub.mu.Unlock()

	hub.JoinRoom(client, "job:job-123", RoomTypeJob, "Job 123")

	// Start hub in background to process broadcasts
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	time.Sleep(50 * time.Millisecond)

	hub.BroadcastJobUpdate("job-123", map[string]string{"status": "updated"})

	time.Sleep(50 * time.Millisecond)
}

func TestHub_BroadcastExecutionEvent(t *testing.T) {
	hub := NewHub(DefaultHubConfig())

	// This test just ensures no panic
	hub.BroadcastExecutionEvent("job-1", "exec-1", MessageTypeExecStarted, map[string]string{"status": "running"})
}

func TestMessageTypeConstants(t *testing.T) {
	types := map[MessageType]string{
		MessageTypeJoinRoom:      "join_room",
		MessageTypeLeaveRoom:     "leave_room",
		MessageTypePresence:      "presence",
		MessageTypeCursorMove:    "cursor_move",
		MessageTypeWelcome:       "welcome",
		MessageTypeRoomJoined:    "room_joined",
		MessageTypeUserJoined:    "user_joined",
		MessageTypeJobUpdated:    "job_updated",
		MessageTypeError:         "error",
	}

	for msgType, expected := range types {
		if string(msgType) != expected {
			t.Errorf("MessageType %s = %s, want %s", msgType, string(msgType), expected)
		}
	}
}

func TestRoomTypeConstants(t *testing.T) {
	types := map[RoomType]string{
		RoomTypeDashboard: "dashboard",
		RoomTypeJob:       "job",
		RoomTypeExecution: "execution",
		RoomTypeCluster:   "cluster",
	}

	for roomType, expected := range types {
		if string(roomType) != expected {
			t.Errorf("RoomType %s = %s, want %s", roomType, string(roomType), expected)
		}
	}
}

func TestGenerateUserColor(t *testing.T) {
	color1 := generateUserColor("user-1")
	color2 := generateUserColor("user-2")

	if color1 == "" {
		t.Error("color should not be empty")
	}
	if color1 == color2 {
		t.Error("different users should likely have different colors")
	}
	if color1[0] != '#' {
		t.Error("color should be hex format")
	}
}

func TestMustJSON(t *testing.T) {
	data := map[string]string{"key": "value"}
	result := mustJSON(data)

	var parsed map[string]string
	json.Unmarshal(result, &parsed)

	if parsed["key"] != "value" {
		t.Error("mustJSON should produce valid JSON")
	}
}

func TestConcurrentHubAccess(t *testing.T) {
	hub := NewHub(DefaultHubConfig())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	time.Sleep(50 * time.Millisecond)

	var wg sync.WaitGroup

	// Concurrent client registration
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conn := &mockWebSocketConn{}
			client := hub.RegisterClient(conn, &UserInfo{ID: "user-" + string(rune('0'+i))})
			time.Sleep(10 * time.Millisecond)
			hub.UnregisterClient(client)
		}(i)
	}

	wg.Wait()
}

// Mock WebSocket connection for testing
type mockWebSocketConn struct {
	mu       sync.Mutex
	messages [][]byte
	closed   bool
}

func (m *mockWebSocketConn) ReadMessage() (int, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, nil, ErrConnectionClosed
	}
	return 1, nil, nil
}

func (m *mockWebSocketConn) WriteMessage(messageType int, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrConnectionClosed
	}
	m.messages = append(m.messages, data)
	return nil
}

func (m *mockWebSocketConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockWebSocketConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockWebSocketConn) SetWriteDeadline(t time.Time) error {
	return nil
}
