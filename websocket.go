package xcore

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WSMessageType int

const (
	WSMessageText   WSMessageType = 1
	WSMessageBinary WSMessageType = 2
	WSMessageClose  WSMessageType = 3
	WSMessagePing   WSMessageType = 4
	WSMessagePong   WSMessageType = 5
)

type WSMessage struct {
	Type    WSMessageType `json:"type"`
	Payload []byte        `json:"payload"`
	Room    string        `json:"room,omitempty"`
}

type WSAuthFunc func(r *http.Request) (bool, string)

type WSConnection struct {
	conn      *websocket.Conn
	send      chan []byte
	ws        *WebSocket
	done      chan struct{}
	mu        sync.Mutex
	id        string
	rooms     map[string]bool
	userAgent string
}

func NewWSConnection(conn *websocket.Conn, ws *WebSocket, id string) *WSConnection {
	return &WSConnection{
		conn:  conn,
		send:  make(chan []byte, 256),
		ws:    ws,
		done:  make(chan struct{}),
		id:    id,
		rooms: make(map[string]bool),
	}
}

func (c *WSConnection) ReadLoop() {
	defer func() {
		c.ws.hub.unregister <- c
		c.close()
	}()

	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				if c.ws.logger != nil {
					c.ws.logger.Error().Err(err).Msg("websocket read error")
				}
			}
			break
		}

		wsType := WSMessageText
		if messageType == websocket.BinaryMessage {
			wsType = WSMessageBinary
		}

		c.ws.handleMessage(c, wsType, message)
	}
}

func (c *WSConnection) WriteLoop() {
	ticker := time.NewTicker(time.Duration(c.ws.config.PingInterval) * time.Second)
	defer func() {
		ticker.Stop()
		c.close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if message == nil {
				continue
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.done:
			return
		}
	}
}

func (c *WSConnection) Send(message []byte) bool {
	select {
	case c.send <- message:
		return true
	default:
		return false
	}
}

func (c *WSConnection) SendText(msg string) bool {
	return c.Send([]byte(msg))
}

func (c *WSConnection) SendBinary(msg []byte) bool {
	return c.Send(msg)
}

func (c *WSConnection) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.Send(data)
	return nil
}

func (c *WSConnection) Close() error {
	c.close()
	return nil
}

func (c *WSConnection) close() {
	select {
	case <-c.done:
		return
	default:
		close(c.done)
	}
	close(c.send)
	c.conn.Close()
}

func (c *WSConnection) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

func (c *WSConnection) ID() string {
	return c.id
}

func (c *WSConnection) JoinRoom(room string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rooms[room] = true
	c.ws.hub.mu.Lock()
	defer c.ws.hub.mu.Unlock()
	if c.ws.hub.rooms[room] == nil {
		c.ws.hub.rooms[room] = make(map[*WSConnection]bool)
	}
	c.ws.hub.rooms[room][c] = true
}

func (c *WSConnection) LeaveRoom(room string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.rooms, room)
	c.ws.hub.mu.Lock()
	defer c.ws.hub.mu.Unlock()
	if c.ws.hub.rooms[room] != nil {
		delete(c.ws.hub.rooms[room], c)
	}
}

func (c *WSConnection) Rooms() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	rooms := make([]string, 0, len(c.rooms))
	for r := range c.rooms {
		rooms = append(rooms, r)
	}
	return rooms
}

type WebSocket struct {
	config      *WebsocketConfig
	logger      *Logger
	hub         *Hub
	register    chan *WSConnection
	unregister  chan *WSConnection
	broadcast   chan []byte
	authHandler WSAuthFunc
	upgrader    websocket.Upgrader
}

type Hub struct {
	connections map[*WSConnection]bool
	rooms       map[string]map[*WSConnection]bool
	broadcast   chan []byte
	register    chan *WSConnection
	unregister  chan *WSConnection
	stop        chan struct{}
	mu          sync.RWMutex
}

func newHub() *Hub {
	return &Hub{
		connections: make(map[*WSConnection]bool),
		rooms:       make(map[string]map[*WSConnection]bool),
		broadcast:   make(chan []byte, 256),
		register:    make(chan *WSConnection),
		unregister:  make(chan *WSConnection),
		stop:        make(chan struct{}),
	}
}

func (h *Hub) run() {
	for {
		select {
		case <-h.stop:
			return
		case conn := <-h.register:
			h.mu.Lock()
			h.connections[conn] = true
			h.mu.Unlock()

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.connections[conn]; ok {
				delete(h.connections, conn)
				for room := range conn.rooms {
					if h.rooms[room] != nil {
						delete(h.rooms[room], conn)
					}
				}
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for conn := range h.connections {
				select {
				case conn.send <- message:
				default:
					close(conn.send)
					delete(h.connections, conn)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) BroadcastToRoom(room string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if conns, ok := h.rooms[room]; ok {
		for conn := range conns {
			select {
			case conn.send <- message:
			default:
			}
		}
	}
}

func (h *Hub) Stop() {
	select {
	case h.stop <- struct{}{}:
	default:
	}
}

func NewWebSocket(cfg *WebsocketConfig, logger *Logger) *WebSocket {
	if cfg == nil {
		cfg = &WebsocketConfig{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			PingInterval:    30,
			PongTimeout:     10,
			MaxMessageSize:  4096,
		}
	}

	hub := newHub()
	go hub.run()

	upgrader := websocket.Upgrader{
		ReadBufferSize:  cfg.ReadBufferSize,
		WriteBufferSize: cfg.WriteBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	ws := &WebSocket{
		config:     cfg,
		logger:     logger,
		hub:        hub,
		register:   hub.register,
		unregister: hub.unregister,
		broadcast:  hub.broadcast,
		upgrader:   upgrader,
	}

	return ws
}

func (ws *WebSocket) WithAuth(authFn WSAuthFunc) *WebSocket {
	ws.authHandler = authFn
	return ws
}

func (ws *WebSocket) WithUpgrader(upgrader websocket.Upgrader) *WebSocket {
	ws.upgrader = upgrader
	return ws
}

func (ws *WebSocket) handleMessage(conn *WSConnection, msgType WSMessageType, message []byte) {
	if ws.logger != nil {
		ws.logger.Debug().Str("remote", conn.RemoteAddr()).Msg("websocket message received")
	}
}

func (ws *WebSocket) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	if ws.authHandler != nil {
		ok, id := ws.authHandler(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if id == "" {
			id = generateWSID()
		}
		conn, err := ws.upgrader.Upgrade(w, r, nil)
		if err != nil {
			if ws.logger != nil {
				ws.logger.Error().Err(err).Msg("websocket upgrade failed")
			}
			return
		}
		wsConn := NewWSConnection(conn, ws, id)
		ws.hub.register <- wsConn
		go wsConn.WriteLoop()
		go wsConn.ReadLoop()
		return
	}

	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		if ws.logger != nil {
			ws.logger.Error().Err(err).Msg("websocket upgrade failed")
		}
		return
	}

	wsConn := NewWSConnection(conn, ws, generateWSID())
	ws.hub.register <- wsConn

	go wsConn.WriteLoop()
	go wsConn.ReadLoop()
}

func (ws *WebSocket) HandleFunc(w http.ResponseWriter, r *http.Request) {
	ws.HandleHTTP(w, r)
}

func (ws *WebSocket) Broadcast(message []byte) {
	ws.hub.broadcast <- message
}

func (ws *WebSocket) BroadcastText(msg string) {
	ws.hub.broadcast <- []byte(msg)
}

func (ws *WebSocket) BroadcastJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	ws.hub.broadcast <- data
	return nil
}

func (ws *WebSocket) BroadcastToRoom(room string, message []byte) {
	ws.hub.BroadcastToRoom(room, message)
}

func (ws *WebSocket) BroadcastToRoomText(room string, msg string) {
	ws.hub.BroadcastToRoom(room, []byte(msg))
}

func (ws *WebSocket) BroadcastToRoomJSON(room string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	ws.hub.BroadcastToRoom(room, data)
	return nil
}

func (ws *WebSocket) Hub() *Hub {
	return ws.hub
}

func (ws *WebSocket) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws.HandleHTTP(w, r)
}

func (ws *WebSocket) Shutdown() {
	ws.hub.BroadcastToRoom("", []byte{})
	ws.hub.mu.Lock()
	for conn := range ws.hub.connections {
		conn.close()
	}
	ws.hub.connections = make(map[*WSConnection]bool)
	ws.hub.rooms = make(map[string]map[*WSConnection]bool)
	ws.hub.mu.Unlock()
	ws.hub.Stop()
}

func generateWSID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	seed := time.Now().UnixNano()
	for i := range b {
		seed = (seed*1103515245 + 12345) & 0x7fffffff
		b[i] = letters[seed%int64(len(letters))]
	}
	return string(b)
}
