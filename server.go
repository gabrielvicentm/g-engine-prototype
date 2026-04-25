package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/go-gl/mathgl/mgl32"
)

type ServerPlayerState struct {
	ID       int
	Position mgl32.Vec3
	Input    mgl32.Vec2
}

type Server struct {
	listener net.Listener

	mu      sync.Mutex
	nextID  int
	conns   map[int]*serverClient
	players map[int]*ServerPlayerState
}

type serverClient struct {
	conn    net.Conn
	encoder *json.Encoder
	mu      sync.Mutex
}

func RunServer(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("falha ao iniciar listener TCP: %w", err)
	}
	defer listener.Close()

	server := &Server{
		listener: listener,
		nextID:   1,
		conns:    make(map[int]*serverClient),
		players:  make(map[int]*ServerPlayerState),
	}

	log.Println("servidor multiplayer escutando em", addr)

	go server.acceptLoop()
	server.tickLoop()

	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		playerID, spawn := s.addClient(conn)
		log.Printf("cliente conectado id=%d remote=%s", playerID, conn.RemoteAddr())

		if err := sWrite(s, playerID, NetworkMessage{
			Type:     messageTypeWelcome,
			PlayerID: playerID,
		}); err != nil {
			log.Printf("falha ao enviar boas-vindas ao player %d: %v", playerID, err)
			s.removeClient(playerID)
			continue
		}

		log.Printf("player %d spawnado em (%.2f, %.2f)", playerID, spawn.X(), spawn.Y())
		go s.readClientLoop(playerID, conn)
	}
}

func (s *Server) addClient(conn net.Conn) (int, mgl32.Vec3) {
	s.mu.Lock()
	defer s.mu.Unlock()

	playerID := s.nextID
	s.nextID++

	spawn := mgl32.Vec3{
		worldWidth()/2 + float32((playerID-1)%2)*float32(defaultMapTileWidth*3),
		worldHeight()/2 - float32((playerID-1)/2)*float32(defaultMapTileHeight*3),
		0,
	}

	s.conns[playerID] = &serverClient{
		conn:    conn,
		encoder: json.NewEncoder(conn),
	}
	s.players[playerID] = &ServerPlayerState{
		ID:       playerID,
		Position: spawn,
	}

	return playerID, spawn
}

func (s *Server) readClientLoop(playerID int, conn net.Conn) {
	defer s.removeClient(playerID)

	decoder := json.NewDecoder(conn)
	for {
		var msg NetworkMessage
		if err := decoder.Decode(&msg); err != nil {
			log.Printf("cliente %d desconectado: %v", playerID, err)
			return
		}

		if msg.Type != messageTypeInput {
			continue
		}

		s.mu.Lock()
		if player, ok := s.players[playerID]; ok {
			player.Input = mgl32.Vec2{msg.MoveX, msg.MoveY}
		}
		s.mu.Unlock()
	}
}

func (s *Server) removeClient(playerID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if client, ok := s.conns[playerID]; ok {
		client.conn.Close()
		delete(s.conns, playerID)
	}
	delete(s.players, playerID)
}

func (s *Server) tickLoop() {
	ticker := time.NewTicker(time.Duration(float64(time.Second) / serverTickRate))
	defer ticker.Stop()

	deltaTime := float32(1.0 / serverTickRate)
	for range ticker.C {
		s.stepWorld(deltaTime)
		s.broadcastWorldState()
	}
}

func (s *Server) stepWorld(deltaTime float32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, player := range s.players {
		movement := player.Input
		if movement.Len() > 0 {
			movement = movement.Normalize()
		}

		player.Position[0] += movement.X() * playerSpeed * deltaTime
		player.Position[1] += movement.Y() * playerSpeed * deltaTime
		player.Position[0] = clamp(player.Position[0], 0, worldWidth())
		player.Position[1] = clamp(player.Position[1], 0, worldHeight())
	}
}

func (s *Server) broadcastWorldState() {
	s.mu.Lock()
	defer s.mu.Unlock()

	players := make([]NetworkPlayerState, 0, len(s.players))
	for _, player := range s.players {
		players = append(players, NetworkPlayerState{
			ID: player.ID,
			X:  player.Position.X(),
			Y:  player.Position.Y(),
		})
	}

	message := NetworkMessage{
		Type:    messageTypeWorldState,
		Players: players,
	}

	for id, client := range s.conns {
		client.mu.Lock()
		err := client.encoder.Encode(message)
		client.mu.Unlock()
		if err != nil {
			log.Printf("falha ao enviar estado para player %d: %v", id, err)
			client.conn.Close()
			delete(s.conns, id)
			delete(s.players, id)
		}
	}
}

func sWrite(s *Server, playerID int, message NetworkMessage) error {
	s.mu.Lock()
	client, ok := s.conns[playerID]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("player %d nao possui conexao ativa", playerID)
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	return client.encoder.Encode(message)
}

func worldWidth() float32 {
	return float32(defaultMapTilesWide * defaultMapTileWidth)
}

func worldHeight() float32 {
	return float32(defaultMapTilesHigh * defaultMapTileHeight)
}

func clamp(value, minValue, maxValue float32) float32 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
