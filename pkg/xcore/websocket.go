package xcore

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// WebSocket configuration constants
const (
	// Default timeouts
	DefaultWriteTimeout     = 10 * time.Second
	DefaultReadTimeout      = 60 * time.Second
	DefaultHandshakeTimeout = 10 * time.Second
	DefaultPingInterval     = 30 * time.Second
	DefaultPongWait         = 45 * time.Second // Should be > PingInterval
	DefaultMessageSizeLimit = 4096             // 4KB default
	DefaultBufferSize       = 256              // Buffer size for messages

	// Close codes
	CloseNormalClosure           = websocket.CloseNormalClosure
	CloseGoingAway               = websocket.CloseGoingAway
	CloseProtocolError           = websocket.CloseProtocolError
	CloseUnsupportedData         = websocket.CloseUnsupportedData
	CloseNoStatusReceived        = websocket.CloseNoStatusReceived
	CloseAbnormalClosure         = websocket.CloseAbnormalClosure
	CloseInvalidFramePayloadData = websocket.CloseInvalidFramePayloadData
	ClosePolicyViolation         = websocket.ClosePolicyViolation
	CloseMessageTooBig           = websocket.CloseMessageTooBig
	CloseMandatoryExtension      = websocket.CloseMandatoryExtension
	CloseInternalServerErr       = websocket.CloseInternalServerErr
	CloseServiceRestart          = websocket.CloseServiceRestart
	CloseTryAgainLater           = websocket.CloseTryAgainLater
	CloseTLSHandshake            = websocket.CloseTLSHandshake
)

// WebSocketConfig holds configuration for WebSocket server
type WebSocketConfig struct {
	// Timeouts
	WriteTimeout     time.Duration `mapstructure:"write_timeout"`
	ReadTimeout      time.Duration `mapstructure:"read_timeout"`
	HandshakeTimeout time.Duration `mapstructure:"handshake_timeout"`
	PingInterval     time.Duration `mapstructure:"ping_interval"`
	PongWait         time.Duration `mapstructure:"pong_wait"`

	// Limits
	MessageSizeLimit int64 `mapstructure:"message_size_limit"`
	BufferSize       int   `mapstructure:"buffer_size"`
	MaxConnections   int   `mapstructure:"max_connections"`

	// Enable compression
	EnableCompression bool `mapstructure:"enable_compression"`

	// Allowed origins for CORS (empty means all)
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

// DefaultWebSocketConfig returns a default WebSocket configuration
func DefaultWebSocketConfig() *WebSocketConfig {
	return &WebSocketConfig{
		WriteTimeout:      DefaultWriteTimeout,
		ReadTimeout:       DefaultReadTimeout,
		HandshakeTimeout:  DefaultHandshakeTimeout,
		PingInterval:      DefaultPingInterval,
		PongWait:          DefaultPongWait,
		MessageSizeLimit:  DefaultMessageSizeLimit,
		BufferSize:        DefaultBufferSize,
		MaxConnections:    10000,
		EnableCompression: false,
		AllowedOrigins:    []string{"*"},
	}
}

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type      int         `json:"type"` // websocket.TextMessage, websocket.BinaryMessage
	Data      []byte      `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	Metadata  interface{} `json:"metadata,omitempty"`
}

// WSConnection represents a WebSocket connection with metadata
type WSConnection struct {
	ID         string
	Conn       *websocket.Conn
	Hub        *WSHub
	Mu         sync.RWMutex
	IsClosed   bool
	CreatedAt  time.Time
	LastActive time.Time
	Metadata   map[string]interface{}
	sendChan   chan []byte
}

// WSHub manages all WebSocket connections
type WSHub struct {
	mu           sync.RWMutex
	connections  map[string]*WSConnection
	broadcast    chan *WSBroadcastMessage
	register     chan *WSConnection
	unregister   chan *WSConnection
	config       *WebSocketConfig
	logger       *zerolog.Logger
	authFunc     WSAuthFunc
	onConnect    WSConnectFunc
	onDisconnect WSDisconnectFunc
	onMessage    WSMessageFunc
	errorHandler WSErrorHandler
	ctx          context.Context
	cancel       context.CancelFunc
}

// WSBroadcastMessage represents a message to broadcast
type WSBroadcastMessage struct {
	Data       []byte
	ExcludeIDs []string
	IncludeIDs []string // If set, only send to these IDs
	Predicate  func(*WSConnection) bool
}

// WSAuthFunc is the authentication function signature
type WSAuthFunc func(r *http.Request) (string, interface{}, error)

// WSConnectFunc is called when a new connection is established
type WSConnectFunc func(conn *WSConnection)

// WSDisconnectFunc is called when a connection is closed
type WSDisconnectFunc func(conn *WSConnection, code int, reason string)

// WSMessageFunc is called when a message is received
type WSMessageFunc func(conn *WSConnection, message *WSMessage)

// WSErrorHandler handles errors
type WSErrorHandler func(conn *WSConnection, err error)

// NewWSHub creates a new WebSocket hub
func NewWSHub(config *WebSocketConfig, logger *zerolog.Logger) *WSHub {
	ctx, cancel := context.WithCancel(context.Background())

	hub := &WSHub{
		connections: make(map[string]*WSConnection),
		broadcast:   make(chan *WSBroadcastMessage, 256),
		register:    make(chan *WSConnection),
		unregister:  make(chan *WSConnection),
		config:      config,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}

	return hub
}

// SetAuthFunc sets the authentication function
func (h *WSHub) SetAuthFunc(fn WSAuthFunc) {
	h.authFunc = fn
}

// SetOnConnectFunc sets the on-connect callback
func (h *WSHub) SetOnConnectFunc(fn WSConnectFunc) {
	h.onConnect = fn
}

// SetOnDisconnectFunc sets the on-disconnect callback
func (h *WSHub) SetOnDisconnectFunc(fn WSDisconnectFunc) {
	h.onDisconnect = fn
}

// SetOnMessageFunc sets the on-message callback
func (h *WSHub) SetOnMessageFunc(fn WSMessageFunc) {
	h.onMessage = fn
}

// SetErrorHandler sets the error handler
func (h *WSHub) SetErrorHandler(fn WSErrorHandler) {
	h.errorHandler = fn
}

// Run starts the hub's main loop
func (h *WSHub) Run() {
	h.logger.Info().Msg("WebSocket hub started")

	for {
		select {
		case <-h.ctx.Done():
			h.shutdown()
			return
		case conn := <-h.register:
			h.handleRegister(conn)
		case conn := <-h.unregister:
			h.handleUnregister(conn)
		case msg := <-h.broadcast:
			h.handleBroadcast(msg)
		}
	}
}

// Shutdown gracefully shuts down the hub
func (h *WSHub) Shutdown() {
	h.logger.Info().Msg("Shutting down WebSocket hub...")
	h.cancel()
}

func (h *WSHub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, conn := range h.connections {
		conn.Close(CloseGoingAway, "Server shutting down")
	}

	h.logger.Info().Int("connections", len(h.connections)).Msg("WebSocket hub stopped")
}

func (h *WSHub) handleRegister(conn *WSConnection) {
	h.mu.Lock()

	// Check max connections
	if h.config.MaxConnections > 0 && len(h.connections) >= h.config.MaxConnections {
		h.mu.Unlock()
		conn.Close(CloseServiceRestart, "Server at capacity")
		h.logger.Warn().Msg("Rejected connection: server at capacity")
		return
	}

	h.connections[conn.ID] = conn
	h.mu.Unlock()

	h.logger.Debug().Str("connection_id", conn.ID).Msg("Client connected")

	if h.onConnect != nil {
		h.onConnect(conn)
	}

	// Start connection handlers
	go conn.handleRead()
	go conn.handleWrite()
	go conn.handlePing()
}

func (h *WSHub) handleUnregister(conn *WSConnection) {
	h.mu.Lock()
	if _, ok := h.connections[conn.ID]; ok {
		delete(h.connections, conn.ID)
	}
	h.mu.Unlock()

	conn.Mu.Lock()
	if !conn.IsClosed {
		conn.IsClosed = true
		close(conn.sendChan)
	}
	conn.Mu.Unlock()

	h.logger.Debug().Str("connection_id", conn.ID).Msg("Client disconnected")
}

func (h *WSHub) handleBroadcast(msg *WSBroadcastMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// If IncludeIDs is set, only send to those connections
	if len(msg.IncludeIDs) > 0 {
		for _, id := range msg.IncludeIDs {
			if conn, ok := h.connections[id]; ok {
				select {
				case conn.sendChan <- msg.Data:
				default:
					h.logger.Warn().Str("connection_id", id).Msg("Failed to send message: buffer full")
				}
			}
		}
		return
	}

	// Otherwise, use predicate or send to all except excluded
	for _, conn := range h.connections {
		// Check if excluded
		excluded := false
		for _, id := range msg.ExcludeIDs {
			if id == conn.ID {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		// Check predicate
		if msg.Predicate != nil && !msg.Predicate(conn) {
			continue
		}

		select {
		case conn.sendChan <- msg.Data:
		default:
			h.logger.Warn().Str("connection_id", conn.ID).Msg("Failed to send message: buffer full")
		}
	}
}

// GetConnection returns a connection by ID
func (h *WSHub) GetConnection(id string) (*WSConnection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conn, ok := h.connections[id]
	return conn, ok
}

// GetConnectionCount returns the number of active connections
func (h *WSHub) GetConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// GetAllConnectionIDs returns all connection IDs
func (h *WSHub) GetAllConnectionIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ids := make([]string, 0, len(h.connections))
	for id := range h.connections {
		ids = append(ids, id)
	}
	return ids
}

// Broadcast sends a message to all connections
func (h *WSHub) Broadcast(data []byte) {
	h.broadcast <- &WSBroadcastMessage{Data: data}
}

// BroadcastJSON sends a JSON message to all connections
func (h *WSHub) BroadcastJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	h.Broadcast(data)
	return nil
}

// BroadcastTo sends a message to specific connections
func (h *WSHub) BroadcastTo(ids []string, data []byte) {
	h.broadcast <- &WSBroadcastMessage{Data: data, IncludeIDs: ids}
}

// BroadcastExcept sends a message to all connections except the specified ones
func (h *WSHub) BroadcastExcept(excludeIDs []string, data []byte) {
	h.broadcast <- &WSBroadcastMessage{Data: data, ExcludeIDs: excludeIDs}
}

// BroadcastIf sends a message to connections matching the predicate
func (h *WSHub) BroadcastIf(predicate func(*WSConnection) bool, data []byte) {
	h.broadcast <- &WSBroadcastMessage{Data: data, Predicate: predicate}
}

// Disconnect closes a specific connection
func (h *WSHub) Disconnect(id string, code int, reason string) {
	h.mu.RLock()
	conn, ok := h.connections[id]
	h.mu.RUnlock()

	if ok {
		conn.Close(code, reason)
	}
}

// DisconnectAll closes all connections
func (h *WSHub) DisconnectAll(code int, reason string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, conn := range h.connections {
		conn.Close(code, reason)
	}
}

// handleRead handles reading messages from the connection
func (c *WSConnection) handleRead() {
	defer func() {
		c.Hub.unregister <- c
	}()

	c.Conn.SetReadLimit(c.Hub.config.MessageSizeLimit)
	c.Conn.SetReadDeadline(time.Now().Add(c.Hub.config.ReadTimeout))
	c.Conn.SetPongHandler(c.pongHandler)

	for {
		select {
		case <-c.Hub.ctx.Done():
			return
		default:
			msgType, data, err := c.Conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.Hub.logger.Error().Err(err).Str("connection_id", c.ID).Msg("WebSocket read error")
				}
				if c.Hub.errorHandler != nil {
					c.Hub.errorHandler(c, err)
				}
				return
			}

			c.LastActive = time.Now()

			message := &WSMessage{
				Type:      msgType,
				Data:      data,
				Timestamp: time.Now(),
			}

			if c.Hub.onMessage != nil {
				c.Hub.onMessage(c, message)
			}
		}
	}
}

// handleWrite handles writing messages to the connection
func (c *WSConnection) handleWrite() {
	ticker := time.NewTicker(c.Hub.config.PingInterval)
	defer func() {
		ticker.Stop()
		c.Hub.unregister <- c
	}()

	for {
		select {
		case <-c.Hub.ctx.Done():
			return
		case <-ticker.C:
			// Ping will be sent by handlePing
		case message, ok := <-c.sendChan:
			c.Conn.SetWriteDeadline(time.Now().Add(c.Hub.config.WriteTimeout))
			if !ok {
				// Channel closed, hub closed the connection
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				if c.Hub.errorHandler != nil {
					c.Hub.errorHandler(c, err)
				}
				return
			}

			if _, err := w.Write(message); err != nil {
				w.Close()
				if c.Hub.errorHandler != nil {
					c.Hub.errorHandler(c, err)
				}
				return
			}

			if err := w.Close(); err != nil {
				if c.Hub.errorHandler != nil {
					c.Hub.errorHandler(c, err)
				}
				return
			}
		}
	}
}

// handlePing sends ping messages to keep the connection alive
func (c *WSConnection) handlePing() {
	ticker := time.NewTicker(c.Hub.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.Hub.ctx.Done():
			return
		case <-ticker.C:
			c.Mu.RLock()
			if c.IsClosed {
				c.Mu.RUnlock()
				return
			}
			c.Mu.RUnlock()

			c.Conn.SetWriteDeadline(time.Now().Add(c.Hub.config.WriteTimeout))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				if c.Hub.errorHandler != nil {
					c.Hub.errorHandler(c, err)
				}
				return
			}
		}
	}
}

// pongHandler handles pong messages from the client
func (c *WSConnection) pongHandler(appData string) error {
	c.Conn.SetReadDeadline(time.Now().Add(c.Hub.config.PongWait))
	c.LastActive = time.Now()
	return nil
}

// Send sends a message to this connection
func (c *WSConnection) Send(data []byte) error {
	c.Mu.RLock()
	if c.IsClosed {
		c.Mu.RUnlock()
		return websocket.ErrCloseSent
	}
	c.Mu.RUnlock()

	select {
	case c.sendChan <- data:
		return nil
	default:
		return websocket.ErrCloseSent
	}
}

// SendJSON sends a JSON message to this connection
func (c *WSConnection) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Send(data)
}

// Close closes the connection
func (c *WSConnection) Close(code int, reason string) {
	c.Mu.Lock()
	if c.IsClosed {
		c.Mu.Unlock()
		return
	}
	c.IsClosed = true
	c.Mu.Unlock()

	message := websocket.FormatCloseMessage(code, reason)
	c.Conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(c.Hub.config.WriteTimeout))
	c.Conn.Close()

	if c.Hub.onDisconnect != nil {
		c.Hub.onDisconnect(c, code, reason)
	}
}

// GetMetadata gets a metadata value
func (c *WSConnection) GetMetadata(key string) (interface{}, bool) {
	c.Mu.RLock()
	defer c.Mu.RUnlock()
	val, ok := c.Metadata[key]
	return val, ok
}

// SetMetadata sets a metadata value
func (c *WSConnection) SetMetadata(key string, value interface{}) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	if c.Metadata == nil {
		c.Metadata = make(map[string]interface{})
	}
	c.Metadata[key] = value
}

// GetRemoteAddr returns the remote address
func (c *WSConnection) GetRemoteAddr() string {
	if addr, ok := c.Conn.RemoteAddr().(*net.TCPAddr); ok {
		return addr.IP.String()
	}
	return c.Conn.RemoteAddr().String()
}

// WSHandler creates an HTTP handler for WebSocket connections
func (h *WSHub) WSHandler() http.HandlerFunc {
	upgrader := &websocket.Upgrader{
		ReadBufferSize:    h.config.BufferSize,
		WriteBufferSize:   h.config.BufferSize,
		HandshakeTimeout:  h.config.HandshakeTimeout,
		EnableCompression: h.config.EnableCompression,
		CheckOrigin:       h.createOriginChecker(),
		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
			h.logger.Error().Err(reason).Int("status", status).Msg("WebSocket handshake error")
			rb := NewResponseBuilder(w, r)
			rb.JSON(status, Response{
				Code:     "WEBSOCKET_ERROR",
				Message:  reason.Error(),
				Metadata: Metadata{RequestID: GetRequestID(r), Timestamp: time.Now().UTC().Format(time.RFC3339)},
			})
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r)
		startTime := time.Now()

		// Authenticate
		if h.authFunc != nil {
			connID, metadata, err := h.authFunc(r)
			if err != nil {
				h.logger.Warn().Err(err).Str("request_id", requestID).Msg("WebSocket authentication failed")
				rb := NewResponseBuilder(w, r)
				rb.Unauthorized("Authentication failed: " + err.Error())
				return
			}

			// Store metadata in context for later use
			r = r.WithContext(context.WithValue(r.Context(), contextKey("ws_conn_id"), connID))
			r = r.WithContext(context.WithValue(r.Context(), contextKey("ws_metadata"), metadata))
		}

		// Upgrade connection
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			h.logger.Error().Err(err).Str("request_id", requestID).Msg("WebSocket upgrade failed")
			return
		}

		// Get or generate connection ID
		connID, _ := r.Context().Value(contextKey("ws_conn_id")).(string)
		if connID == "" {
			connID = generateConnectionID()
		}

		// Get metadata from context
		metadata, _ := r.Context().Value(contextKey("ws_metadata")).(map[string]interface{})

		// Create WSConnection
		wsConn := &WSConnection{
			ID:         connID,
			Conn:       conn,
			Hub:        h,
			IsClosed:   false,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
			Metadata:   metadata,
			sendChan:   make(chan []byte, h.config.BufferSize),
		}

		rb := NewRequestBuilder(r)
		h.logger.Info().
			Str("connection_id", connID).
			Str("request_id", requestID).
			Str("ip", rb.GetClientIP()).
			Dur("handshake_duration", time.Since(startTime)).
			Msg("WebSocket connection established")

		// Register connection
		h.register <- wsConn
	}
}

// createOriginChecker creates a function to check allowed origins
func (h *WSHub) createOriginChecker() func(r *http.Request) bool {
	if len(h.config.AllowedOrigins) == 0 {
		return func(r *http.Request) bool { return true }
	}

	// Check if all origins are allowed
	for _, origin := range h.config.AllowedOrigins {
		if origin == "*" {
			return func(r *http.Request) bool { return true }
		}
	}

	// Check specific origins
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		for _, allowed := range h.config.AllowedOrigins {
			if allowed == origin {
				return true
			}
		}
		return false
	}
}

// generateConnectionID generates a unique connection ID
func generateConnectionID() string {
	return time.Now().Format("20060102150405.000000") + "-" + randomString(8)
}

// randomString generates a random string of given length
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond) // Ensure different values
	}
	return string(b)
}

// WSResponse is a standard WebSocket response structure
type WSResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// NewWSResponse creates a new WebSocket response
func NewWSResponse(success bool, message string, data interface{}) *WSResponse {
	return &WSResponse{
		Success:   success,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewWSErrorResponse creates a new WebSocket error response
func NewWSErrorResponse(message string, requestID string) *WSResponse {
	return &WSResponse{
		Success:   false,
		Message:   message,
		Error:     message,
		RequestID: requestID,
		Timestamp: time.Now(),
	}
}

// WSRequest is a standard WebSocket request structure
type WSRequest struct {
	Action    string      `json:"action"`
	Payload   interface{} `json:"payload,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

// Middleware for WebSocket connections

// rateCounter is used for rate limiting
type rateCounter struct {
	count     int
	resetTime time.Time
}

// WSRateLimitMiddleware creates a rate limiting middleware for WebSocket connections
func WSRateLimitMiddleware(hub *WSHub, maxMessages int, window time.Duration) func(*WSConnection, *WSMessage) {
	counters := make(map[string]*rateCounter)
	var mu sync.Mutex

	return func(conn *WSConnection, msg *WSMessage) {
		mu.Lock()
		defer mu.Unlock()

		now := time.Now()
		counter, exists := counters[conn.ID]

		if !exists || now.After(counter.resetTime) {
			counters[conn.ID] = &rateCounter{
				count:     1,
				resetTime: now.Add(window),
			}
			return
		}

		counter.count++
		if counter.count > maxMessages {
			hub.logger.Warn().
				Str("connection_id", conn.ID).
				Int("count", counter.count).
				Msg("WebSocket rate limit exceeded")

			conn.Close(ClosePolicyViolation, "Rate limit exceeded")
			delete(counters, conn.ID)
		}
	}
}

// WSLoggerMiddleware creates a logging middleware for WebSocket messages
func WSLoggerMiddleware(hub *WSHub) func(*WSConnection, *WSMessage) {
	return func(conn *WSConnection, msg *WSMessage) {
		hub.logger.Debug().
			Str("connection_id", conn.ID).
			Int("type", msg.Type).
			Int("size", len(msg.Data)).
			Msg("WebSocket message received")
	}
}
