package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
)

type SessionWebSocketMessage struct {
	Type      string `json:"type"`
	SessionID int    `json:"session_id"`
}

type wsClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *wsClient) write(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

type sessionWsHub struct {
	mu      sync.Mutex
	clients map[int]map[*wsClient]bool
}

var sessionHub = &sessionWsHub{
	clients: make(map[int]map[*wsClient]bool),
}

var sessionWsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func SessionWebSocket(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid session id", http.StatusBadRequest)
		return
	}

	conn, err := sessionWsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("websocket upgrade error:", err)
		return
	}

	client := &wsClient{
		conn: conn,
	}

	sessionHub.addClient(sessionID, client)

	defer func() {
		sessionHub.removeClient(sessionID, client)
		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			return
		}
	}
}

func BroadcastSessionUpdated(sessionID int) {
	message := SessionWebSocketMessage{
		Type:      "session_updated",
		SessionID: sessionID,
	}

	sessionHub.broadcast(sessionID, message)
}

func (h *sessionWsHub) addClient(sessionID int, client *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[sessionID] == nil {
		h.clients[sessionID] = make(map[*wsClient]bool)
	}

	h.clients[sessionID][client] = true
}

func (h *sessionWsHub) removeClient(sessionID int, client *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[sessionID] == nil {
		return
	}

	delete(h.clients[sessionID], client)

	if len(h.clients[sessionID]) == 0 {
		delete(h.clients, sessionID)
	}
}

func (h *sessionWsHub) broadcast(sessionID int, message SessionWebSocketMessage) {
	data, err := json.Marshal(message)
	if err != nil {
		log.Println("websocket marshal error:", err)
		return
	}

	h.mu.Lock()

	clients := make([]*wsClient, 0)

	for client := range h.clients[sessionID] {
		clients = append(clients, client)
	}

	h.mu.Unlock()

	for _, client := range clients {
		err := client.write(data)
		if err != nil {
			log.Println("websocket write error:", err)
			h.removeClient(sessionID, client)
			client.conn.Close()
		}
	}
}
