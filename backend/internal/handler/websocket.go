package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"

	"github.com/topic-voting/backend/internal/model"
)

type WSBroadcaster interface {
	BroadcastLeaderboard(topicID uuid.UUID)
	BroadcastChatMessage(topicID uuid.UUID, msg *wsMessage)
}

type WebSocketHub struct {
	mu         sync.RWMutex
	dashboard  map[uuid.UUID]map[*Client]struct{}
	chat       map[uuid.UUID]map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	getLB      func(uuid.UUID) (*model.Leaderboard, error)
	quit       chan struct{}
}

func NewWebSocketHub(getLB func(uuid.UUID) (*model.Leaderboard, error)) *WebSocketHub {
	return &WebSocketHub{
		dashboard:  make(map[uuid.UUID]map[*Client]struct{}),
		chat:       make(map[uuid.UUID]map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		getLB:      getLB,
		quit:       make(chan struct{}),
	}
}

func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if client.kind == "dashboard" {
				if _, ok := h.dashboard[client.topicID]; !ok {
					h.dashboard[client.topicID] = make(map[*Client]struct{})
				}
				h.dashboard[client.topicID][client] = struct{}{}
			} else if client.kind == "chat" {
				if _, ok := h.chat[client.topicID]; !ok {
					h.chat[client.topicID] = make(map[*Client]struct{})
				}
				h.chat[client.topicID][client] = struct{}{}
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.dashboard[client.topicID]; ok {
				delete(h.dashboard[client.topicID], client)
				if len(h.dashboard[client.topicID]) == 0 {
					delete(h.dashboard, client.topicID)
				}
			}
			if _, ok := h.chat[client.topicID]; ok {
				delete(h.chat[client.topicID], client)
				if len(h.chat[client.topicID]) == 0 {
					delete(h.chat, client.topicID)
				}
			}
			h.mu.Unlock()
			close(client.send)

		case <-h.quit:
			return
		}
	}
}

func (h *WebSocketHub) BroadcastLeaderboard(topicID uuid.UUID) {
	lb, err := h.getLB(topicID)
	if err != nil || lb == nil {
		return
	}

	msg, err := json.Marshal(wsMessage{
		Type: "leaderboard_update",
		Data: lb,
	})
	if err != nil {
		return
	}

	h.mu.RLock()
	clients := h.dashboard[topicID]
	h.mu.RUnlock()

	for client := range clients {
		select {
		case client.send <- msg:
		default:
			close(client.send)
		}
	}
}

func (h *WebSocketHub) BroadcastChatMessage(topicID uuid.UUID, msg *wsMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	clients := h.chat[topicID]
	h.mu.RUnlock()

	for client := range clients {
		select {
		case client.send <- data:
		default:
			close(client.send)
		}
	}
}

func (h *WebSocketHub) Stop() {
	close(h.quit)
}

type Client struct {
	conn    *websocket.Conn
	send    chan []byte
	hub     *WebSocketHub
	topicID uuid.UUID
	kind    string
}

func (c *Client) readPump(pingInterval, pongTimeout time.Duration) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	for {
		_, message, err := c.conn.Read(context.Background())
		if err != nil {
			break
		}

		if c.kind == "chat" {
			c.handleChatMessage(message)
		}
	}
}

func (c *Client) handleChatMessage(raw []byte) {
	var msg struct {
		Type string `json:"type"`
		Data struct {
			Username   string `json:"username"`
			Message    string `json:"message"`
			IsDonation bool   `json:"is_donation"`
			BitsAmount int    `json:"bits_amount"`
		} `json:"data"`
	}

	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	if msg.Type != "chat_message" {
		return
	}

	voteReq := model.SubmitVoteRequest{
		TopicID:    c.topicID,
		Username:   msg.Data.Username,
		Message:    msg.Data.Message,
		IsDonation: msg.Data.IsDonation,
		BitsAmount: msg.Data.BitsAmount,
	}

	payload, err := json.Marshal(voteReq)
	if err != nil {
		return
	}

	resp, err := http.Post(
		"http://localhost:8585/api/votes",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		log.Printf("[ws/chat] internal vote request error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		log.Printf("[ws/chat] internal vote request status: %d", resp.StatusCode)
		return
	}
}

func (c *Client) writePump(pingInterval time.Duration) {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.Write(context.Background(), websocket.StatusNormalClosure, []byte{})
				return
			}
			c.conn.Write(context.Background(), websocket.MessageText, message)

		case <-ticker.C:
			c.conn.Ping(context.Background())
		}
	}
}

type wsMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func (h *WebSocketHub) HandleDashboard(ws *websocket.Conn, topicID uuid.UUID, pingInterval, pongTimeout time.Duration) {
	client := &Client{
		conn:    ws,
		send:    make(chan []byte, 256),
		hub:     h,
		topicID: topicID,
		kind:    "dashboard",
	}

	h.register <- client

	lb, err := h.getLB(topicID)
	if err == nil && lb != nil {
		msg, _ := json.Marshal(wsMessage{
			Type: "leaderboard_update",
			Data: lb,
		})
		select {
		case client.send <- msg:
		default:
		}
	}

	go client.writePump(pingInterval)
	client.readPump(pingInterval, pongTimeout)
}

func (h *WebSocketHub) HandleChat(ws *websocket.Conn, topicID uuid.UUID, pingInterval, pongTimeout time.Duration) {
	client := &Client{
		conn:    ws,
		send:    make(chan []byte, 256),
		hub:     h,
		topicID: topicID,
		kind:    "chat",
	}

	h.register <- client

	go client.writePump(pingInterval)
	client.readPump(pingInterval, pongTimeout)
}
