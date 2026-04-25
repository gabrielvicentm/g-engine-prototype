package main

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

const (
	messageTypeInput      = "input"
	messageTypeWelcome    = "welcome"
	messageTypeWorldState = "world_state"
)

type NetworkMessage struct {
	Type     string               `json:"type"`
	PlayerID int                  `json:"player_id,omitempty"`
	MoveX    float32              `json:"move_x,omitempty"`
	MoveY    float32              `json:"move_y,omitempty"`
	Players  []NetworkPlayerState `json:"players,omitempty"`
}

type NetworkPlayerState struct {
	ID int     `json:"id"`
	X  float32 `json:"x"`
	Y  float32 `json:"y"`
}

type ClientConnection struct {
	conn     net.Conn
	encoder  *json.Encoder
	updates  chan NetworkMessage
	playerID int
	mu       sync.RWMutex
}

func ConnectToServer(addr string) (*ClientConnection, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("falha ao conectar no servidor %q: %w", addr, err)
	}

	client := &ClientConnection{
		conn:    conn,
		encoder: json.NewEncoder(conn),
		updates: make(chan NetworkMessage, 32),
	}

	go client.readLoop()

	return client, nil
}

func (c *ClientConnection) readLoop() {
	defer close(c.updates)

	decoder := json.NewDecoder(c.conn)
	for {
		var msg NetworkMessage
		if err := decoder.Decode(&msg); err != nil {
			return
		}

		if msg.Type == messageTypeWelcome {
			c.mu.Lock()
			c.playerID = msg.PlayerID
			c.mu.Unlock()
			continue
		}

		c.updates <- msg
	}
}

func (c *ClientConnection) SendInput(moveX, moveY float32) error {
	return c.encoder.Encode(NetworkMessage{
		Type:  messageTypeInput,
		MoveX: moveX,
		MoveY: moveY,
	})
}

func (c *ClientConnection) PlayerID() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.playerID
}

func (c *ClientConnection) Close() error {
	return c.conn.Close()
}
