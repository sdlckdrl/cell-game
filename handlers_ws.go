package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"time"
)

func (s *gameState) handleWS(w http.ResponseWriter, r *http.Request) {
	if !headerContainsToken(r.Header, "Connection", "upgrade") || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "upgrade required", http.StatusUpgradeRequired)
		return
	}

	playerID := r.URL.Query().Get("playerId")
	sessionID := r.URL.Query().Get("sessionId")
	if playerID == "" || sessionID == "" {
		http.Error(w, "missing credentials", http.StatusUnauthorized)
		return
	}

	s.mu.Lock()
	p, ok := s.players[playerID]
	if !ok || p.SessionID != sessionID {
		s.mu.Unlock()
		http.Error(w, "invalid session", http.StatusUnauthorized)
		return
	}
	s.mu.Unlock()

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket unsupported", http.StatusInternalServerError)
		return
	}

	conn, rw, err := hj.Hijack()
	if err != nil {
		http.Error(w, "hijack failed", http.StatusInternalServerError)
		return
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		_ = conn.Close()
		return
	}

	accept := websocketAccept(key)
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"

	if _, err := rw.WriteString(response); err != nil {
		_ = conn.Close()
		return
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return
	}

	ws := &wsConn{conn: conn}

	s.mu.Lock()
	if current, exists := s.players[playerID]; exists {
		if current.Conn != nil {
			current.Conn.close()
		}
		current.Conn = ws
		current.LastSeen = time.Now()
	}
	s.mu.Unlock()

	if err := s.sendMetaTo(ws); err != nil {
		s.dropConnection(playerID, ws)
		return
	}

	if err := s.sendSnapshotTo(playerID, ws); err != nil {
		s.dropConnection(playerID, ws)
		return
	}

	go s.readLoop(playerID, ws)
}

func (s *gameState) readLoop(playerID string, ws *wsConn) {
	defer s.dropConnection(playerID, ws)

	for {
		payload, opcode, err := readClientFrame(ws.conn)
		if err != nil {
			return
		}

		if opcode == 0x8 {
			return
		}
		if opcode != 0x1 {
			continue
		}

		var msg inputMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			continue
		}
		if msg.Type == "chat" {
			s.handleChatMessage(playerID, ws, msg.Message)
			continue
		}
		if msg.Type == "upgrade" {
			s.mu.Lock()
			if p, ok := s.players[playerID]; ok && p.Conn == ws {
				s.handleUpgradePurchaseLocked(p, msg.Upgrade)
			}
			s.mu.Unlock()
			continue
		}
		if msg.Type != "input" {
			continue
		}

		s.mu.Lock()
		if p, ok := s.players[playerID]; ok && p.Conn == ws {
			ownerID := ownerIDOf(p)
			now := time.Now()
			fragments := s.ownedPlayersLocked(ownerID)
			for _, fragment := range fragments {
				fragment.Direction.X = clamp(msg.Direction.X, -1, 1)
				fragment.Direction.Y = clamp(msg.Direction.Y, -1, 1)
				fragment.LastSeen = now
				fragment.IsAbilityActive = msg.UseAbility
			}
			if isRespawningAt(now, p) {
				s.mu.Unlock()
				continue
			}
			if msg.UseAbility {
				s.tryUseAbility(p)
			}
			if msg.UseSplit {
				for _, fragment := range fragments {
					s.trySplit(fragment)
				}
			}
			if msg.UseMerge {
				s.tryUpgradeMergeLocked(p)
			}
		}
		s.mu.Unlock()
	}
}

func (s *gameState) handleChatMessage(playerID string, ws *wsConn, raw string) {
	message := sanitizeChatMessage(raw)
	if message == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.players[playerID]
	if !ok || p.Conn != ws {
		return
	}

	entry := chatEntry{
		ID:       randomID(),
		Nickname: p.Nickname,
		Message:  message,
		IsBot:    p.IsBot,
	}
	s.chats = append(s.chats, entry)
	if len(s.chats) > 20 {
		copy(s.chats, s.chats[len(s.chats)-20:])
		s.chats = s.chats[:20]
	}
}

func (s *gameState) dropConnection(playerID string, ws *wsConn) {
	s.mu.Lock()
	if p, ok := s.players[playerID]; ok && p.Conn == ws {
		p.Conn = nil
	}
	s.mu.Unlock()
	ws.close()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func headerContainsToken(h http.Header, key, token string) bool {
	for _, value := range h.Values(key) {
		for _, item := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(item), token) {
				return true
			}
		}
	}
	return false
}

func websocketAccept(key string) string {
	hash := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func readClientFrame(conn net.Conn) ([]byte, byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, 0, err
	}

	opcode := header[0] & 0x0f
	masked := (header[1] & 0x80) != 0
	if !masked {
		return nil, 0, fmt.Errorf("client frames must be masked")
	}

	payloadLen := uint64(header[1] & 0x7f)
	switch payloadLen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(conn, ext); err != nil {
			return nil, 0, err
		}
		payloadLen = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(conn, ext); err != nil {
			return nil, 0, err
		}
		payloadLen = binary.BigEndian.Uint64(ext)
	}

	mask := make([]byte, 4)
	if _, err := io.ReadFull(conn, mask); err != nil {
		return nil, 0, err
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, 0, err
	}

	for i := range payload {
		payload[i] ^= mask[i%4]
	}

	return payload, opcode, nil
}

func (c *wsConn) writeText(payload []byte) error {
	return c.writeFrame(0x81, payload)
}

func (c *wsConn) writeBinary(payload []byte) error {
	return c.writeFrame(0x82, payload)
}

func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("connection closed")
	}

	var header [10]byte
	header[0] = opcode
	headerLen := 2
	length := len(payload)
	switch {
	case length < 126:
		header[1] = byte(length)
	case length <= math.MaxUint16:
		header[1] = 126
		binary.BigEndian.PutUint16(header[2:4], uint16(length))
		headerLen = 4
	default:
		header[1] = 127
		binary.BigEndian.PutUint64(header[2:10], uint64(length))
		headerLen = 10
	}

	segments := net.Buffers{header[:headerLen], payload}
	_, err := segments.WriteTo(c.conn)
	return err
}

func (c *wsConn) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}
