package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	mathrand "math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	port                  = "8000"
	worldSize             = 3600.0
	foodTarget            = 320
	cactusTarget          = 28
	defaultWormholePairs  = 3
	playerStartMass       = 36.0
	tickRate              = 30
	playerTimeout         = 60 * time.Second
	playerCullRange       = 1280.0
	foodCullRange         = 1460.0
	objectCullRange       = 1600.0
	defaultMinimumPlayers = 20
	defaultBaseSpeed      = 285.0
	defaultSpeedDivisor   = 8.5
	defaultMinimumSpeed   = 92.0
	dividerSplitCooldown  = 1400 * time.Millisecond
	dividerMergeDelay     = 7 * time.Second
	dividerMinSplitMass   = 40.0
	dividerMaxFragments   = 16

	// 공간 분할(Spatial Partitioning) 관련 상수
	spatialCellSize = 500.0
	spatialGridCols = (int(worldSize) / int(spatialCellSize)) + 1
	spatialGridRows = spatialGridCols
)

var mimeTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".js":   "text/javascript; charset=utf-8",
	".json": "application/json; charset=utf-8",
}

type gameState struct {
	mu           sync.RWMutex
	players      map[string]*player
	foods        []*food
	cacti        []*cactus
	wormholes    []*wormhole
	config       runtimeConfig
	spatialCache *spatialGrid
}

type runtimeConfig struct {
	MinimumPlayers int     `json:"minimumPlayers"`
	WormholePairs  int     `json:"wormholePairs"`
	BaseSpeed      float64 `json:"baseSpeed"`
	SpeedDivisor   float64 `json:"speedDivisor"`
	MinimumSpeed   float64 `json:"minimumSpeed"`
}

type player struct {
	ID                string    `json:"id"`
	SessionID         string    `json:"-"`
	OwnerID           string    `json:"ownerId"`
	Nickname          string    `json:"nickname"`
	CellType          string    `json:"cellType"`
	Ability           string    `json:"abilityName"`
	X                 float64   `json:"x"`
	Y                 float64   `json:"y"`
	Mass              float64   `json:"mass"`
	Radius            float64   `json:"radius"`
	Scale             float64   `json:"scale"`
	Color             string    `json:"color"`
	IsBot             bool      `json:"isBot"`
	Direction         direction `json:"-"`
	CooldownRemaining int64     `json:"cooldownRemaining"`
	EffectRemaining   int64     `json:"effectRemaining"`
	CooldownUntil     time.Time `json:"-"`
	EffectUntil       time.Time `json:"-"`
	CactusUntil       time.Time `json:"-"`
	PortalUntil       time.Time `json:"-"`
	MergeReadyAt      time.Time `json:"-"`
	LastSeen          time.Time `json:"-"`
	NextBotThinkAt    time.Time `json:"-"`
	Conn              *wsConn   `json:"-"`
	IsAbilityActive   bool      `json:"-"` // ✅ 추가: 현재 스킬 버튼 누름 여부
	Energy            float64   `json:"-"` // ✅ 추가: 오버클럭 에너지 (0 ~ 4000)
}

type direction struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type food struct {
	ID     string  `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius"`
	Value  float64 `json:"value"`
	VX     float64 `json:"-"`
	VY     float64 `json:"-"`
}

type cactus struct {
	ID     string  `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Size   float64 `json:"size"`
	Height float64 `json:"height"`
}

type wormhole struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	PairID    string  `json:"pairId"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Radius    float64 `json:"radius"`
	PullRange float64 `json:"pullRange"`
}

type joinRequest struct {
	Nickname string `json:"nickname"`
	CellType string `json:"cellType"`
}

type leaveRequest struct {
	PlayerID  string `json:"playerId"`
	SessionID string `json:"sessionId"`
}

type joinResponse struct {
	PlayerID  string `json:"playerId"`
	SessionID string `json:"sessionId"`
	Nickname  string `json:"nickname"`
	CellType  string `json:"cellType"`
}

type inputMessage struct {
	Type       string    `json:"type"`
	Direction  direction `json:"direction"`
	UseAbility bool      `json:"useAbility"`
	UseSplit   bool      `json:"useSplit"`
}

type snapshotMessage struct {
	Type        string         `json:"type"`
	Players     []*player      `json:"players"`
	Foods       []*food        `json:"foods"`
	Cacti       []*cactus      `json:"cacti"`
	Wormholes   []*wormhole    `json:"wormholes"`
	Leaderboard []ownerSummary `json:"leaderboard"`
}

type ownerSummary struct {
	OwnerID  string  `json:"ownerId"`
	Nickname string  `json:"nickname"`
	Mass     float64 `json:"mass"`
	IsBot    bool    `json:"isBot"`
}

type adminStatusResponse struct {
	HumanPlayers int           `json:"humanPlayers"`
	BotPlayers   int           `json:"botPlayers"`
	TotalPlayers int           `json:"totalPlayers"`
	Config       runtimeConfig `json:"config"`
}

type adminConfigRequest struct {
	MinimumPlayers *int     `json:"minimumPlayers"`
	WormholePairs  *int     `json:"wormholePairs"`
	BaseSpeed      *float64 `json:"baseSpeed"`
	SpeedDivisor   *float64 `json:"speedDivisor"`
	MinimumSpeed   *float64 `json:"minimumSpeed"`
}

type wsConn struct {
	conn net.Conn
	mu   sync.Mutex
}

// ----------------------------------------------------
// 공간 분할(Spatial Partitioning) 헬퍼 구조체 및 함수
// ----------------------------------------------------

type spatialGrid struct {
	players   [][][]*player
	foods     [][][]*food
	cacti     [][][]*cactus
	wormholes [][][]*wormhole
}

func newSpatialGrid() *spatialGrid {
	g := &spatialGrid{
		players:   make([][][]*player, spatialGridCols),
		foods:     make([][][]*food, spatialGridCols),
		cacti:     make([][][]*cactus, spatialGridCols),
		wormholes: make([][][]*wormhole, spatialGridCols),
	}
	for i := 0; i < spatialGridCols; i++ {
		g.players[i] = make([][]*player, spatialGridRows)
		g.foods[i] = make([][]*food, spatialGridRows)
		g.cacti[i] = make([][]*cactus, spatialGridRows)
		g.wormholes[i] = make([][]*wormhole, spatialGridRows)
	}
	return g
}

func getCellIndex(x, y float64) (int, int) {
	cx := int(x / spatialCellSize)
	cy := int(y / spatialCellSize)
	if cx < 0 {
		cx = 0
	}
	if cy < 0 {
		cy = 0
	}
	if cx >= spatialGridCols {
		cx = spatialGridCols - 1
	}
	if cy >= spatialGridRows {
		cy = spatialGridRows - 1
	}
	return cx, cy
}

func main() {
	mathrand.Seed(time.Now().UnixNano())

	state := &gameState{
		players:   make(map[string]*player),
		foods:     make([]*food, 0, foodTarget),
		cacti:     make([]*cactus, 0, cactusTarget),
		wormholes: make([]*wormhole, 0, defaultWormholePairs*2),
		config: runtimeConfig{
			MinimumPlayers: defaultMinimumPlayers,
			WormholePairs:  defaultWormholePairs,
			BaseSpeed:      defaultBaseSpeed,
			SpeedDivisor:   defaultSpeedDivisor,
			MinimumSpeed:   defaultMinimumSpeed,
		},
		spatialCache: newSpatialGrid(),
	}
	state.seedFoods()
	state.seedCacti()
	state.reconcileWormholesLocked()
	state.reconcileBotsLocked()
	go state.runWorld()

	http.HandleFunc("/api/join", state.handleJoin)
	http.HandleFunc("/api/leave", state.handleLeave)
	http.HandleFunc("/api/admin/status", state.handleAdminStatus)
	http.HandleFunc("/api/admin/config", state.handleAdminConfig)
	http.HandleFunc("/ws", state.handleWS)
	http.HandleFunc("/super", serveSuperPage)
	http.HandleFunc("/", serveStatic)

	log.Printf("Go cell server running at http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func (s *gameState) handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req joinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	nickname := sanitizeNickname(req.Nickname)
	cellType := sanitizeCellType(req.CellType)
	playerID := randomID()
	sessionID := randomID()

	p := &player{
		ID:        playerID,
		SessionID: sessionID,
		OwnerID:   playerID,
		Nickname:  nickname,
		CellType:  cellType,
		Ability:   abilityName(cellType),
		X:         400 + mathrand.Float64()*(worldSize-800),
		Y:         400 + mathrand.Float64()*(worldSize-800),
		Mass:      playerStartMass,
		Radius:    massToRadius(playerStartMass),
		Scale:     1,
		Color:     randomColor(),
		LastSeen:  time.Now(),
	}
	p.Energy = 4000
	s.mu.Lock()
	s.players[playerID] = p
	s.reconcileBotsLocked()
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, joinResponse{
		PlayerID:  playerID,
		SessionID: sessionID,
		Nickname:  nickname,
		CellType:  cellType,
	})
}

func (s *gameState) handleLeave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req leaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if p, ok := s.players[req.PlayerID]; ok && !p.IsBot && p.SessionID == req.SessionID {
		if p.Conn != nil {
			p.Conn.close()
		}
		delete(s.players, req.PlayerID)
		s.reconcileBotsLocked()
	}
	s.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

func (s *gameState) handleAdminStatus(w http.ResponseWriter, r *http.Request) {
	if !requireSuperAuth(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	humans := 0
	bots := 0
	for _, p := range s.players {
		if p.IsBot {
			bots++
		} else {
			humans++
		}
	}
	config := s.config
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, adminStatusResponse{
		HumanPlayers: humans,
		BotPlayers:   bots,
		TotalPlayers: humans + bots,
		Config:       config,
	})
}

func (s *gameState) handleAdminConfig(w http.ResponseWriter, r *http.Request) {
	if !requireSuperAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req adminConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if req.MinimumPlayers != nil {
		s.config.MinimumPlayers = int(math.Max(0, float64(*req.MinimumPlayers)))
	}
	if req.WormholePairs != nil {
		s.config.WormholePairs = int(math.Max(0, float64(*req.WormholePairs)))
	}
	if req.BaseSpeed != nil {
		s.config.BaseSpeed = math.Max(50, *req.BaseSpeed)
	}
	if req.SpeedDivisor != nil {
		s.config.SpeedDivisor = math.Max(1, *req.SpeedDivisor)
	}
	if req.MinimumSpeed != nil {
		s.config.MinimumSpeed = math.Max(10, *req.MinimumSpeed)
	}
	s.reconcileWormholesLocked()
	s.reconcileBotsLocked()
	config := s.config
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, config)
}

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
		if msg.Type != "input" {
			continue
		}

		s.mu.Lock()
		if p, ok := s.players[playerID]; ok && p.Conn == ws {
			ownerID := p.OwnerID
			if ownerID == "" {
				ownerID = p.ID
			}
			now := time.Now()
			for _, fragment := range s.ownedPlayersLocked(ownerID) {
				fragment.Direction.X = clamp(msg.Direction.X, -1, 1)
				fragment.Direction.Y = clamp(msg.Direction.Y, -1, 1)
				fragment.LastSeen = now
				fragment.IsAbilityActive = msg.UseAbility
			}
			if msg.UseAbility {
				s.tryUseAbility(p)
			}
			if msg.UseSplit {
				for _, fragment := range s.ownedPlayersLocked(ownerID) {
					s.trySplit(fragment)
				}
			}
		}
		s.mu.Unlock()
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

func (s *gameState) runWorld() {
	ticker := time.NewTicker(time.Second / tickRate)
	defer ticker.Stop()

	for range ticker.C {
		s.updateWorld()
		s.broadcastSnapshot()
	}
}

func (s *gameState) updateWorld() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, f := range s.foods {
		if math.Abs(f.VX) > 0.01 || math.Abs(f.VY) > 0.01 {
			f.X = clamp(f.X+f.VX/tickRate, f.Radius, worldSize-f.Radius)
			f.Y = clamp(f.Y+f.VY/tickRate, f.Radius, worldSize-f.Radius)
			f.VX *= 0.9
			f.VY *= 0.9
		}
	}

	for id, p := range s.players {
		if !p.IsBot && now.Sub(p.LastSeen) > playerTimeout {
			if p.Conn != nil {
				p.Conn.close()
			}
			delete(s.players, id)
			continue
		}

		if p.IsBot {
			s.updateBotLocked(p, now)
		}

		speed := s.movementSpeed(p.Mass)
		switch {
		case p.CellType == "giant" && now.Before(p.EffectUntil):
			speed *= 0.68
			p.Scale = 1.9
		case p.CellType == "classic": // ✅ 오버클럭 에너지 제어 로직
			p.Scale = 1
			depleteRate := 4000.0 / (1.5 * tickRate)  // 1.5초 만에 방전
			rechargeRate := 4000.0 / (4.0 * tickRate) // 4초 만에 완충

			if p.IsAbilityActive {
				if p.Energy > 0 {
					p.Energy -= depleteRate
					speed *= 1.9
					p.EffectUntil = now.Add(150 * time.Millisecond) // UI 점등용
				}
				if p.Energy < 0 {
					p.Energy = 0
				}
			} else {
				p.Energy += rechargeRate
				if p.Energy > 4000 {
					p.Energy = 4000
				}
			}
		default:
			p.Scale = 1
		}
		effectiveRadius := currentRadius(p)
		p.X = clamp(p.X+p.Direction.X*speed/tickRate, effectiveRadius, worldSize-effectiveRadius)
		p.Y = clamp(p.Y+p.Direction.Y*speed/tickRate, effectiveRadius, worldSize-effectiveRadius)
		if p.CellType == "magnet" && now.Before(p.EffectUntil) {
			s.pullNearbyFoodLocked(p, 220)
		}
		s.applyWormholeForceLocked(p, now)
		s.resolveCactusHitLocked(p, now)

		for i := len(s.foods) - 1; i >= 0; i-- {
			f := s.foods[i]
			if distance(p.X, p.Y, f.X, f.Y) < effectiveRadius+f.Radius {
				p.Mass += f.Value
				p.Radius = massToRadius(p.Mass)

				// [Memory Leak Fix]: 요소를 삭제할 때 끝부분의 포인터를 nil로 명시적 해제
				s.foods[i] = s.foods[len(s.foods)-1]
				s.foods[len(s.foods)-1] = nil
				s.foods = s.foods[:len(s.foods)-1]
			}
		}
	}

	s.reconcileBotsLocked()
	s.applyOwnedCohesionLocked(now)
	s.resolvePlayerEating()
	s.resolveOwnedMergesLocked(now)
	s.topUpFoods()
}

func (s *gameState) resolvePlayerEating() {
	players := make([]*player, 0, len(s.players))
	for _, p := range s.players {
		players = append(players, p)
	}

	now := time.Now() // 현재 시간 한 번만 호출

	for i := 0; i < len(players); i++ {
		for j := i + 1; j < len(players); j++ {
			a := players[i]
			b := players[j]

			// 이미 먹혔거나 제거된 플레이어인지 확인
			if _, ok := s.players[a.ID]; !ok {
				continue
			}
			if _, ok := s.players[b.ID]; !ok {
				continue
			}

			// 같은 소유주의 조각끼리는 먹지 않음
			if a.OwnerID != "" && a.OwnerID == b.OwnerID {
				continue
			}

			gap := distance(a.X, a.Y, b.X, b.Y)

			if canEatPlayer(a, b, gap) {
				// A가 B를 포식
				a.Mass += b.Mass * 0.85
				a.Radius = massToRadius(a.Mass)
				s.handleConsumedPlayerLocked(b)
			} else if canEatPlayer(b, a, gap) {
				// B가 A를 포식
				b.Mass += a.Mass * 0.85
				b.Radius = massToRadius(b.Mass)
				s.handleConsumedPlayerLocked(a)
			} else {
				// 포식 관계가 성립하지 않을 때 (비슷한 크기이거나, 무적 상태 등)

				// 추가된 조건: 둘 중 하나라도 방어 관련 스킬(실드, 자이언트)이 켜져 있을 때만 밀어내기
				aHasShield := a.CellType == "shield" && now.Before(a.EffectUntil)
				bHasShield := b.CellType == "shield" && now.Before(b.EffectUntil)
				aIsGiant := a.CellType == "giant" && now.Before(a.EffectUntil)
				bIsGiant := b.CellType == "giant" && now.Before(b.EffectUntil)

				if aHasShield || bHasShield || aIsGiant || bIsGiant {
					radiusA := currentRadius(a)
					radiusB := currentRadius(b)
					minGap := radiusA + radiusB

					if gap < minGap && gap > 0 {
						overlap := minGap - gap

						// 질량 합산 및 밀려나는 비율 계산 (가벼울수록 더 많이 밀려남)
						totalMass := a.Mass + b.Mass
						if totalMass <= 0 {
							totalMass = 1
						}

						pushA := (b.Mass / totalMass) * overlap * 0.5
						pushB := (a.Mass / totalMass) * overlap * 0.5

						dirX := (a.X - b.X) / gap
						dirY := (a.Y - b.Y) / gap

						// A 이동 및 월드 경계 클램핑
						a.X = clamp(a.X+dirX*pushA, radiusA, worldSize-radiusA)
						a.Y = clamp(a.Y+dirY*pushA, radiusA, worldSize-radiusA)

						// B 이동 및 월드 경계 클램핑
						b.X = clamp(b.X-dirX*pushB, radiusB, worldSize-radiusB)
						b.Y = clamp(b.Y-dirY*pushB, radiusB, worldSize-radiusB)
					}
				}
			}
		}
	}
}

// ----------------------------------------------------
// 최적화된 브로드캐스트 함수 (Spatial Partitioning)
// ----------------------------------------------------
func (s *gameState) broadcastSnapshot() {
	s.mu.RLock()

	// 1. O(N²) 방지를 위한 공간 분할(Grid) 캐시 구축
	grid := s.spatialCache
	for cx := 0; cx < spatialGridCols; cx++ {
		for cy := 0; cy < spatialGridRows; cy++ {
			grid.players[cx][cy] = grid.players[cx][cy][:0]
			grid.foods[cx][cy] = grid.foods[cx][cy][:0]
			grid.cacti[cx][cy] = grid.cacti[cx][cy][:0]
			grid.wormholes[cx][cy] = grid.wormholes[cx][cy][:0]
		}
	}
	for _, p := range s.players {
		cx, cy := getCellIndex(p.X, p.Y)
		grid.players[cx][cy] = append(grid.players[cx][cy], p)
	}
	for _, f := range s.foods {
		cx, cy := getCellIndex(f.X, f.Y)
		grid.foods[cx][cy] = append(grid.foods[cx][cy], f)
	}
	for _, c := range s.cacti {
		cx, cy := getCellIndex(c.X, c.Y)
		grid.cacti[cx][cy] = append(grid.cacti[cx][cy], c)
	}
	for _, w := range s.wormholes {
		cx, cy := getCellIndex(w.X, w.Y)
		grid.wormholes[cx][cy] = append(grid.wormholes[cx][cy], w)
	}

	leaderboard := buildOwnerLeaderboard(s.players)

	// 2. 소유자(Owner)별 중심점 사전 계산 (루프 내 중복 연산 방지)
	type centerMass struct {
		x, y, totalMass float64
	}
	centers := make(map[string]*centerMass)
	for _, p := range s.players {
		ownerID := p.OwnerID
		if ownerID == "" {
			ownerID = p.ID
		}
		if centers[ownerID] == nil {
			centers[ownerID] = &centerMass{}
		}
		centers[ownerID].x += p.X * p.Mass
		centers[ownerID].y += p.Y * p.Mass
		centers[ownerID].totalMass += p.Mass
	}
	for _, c := range centers {
		if c.totalMass > 0 {
			c.x /= c.totalMass
			c.y /= c.totalMass
		}
	}

	type snapshotTarget struct {
		conn    *wsConn
		message snapshotMessage
	}
	var targets []snapshotTarget

	// 3. 접속 중인 유저별로 타겟 페이로드 생성
	for _, viewer := range s.players {
		if viewer.Conn == nil {
			continue
		}

		viewerOwnerID := viewer.OwnerID
		if viewerOwnerID == "" {
			viewerOwnerID = viewer.ID
		}

		// 캐싱된 중심점 가져오기
		var centerX, centerY float64
		if c, ok := centers[viewerOwnerID]; ok && c.totalMass > 0 {
			centerX = c.x
			centerY = c.y
		} else {
			centerX = viewer.X
			centerY = viewer.Y
		}

		// 조회할 Grid의 최소/최대 인덱스 도출
		minCx, minCy := getCellIndex(centerX-objectCullRange, centerY-objectCullRange)
		maxCx, maxCy := getCellIndex(centerX+objectCullRange, centerY+objectCullRange)

		var players []*player
		var foods []*food
		var cacti []*cactus
		var wormholes []*wormhole

		// 4. 전체 순회 대신 인접 격자(Grid)만 순회하여 성능 극대화
		for cx := minCx; cx <= maxCx; cx++ {
			for cy := minCy; cy <= maxCy; cy++ {
				for _, p := range grid.players[cx][cy] {
					targetOwnerID := p.OwnerID
					if targetOwnerID == "" {
						targetOwnerID = p.ID
					}
					if targetOwnerID != viewerOwnerID && !isWithinCullRange(centerX, centerY, p.X, p.Y, playerCullRange+currentRadius(p)) {
						continue
					}
					players = append(players, clonePlayer(p))
				}
				for _, f := range grid.foods[cx][cy] {
					if !isWithinCullRange(centerX, centerY, f.X, f.Y, foodCullRange+f.Radius) {
						continue
					}
					copyFood := *f
					foods = append(foods, &copyFood)
				}
				for _, c := range grid.cacti[cx][cy] {
					if !isWithinCullRange(centerX, centerY, c.X, c.Y, objectCullRange+c.Size*1.3) {
						continue
					}
					copyCactus := *c
					cacti = append(cacti, &copyCactus)
				}
				for _, w := range grid.wormholes[cx][cy] {
					if !isWithinCullRange(centerX, centerY, w.X, w.Y, objectCullRange+w.PullRange) {
						continue
					}
					copyHole := *w
					wormholes = append(wormholes, &copyHole)
				}
			}
		}

		targets = append(targets, snapshotTarget{
			conn: viewer.Conn,
			message: snapshotMessage{
				Type:        "snapshot",
				Players:     players,
				Foods:       foods,
				Cacti:       cacti,
				Wormholes:   wormholes,
				Leaderboard: leaderboard,
			},
		})
	}
	s.mu.RUnlock() // JSON 생성 후 ReadLock 조기 해제

	// 5. 실제 웹소켓 전송 (락 해제 상태에서 수행)
	for _, target := range targets {
		payload, err := json.Marshal(target.message)
		if err != nil {
			continue
		}
		if err := target.conn.writeText(payload); err != nil {
			target.conn.close()
		}
	}
}

func (s *gameState) sendSnapshotTo(playerID string, conn *wsConn) error {
	payload, err := s.buildSnapshotPayload(playerID)
	if err != nil {
		return err
	}
	return conn.writeText(payload)
}

// 최초 1회 Join 시 호출되는 단일 페이로드 생성 로직 (기존 로직 유지)
func (s *gameState) buildSnapshotPayload(playerID string) ([]byte, error) {
	s.mu.RLock()
	viewer, ok := s.players[playerID]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("viewer not found")
	}
	viewerOwnerID := viewer.OwnerID
	if viewerOwnerID == "" {
		viewerOwnerID = viewer.ID
	}
	centerX, centerY := s.ownerCenterLocked(viewerOwnerID)

	players := make([]*player, 0, len(s.players))
	for _, p := range s.players {
		targetOwnerID := p.OwnerID
		if targetOwnerID == "" {
			targetOwnerID = p.ID
		}
		if targetOwnerID != viewerOwnerID && !isWithinCullRange(centerX, centerY, p.X, p.Y, playerCullRange+currentRadius(p)) {
			continue
		}
		players = append(players, clonePlayer(p))
	}

	foods := make([]*food, 0, len(s.foods))
	for _, f := range s.foods {
		if !isWithinCullRange(centerX, centerY, f.X, f.Y, foodCullRange+f.Radius) {
			continue
		}
		copyFood := *f
		foods = append(foods, &copyFood)
	}

	cacti := make([]*cactus, 0, len(s.cacti))
	for _, c := range s.cacti {
		if !isWithinCullRange(centerX, centerY, c.X, c.Y, objectCullRange+c.Size*1.3) {
			continue
		}
		copyCactus := *c
		cacti = append(cacti, &copyCactus)
	}

	wormholes := make([]*wormhole, 0, len(s.wormholes))
	for _, hole := range s.wormholes {
		if !isWithinCullRange(centerX, centerY, hole.X, hole.Y, objectCullRange+hole.PullRange) {
			continue
		}
		copyHole := *hole
		wormholes = append(wormholes, &copyHole)
	}
	leaderboard := buildOwnerLeaderboard(s.players)
	s.mu.RUnlock()

	return json.Marshal(snapshotMessage{
		Type:        "snapshot",
		Players:     players,
		Foods:       foods,
		Cacti:       cacti,
		Wormholes:   wormholes,
		Leaderboard: leaderboard,
	})
}

func (s *gameState) seedFoods() {
	for len(s.foods) < foodTarget {
		s.foods = append(s.foods, createFood())
	}
}

func (s *gameState) seedCacti() {
	for len(s.cacti) < cactusTarget {
		s.cacti = append(s.cacti, createCactus())
	}
}

func (s *gameState) reconcileWormholesLocked() {
	targetPairs := s.config.WormholePairs
	if targetPairs < 0 {
		targetPairs = 0
	}

	targetCount := targetPairs * 2
	for len(s.wormholes) > targetCount {
		// [Memory Leak Fix]
		s.wormholes[len(s.wormholes)-1] = nil
		s.wormholes[len(s.wormholes)-2] = nil
		s.wormholes = s.wormholes[:len(s.wormholes)-2]
	}

	for len(s.wormholes) < targetCount {
		pairID := randomID()
		entry := createWormhole("blackhole", pairID)
		exit := createWormhole("whitehole", pairID)
		for distance(entry.X, entry.Y, exit.X, exit.Y) < 700 {
			exit = createWormhole("whitehole", pairID)
		}
		s.wormholes = append(s.wormholes, entry, exit)
	}
}

func (s *gameState) topUpFoods() {
	for len(s.foods) < foodTarget {
		s.foods = append(s.foods, createFood())
	}
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	root, err := appBaseDir()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	requestPath := r.URL.Path
	if requestPath == "/" {
		requestPath = "/index.html"
	}

	relativePath := strings.TrimPrefix(requestPath, "/")
	cleanPath := filepath.Clean(relativePath)
	fullPath := filepath.Join(root, cleanPath)
	if !strings.HasPrefix(fullPath, root) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ext := filepath.Ext(fullPath)
	if contentType, ok := mimeTypes[ext]; ok {
		w.Header().Set("Content-Type", contentType)
	}
	_, _ = w.Write(data)
}

func serveSuperPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/super" {
		http.NotFound(w, r)
		return
	}
	if !requireSuperAuth(w, r) {
		return
	}
	root, err := appBaseDir()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, r, filepath.Join(root, "super.html"))
}

func appBaseDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exePath), nil
}

func requireSuperAuth(w http.ResponseWriter, r *http.Request) bool {
	expectedUser := "sdlckdrl"
	expectedPassword := os.Getenv("SUPER_PASSWORD")
	if expectedPassword == "" {
		expectedPassword = "1729ck!@"
	}

	expectedToken := superAuthToken(expectedUser, expectedPassword)
	if cookie, err := r.Cookie("super_auth"); err == nil && cookie.Value == expectedToken {
		return true
	}

	username, password, ok := r.BasicAuth()
	if ok && username == expectedUser && password == expectedPassword {
		http.SetCookie(w, &http.Cookie{
			Name:     "super_auth",
			Value:    expectedToken,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   86400,
		})
		return true
	}

	w.Header().Set("WWW-Authenticate", `Basic realm="Super Admin"`)
	http.Error(w, "unauthorized", http.StatusUnauthorized)
	return false
}

func superAuthToken(username, password string) string {
	hash := sha1.Sum([]byte(username + ":" + password + ":super"))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func createFood() *food {
	return &food{
		ID:     randomID(),
		X:      30 + mathrand.Float64()*(worldSize-60),
		Y:      30 + mathrand.Float64()*(worldSize-60),
		Radius: 6 + mathrand.Float64()*3,
		Value:  2 + mathrand.Float64()*2,
	}
}

func createCactus() *cactus {
	size := 20 + mathrand.Float64()*18
	return &cactus{
		ID:     randomID(),
		X:      120 + mathrand.Float64()*(worldSize-240),
		Y:      120 + mathrand.Float64()*(worldSize-240),
		Size:   size,
		Height: size * (1.4 + mathrand.Float64()*0.6),
	}
}

func createWormhole(kind, pairID string) *wormhole {
	radius := 34 + mathrand.Float64()*10
	return &wormhole{
		ID:        randomID(),
		Kind:      kind,
		PairID:    pairID,
		X:         240 + mathrand.Float64()*(worldSize-480),
		Y:         240 + mathrand.Float64()*(worldSize-480),
		Radius:    radius,
		PullRange: radius * 4.6,
	}
}

func respawnPlayer(p *player) {
	ownerID := p.OwnerID
	if ownerID == "" {
		ownerID = p.ID
	}
	p.Energy = 4000
	p.Mass = playerStartMass
	p.Radius = massToRadius(playerStartMass)
	p.X = 400 + mathrand.Float64()*(worldSize-800)
	p.Y = 400 + mathrand.Float64()*(worldSize-800)
	p.Scale = 1
	p.OwnerID = ownerID
	p.Direction = direction{}
	p.CooldownUntil = time.Time{}
	p.EffectUntil = time.Time{}
	p.CactusUntil = time.Time{}
	p.PortalUntil = time.Time{}
	p.MergeReadyAt = time.Time{}
}

func clonePlayer(p *player) *player {
	now := time.Now()

	// 1. 밀리초(ms) 단위로 변환한 값을 먼저 변수에 담습니다.
	cooldownRemainingMs := int64(maxDuration(0, p.CooldownUntil.Sub(now)) / time.Millisecond)
	effectRemainingMs := int64(maxDuration(0, p.EffectUntil.Sub(now)) / time.Millisecond)

	// ✅ 2. 오버클럭(classic)일 경우, 에너지를 쿨타임 수치로 변환합니다.
	// 에너지가 4000(완충)이면 쿨타임 0(준비 완료).
	// 에너지가 0(방전)이면 쿨타임 4000(재충전 중)으로 프론트엔드에 전달합니다.
	if p.CellType == "classic" {
		cooldownRemainingMs = int64(4000 - p.Energy)
		if cooldownRemainingMs < 0 {
			cooldownRemainingMs = 0
		}
	}

	return &player{
		ID:                p.ID,
		OwnerID:           p.OwnerID, // 이전에 GetOwnerID()로 통합하셨다면 p.GetOwnerID() 사용
		Nickname:          p.Nickname,
		CellType:          p.CellType,
		Ability:           p.Ability,
		X:                 p.X,
		Y:                 p.Y,
		Mass:              p.Mass,
		Radius:            p.Radius,
		Scale:             p.Scale,
		Color:             p.Color,
		IsBot:             p.IsBot,
		CooldownRemaining: cooldownRemainingMs, // ✅ 수정된 변수 적용
		EffectRemaining:   effectRemainingMs,   // ✅ 수정된 변수 적용
	}
}

func (s *gameState) ownedPlayersLocked(ownerID string) []*player {
	fragments := make([]*player, 0)
	for _, p := range s.players {
		currentOwner := p.OwnerID
		if currentOwner == "" {
			currentOwner = p.ID
		}
		if currentOwner == ownerID {
			fragments = append(fragments, p)
		}
	}
	return fragments
}

func (s *gameState) ownerCenterLocked(ownerID string) (float64, float64) {
	fragments := s.ownedPlayersLocked(ownerID)
	if len(fragments) == 0 {
		return worldSize * 0.5, worldSize * 0.5
	}

	var centerX float64
	var centerY float64
	var totalMass float64
	for _, fragment := range fragments {
		centerX += fragment.X * fragment.Mass
		centerY += fragment.Y * fragment.Mass
		totalMass += fragment.Mass
	}
	if totalMass <= 0 {
		return fragments[0].X, fragments[0].Y
	}
	return centerX / totalMass, centerY / totalMass
}

func (s *gameState) handleConsumedPlayerLocked(victim *player) {
	ownerID := victim.OwnerID
	if ownerID == "" {
		ownerID = victim.ID
	}
	fragments := s.ownedPlayersLocked(ownerID)
	if len(fragments) <= 1 {
		respawnPlayer(victim)
		return
	}

	if victim.ID != ownerID {
		delete(s.players, victim.ID)
		return
	}

	successor := largestOwnedFragmentExcluding(fragments, victim.ID)
	if successor == nil {
		respawnPlayer(victim)
		return
	}

	victim.X = successor.X
	victim.Y = successor.Y
	victim.Mass = successor.Mass
	victim.Radius = successor.Radius
	victim.Scale = successor.Scale
	victim.Direction = successor.Direction
	victim.CooldownUntil = successor.CooldownUntil
	victim.EffectUntil = successor.EffectUntil
	victim.CactusUntil = successor.CactusUntil
	victim.PortalUntil = successor.PortalUntil
	victim.MergeReadyAt = successor.MergeReadyAt
	delete(s.players, successor.ID)
}

func largestOwnedFragmentExcluding(fragments []*player, excludedID string) *player {
	var best *player
	for _, fragment := range fragments {
		if fragment.ID == excludedID {
			continue
		}
		if best == nil || fragment.Mass > best.Mass {
			best = fragment
		}
	}
	return best
}

func buildOwnerLeaderboard(players map[string]*player) []ownerSummary {
	totals := make(map[string]*ownerSummary)
	maxMass := make(map[string]float64)

	for _, p := range players {
		ownerID := p.OwnerID
		if ownerID == "" {
			ownerID = p.ID
		}
		entry, exists := totals[ownerID]
		if !exists {
			entry = &ownerSummary{
				OwnerID:  ownerID,
				Nickname: p.Nickname,
				Mass:     0,
				IsBot:    p.IsBot,
			}
			totals[ownerID] = entry
		}
		entry.Mass += p.Mass
		if p.Mass >= maxMass[ownerID] {
			maxMass[ownerID] = p.Mass
			entry.Nickname = p.Nickname
		}
	}

	out := make([]ownerSummary, 0, len(totals))
	for _, entry := range totals {
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool {
		// ✅ 추가된 부분: 질량이 소수점까지 완전히 똑같을 경우 닉네임/ID 순으로 고정시켜 깜빡임 방지
		if out[i].Mass == out[j].Mass {
			return out[i].OwnerID < out[j].OwnerID
		}
		// 기존 정렬 조건
		return out[i].Mass > out[j].Mass
	})
	return out
}

func (s *gameState) resolveOwnedMergesLocked(now time.Time) {
	owners := make(map[string]struct{})
	for _, p := range s.players {
		ownerID := p.OwnerID
		if ownerID == "" {
			ownerID = p.ID
		}
		owners[ownerID] = struct{}{}
	}

	for ownerID := range owners {
		fragments := s.ownedPlayersLocked(ownerID)
		if len(fragments) < 2 {
			continue
		}
		merged := true
		for merged {
			merged = false
			for i := 0; i < len(fragments); i += 1 {
				a := fragments[i]
				if _, ok := s.players[a.ID]; !ok {
					continue
				}
				if now.Before(a.MergeReadyAt) {
					continue
				}
				for j := i + 1; j < len(fragments); j += 1 {
					b := fragments[j]
					if _, ok := s.players[b.ID]; !ok {
						continue
					}
					if now.Before(b.MergeReadyAt) {
						continue
					}
					if distance(a.X, a.Y, b.X, b.Y) > (currentRadius(a)+currentRadius(b))*0.62 {
						continue
					}
					s.mergeOwnedPairLocked(ownerID, a, b)
					fragments = s.ownedPlayersLocked(ownerID)
					merged = true
					break
				}
				if merged {
					break
				}
			}
		}
	}
}

func (s *gameState) applyOwnedCohesionLocked(now time.Time) {
	owners := make(map[string][]*player)
	for _, p := range s.players {
		ownerID := p.OwnerID
		if ownerID == "" {
			ownerID = p.ID
		}
		owners[ownerID] = append(owners[ownerID], p)
	}

	for _, fragments := range owners {
		if len(fragments) < 2 {
			continue
		}

		var centerX float64
		var centerY float64
		var totalMass float64
		for _, fragment := range fragments {
			if now.Before(fragment.MergeReadyAt) {
				continue
			}
			centerX += fragment.X * fragment.Mass
			centerY += fragment.Y * fragment.Mass
			totalMass += fragment.Mass
		}
		if totalMass <= 0 {
			continue
		}

		centerX /= totalMass
		centerY /= totalMass

		for _, fragment := range fragments {
			if now.Before(fragment.MergeReadyAt) {
				continue
			}

			distToCenter := distance(fragment.X, fragment.Y, centerX, centerY)
			if distToCenter > 1 {
				dirX := (centerX - fragment.X) / distToCenter
				dirY := (centerY - fragment.Y) / distToCenter
				movementIntent := math.Hypot(fragment.Direction.X, fragment.Direction.Y)
				idleBoost := 1.0
				if movementIntent < 0.18 {
					idleBoost = 2.25
				} else if movementIntent < 0.45 {
					idleBoost = 1.45
				}
				pull := math.Min(54, distToCenter*0.14) * idleBoost / tickRate
				radius := currentRadius(fragment)
				fragment.X = clamp(fragment.X+dirX*pull, radius, worldSize-radius)
				fragment.Y = clamp(fragment.Y+dirY*pull, radius, worldSize-radius)
			}
		}
	}
}

func (s *gameState) mergeOwnedPairLocked(ownerID string, a, b *player) {
	target := a
	source := b
	if b.ID == ownerID {
		target = b
		source = a
	} else if a.ID != ownerID && b.Mass > a.Mass {
		target = b
		source = a
	}

	target.X = (target.X*target.Mass + source.X*source.Mass) / math.Max(1, target.Mass+source.Mass)
	target.Y = (target.Y*target.Mass + source.Y*source.Mass) / math.Max(1, target.Mass+source.Mass)
	target.Mass += source.Mass
	target.Radius = massToRadius(target.Mass)
	target.MergeReadyAt = time.Time{}
	delete(s.players, source.ID)
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
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("connection closed")
	}

	frame := bytes.NewBuffer(nil)
	frame.WriteByte(0x81)

	length := len(payload)
	switch {
	case length < 126:
		frame.WriteByte(byte(length))
	case length <= math.MaxUint16:
		frame.WriteByte(126)
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(length))
		frame.Write(buf)
	default:
		frame.WriteByte(127)
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(length))
		frame.Write(buf)
	}

	frame.Write(payload)
	_, err := c.conn.Write(frame.Bytes())
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

func sanitizeNickname(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > 16 {
		trimmed = trimmed[:16]
	}
	if trimmed == "" {
		return "Cell"
	}
	return trimmed
}

func sanitizeCellType(value string) string {
	switch value {
	case "blink", "giant", "shield", "magnet", "divider":
		return value
	default:
		return "classic"
	}
}

func abilityName(cellType string) string {
	switch cellType {
	case "classic":
		return "코어 가속"
	case "blink":
		return "순간이동"
	case "giant":
		return "거대화"
	case "shield":
		return "보호막"
	case "magnet":
		return "흡착"
	case "divider":
		return "세포 분열"
	default:
		return "질주"
	}
}

func randomColor() string {
	colors := []string{"#60b9ff", "#8affcf", "#ffcf70", "#ff8b9d", "#c1a6ff"}
	return colors[mathrand.Intn(len(colors))]
}

func massToRadius(mass float64) float64 {
	return 12 + math.Sqrt(mass)*2.4
}

func (s *gameState) movementSpeed(mass float64) float64 {
	return math.Max(s.config.MinimumSpeed, s.config.BaseSpeed/math.Max(1, math.Sqrt(mass)/s.config.SpeedDivisor))
}

func distance(ax, ay, bx, by float64) float64 {
	return math.Hypot(ax-bx, ay-by)
}

func isWithinCullRange(viewerX, viewerY, targetX, targetY, cullRange float64) bool {
	return math.Abs(viewerX-targetX) <= cullRange && math.Abs(viewerY-targetY) <= cullRange
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func (s *gameState) tryUseAbility(p *player) {
	now := time.Now()
	if now.Before(p.CooldownUntil) {
		return
	}

	switch p.CellType {
	case "blink":
		blinkDistance := 180.0
		length := math.Hypot(p.Direction.X, p.Direction.Y)
		if length < 0.1 {
			return
		}
		p.X = clamp(p.X+(p.Direction.X/length)*blinkDistance, p.Radius, worldSize-p.Radius)
		p.Y = clamp(p.Y+(p.Direction.Y/length)*blinkDistance, p.Radius, worldSize-p.Radius)
		p.CooldownUntil = now.Add(6 * time.Second)
	case "giant":
		p.EffectUntil = now.Add(5 * time.Second)
		p.CooldownUntil = now.Add(10 * time.Second)
	case "shield":
		p.EffectUntil = now.Add(3 * time.Second)
		p.CooldownUntil = now.Add(12 * time.Second)
	case "magnet":
		p.EffectUntil = now.Add(4 * time.Second)
		p.CooldownUntil = now.Add(9 * time.Second)
	case "divider":
		s.tryDividerAbilityLocked(p, now)
	default:
		p.CooldownUntil = now.Add(2 * time.Second)
	}
}

func (s *gameState) trySplit(p *player) {
	if p.Mass < 55 {
		return
	}

	dir := normalizeDirection(p.Direction.X, p.Direction.Y)
	if dir.X == 0 && dir.Y == 0 {
		dir = direction{X: 1}
	}

	splitMass := math.Max(10, p.Mass*0.18)
	p.Mass -= splitMass
	p.Radius = massToRadius(p.Mass)

	chunkRadius := math.Max(8, math.Sqrt(splitMass)*1.2)
	chunk := &food{
		ID:     randomID(),
		X:      clamp(p.X+dir.X*(p.Radius+chunkRadius+14), chunkRadius, worldSize-chunkRadius),
		Y:      clamp(p.Y+dir.Y*(p.Radius+chunkRadius+14), chunkRadius, worldSize-chunkRadius),
		Radius: chunkRadius,
		Value:  splitMass * 0.9,
		VX:     dir.X * 380,
		VY:     dir.Y * 380,
	}
	s.foods = append(s.foods, chunk)
}

func (s *gameState) tryDividerAbilityLocked(p *player, now time.Time) {
	ownerID := p.OwnerID
	if ownerID == "" {
		ownerID = p.ID
	}
	fragments := s.ownedPlayersLocked(ownerID)
	if len(fragments) >= dividerMaxFragments {
		return
	}

	type splitPlan struct {
		parent *player
		child  *player
	}
	plans := make([]splitPlan, 0, len(fragments))
	remainingSlots := dividerMaxFragments - len(fragments)

	for _, fragment := range fragments {
		if remainingSlots <= 0 {
			break
		}
		if fragment.Mass < dividerMinSplitMass {
			continue
		}

		dir := normalizeDirection(fragment.Direction.X, fragment.Direction.Y)
		if dir.X == 0 && dir.Y == 0 {
			dir = direction{X: 1}
		}
		childMass := fragment.Mass / 2
		fragment.Mass = childMass
		fragment.Radius = massToRadius(fragment.Mass)
		fragment.MergeReadyAt = now.Add(dividerMergeDelay)
		fragment.CooldownUntil = now.Add(dividerSplitCooldown)

		child := &player{
			ID:             randomID(),
			SessionID:      "",
			OwnerID:        ownerID,
			Nickname:       fragment.Nickname,
			CellType:       fragment.CellType,
			Ability:        fragment.Ability,
			X:              clamp(fragment.X-dir.X*(fragment.Radius+28), fragment.Radius, worldSize-fragment.Radius),
			Y:              clamp(fragment.Y-dir.Y*(fragment.Radius+28), fragment.Radius, worldSize-fragment.Radius),
			Mass:           childMass,
			Radius:         massToRadius(childMass),
			Scale:          1,
			Color:          fragment.Color,
			IsBot:          fragment.IsBot,
			Direction:      fragment.Direction,
			CooldownUntil:  now.Add(dividerSplitCooldown),
			MergeReadyAt:   now.Add(dividerMergeDelay),
			LastSeen:       fragment.LastSeen,
			NextBotThinkAt: fragment.NextBotThinkAt,
			Energy:         fragment.Energy,
		}

		fragment.X = clamp(fragment.X+dir.X*(fragment.Radius+28), fragment.Radius, worldSize-fragment.Radius)
		fragment.Y = clamp(fragment.Y+dir.Y*(fragment.Radius+28), fragment.Radius, worldSize-fragment.Radius)
		plans = append(plans, splitPlan{parent: fragment, child: child})
		remainingSlots--
	}

	for _, plan := range plans {
		s.players[plan.child.ID] = plan.child
	}
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func currentRadius(p *player) float64 {
	return p.Radius * math.Max(1, p.Scale)
}

func canEatPlayer(attacker, defender *player, gap float64) bool {
	if gap >= currentRadius(attacker) {
		return false
	}

	if attacker.Mass <= defender.Mass*1.1 {
		return false
	}

	if defender.CellType == "giant" && time.Now().Before(defender.EffectUntil) {
		requiredMass := defender.Mass * 1.1 * math.Max(1, defender.Scale)
		if attacker.Mass < requiredMass {
			return false
		}
	}

	if defender.CellType == "shield" && time.Now().Before(defender.EffectUntil) {
		return false
	}

	if attacker.CellType == "giant" && time.Now().Before(attacker.EffectUntil) {
		return false
	}

	return true
}

func normalizeDirection(x, y float64) direction {
	length := math.Hypot(x, y)
	if length < 0.0001 {
		return direction{}
	}
	return direction{X: x / length, Y: y / length}
}

func (s *gameState) pullNearbyFoodLocked(p *player, radius float64) {
	for _, f := range s.foods {
		dist := distance(p.X, p.Y, f.X, f.Y)
		if dist < radius && dist > 0.001 {
			dirX := (p.X - f.X) / dist
			dirY := (p.Y - f.Y) / dist
			f.VX += dirX * 22
			f.VY += dirY * 22
		}
	}
}

func (s *gameState) applyWormholeForceLocked(p *player, now time.Time) {
	for _, hole := range s.wormholes {
		if hole.Kind != "blackhole" {
			continue
		}

		dist := distance(p.X, p.Y, hole.X, hole.Y)
		if dist > hole.PullRange {
			continue
		}

		if dist > 0.001 {
			pull := clamp(1-(dist/hole.PullRange), 0, 1)
			dirX := (hole.X - p.X) / dist
			dirY := (hole.Y - p.Y) / dist
			p.X = clamp(p.X+dirX*(22+pull*34)/tickRate, currentRadius(p), worldSize-currentRadius(p))
			p.Y = clamp(p.Y+dirY*(22+pull*34)/tickRate, currentRadius(p), worldSize-currentRadius(p))
		}

		if now.Before(p.PortalUntil) || dist > hole.Radius*0.72 {
			continue
		}

		exit := s.pairedWormholeLocked(hole)
		if exit == nil {
			continue
		}

		offset := normalizeDirection(p.Direction.X, p.Direction.Y)
		if offset.X == 0 && offset.Y == 0 {
			offset = normalizeDirection(exit.X-hole.X, exit.Y-hole.Y)
		}
		if offset.X == 0 && offset.Y == 0 {
			offset = direction{X: 1}
		}

		spawnDistance := exit.Radius + currentRadius(p) + 24
		p.X = clamp(exit.X+offset.X*spawnDistance, currentRadius(p), worldSize-currentRadius(p))
		p.Y = clamp(exit.Y+offset.Y*spawnDistance, currentRadius(p), worldSize-currentRadius(p))
		p.PortalUntil = now.Add(1500 * time.Millisecond)
		return
	}
}

func (s *gameState) pairedWormholeLocked(entry *wormhole) *wormhole {
	for _, hole := range s.wormholes {
		if hole.PairID == entry.PairID && hole.ID != entry.ID && hole.Kind == "whitehole" {
			return hole
		}
	}
	return nil
}

func (s *gameState) resolveCactusHitLocked(p *player, now time.Time) {
	if now.Before(p.CactusUntil) {
		return
	}

	for _, c := range s.cacti {
		cactusRadius := c.Size * 1.18
		dist := distance(p.X, p.Y, c.X, c.Y)
		if dist >= currentRadius(p)+cactusRadius {
			continue
		}

		dir := normalizeDirection(p.X-c.X, p.Y-c.Y)
		if dir.X == 0 && dir.Y == 0 {
			dir = direction{X: 1}
		}

		p.CactusUntil = now.Add(1500 * time.Millisecond)

		if p.Mass >= 120 {
			s.forceSplitFromCactusLocked(p, dir)
		} else {
			escape := currentRadius(p) + cactusRadius + 10
			p.X = clamp(c.X+dir.X*escape, currentRadius(p), worldSize-currentRadius(p))
			p.Y = clamp(c.Y+dir.Y*escape, currentRadius(p), worldSize-currentRadius(p))
		}
		return
	}
}

func (s *gameState) forceSplitFromCactusLocked(p *player, dir direction) {
	loss := math.Min(p.Mass*0.48, 520)
	remainingMass := math.Max(playerStartMass, p.Mass-loss)
	splitMass := p.Mass - remainingMass
	p.Mass = remainingMass
	p.Radius = massToRadius(p.Mass)

	chunks := 6
	perChunk := splitMass * 0.88 / float64(chunks)
	baseAngle := math.Atan2(dir.Y, dir.X)
	for i := 0; i < chunks; i += 1 {
		angle := baseAngle + (float64(i)-float64(chunks-1)/2)*0.34
		chunkRadius := math.Max(11, math.Sqrt(perChunk)*1.3)
		chunk := &food{
			ID:     randomID(),
			X:      clamp(p.X+math.Cos(angle)*(p.Radius+chunkRadius+18), chunkRadius, worldSize-chunkRadius),
			Y:      clamp(p.Y+math.Sin(angle)*(p.Radius+chunkRadius+18), chunkRadius, worldSize-chunkRadius),
			Radius: chunkRadius,
			Value:  perChunk,
			VX:     math.Cos(angle) * 520,
			VY:     math.Sin(angle) * 520,
		}
		s.foods = append(s.foods, chunk)
	}

	recoil := 42.0
	p.X = clamp(p.X-dir.X*recoil, currentRadius(p), worldSize-currentRadius(p))
	p.Y = clamp(p.Y-dir.Y*recoil, currentRadius(p), worldSize-currentRadius(p))
}

func (s *gameState) reconcileBotsLocked() {
	humanOwners := make(map[string]bool)
	botOwners := make(map[string]bool)
	var botMains []*player

	// 1. 조각이 아닌 '고유 소유자(본체)' 기준으로 인원수 계산
	for _, p := range s.players {
		ownerID := p.OwnerID
		if ownerID == "" {
			ownerID = p.ID
		}

		if p.IsBot {
			if !botOwners[ownerID] {
				botOwners[ownerID] = true
				botMains = append(botMains, p) // 삭제 기준이 될 대표 봇 1개만 수집
			}
		} else {
			humanOwners[ownerID] = true
		}
	}

	humans := len(humanOwners)
	requiredBots := s.config.MinimumPlayers - humans
	if requiredBots < 0 {
		requiredBots = 0
	}

	// 2. 봇이 너무 많으면 고유 봇 기준으로 그 봇의 '모든 조각'을 깔끔하게 삭제
	for len(botMains) > requiredBots {
		botToRemove := botMains[len(botMains)-1]
		removeOwnerID := botToRemove.OwnerID
		if removeOwnerID == "" {
			removeOwnerID = botToRemove.ID
		}

		for id, p := range s.players {
			pOwner := p.OwnerID
			if pOwner == "" {
				pOwner = p.ID
			}
			if pOwner == removeOwnerID {
				if p.Conn != nil {
					p.Conn.close()
				}
				delete(s.players, id)
			}
		}

		botMains[len(botMains)-1] = nil
		botMains = botMains[:len(botMains)-1]
	}

	// 3. 부족한 봇 보충
	for len(botMains) < requiredBots {
		bot := newBotPlayer(len(botMains) + 1)
		s.players[bot.ID] = bot
		botMains = append(botMains, bot)
	}
}

func newBotPlayer(index int) *player {
	cellType := randomBotCellType()
	now := time.Now()
	mass := playerStartMass + mathrand.Float64()*18
	id := randomID()
	return &player{
		ID:             id,
		SessionID:      "",
		OwnerID:        id,
		Nickname:       randomBotNickname(index),
		CellType:       cellType,
		Ability:        abilityName(cellType),
		X:              400 + mathrand.Float64()*(worldSize-800),
		Y:              400 + mathrand.Float64()*(worldSize-800),
		Mass:           mass,
		Radius:         massToRadius(mass),
		Scale:          1,
		Color:          randomColor(),
		IsBot:          true,
		LastSeen:       now,
		NextBotThinkAt: now,
		Energy:         4000,
	}
}

func randomBotCellType() string {
	cellTypes := []string{"classic", "blink", "giant", "shield", "magnet", "divider"}
	return cellTypes[mathrand.Intn(len(cellTypes))]
}

func randomBotNickname(index int) string {
	prefixes := []string{"Nova", "Lumi", "Aero", "Milo", "Rin", "Nex", "Sora", "Kai", "Yuna", "Theo", "Lyn", "Iris"}
	suffixes := []string{"Fox", "Ray", "Bit", "Run", "Pulse", "Mint", "Zero", "Core", "Dash", "Pop", "Wave", "Byte"}
	if mathrand.Float64() < 0.35 {
		return fmt.Sprintf("%s%d", prefixes[mathrand.Intn(len(prefixes))], 10+((index+mathrand.Intn(70))%90))
	}
	return prefixes[mathrand.Intn(len(prefixes))] + suffixes[mathrand.Intn(len(suffixes))]
}

func (s *gameState) updateBotLocked(p *player, now time.Time) {
	p.LastSeen = now
	if now.Before(p.NextBotThinkAt) {
		return
	}

	p.NextBotThinkAt = now.Add(time.Duration(400+mathrand.Intn(700)) * time.Millisecond)

	nearestFood := s.findNearestFoodLocked(p)
	smallerTarget := s.findNearestPlayerLocked(p, func(other *player) bool {
		return other.ID != p.ID && other.Mass < p.Mass*0.9
	})
	largerThreat := s.findNearestPlayerLocked(p, func(other *player) bool {
		return other.ID != p.ID && other.Mass > p.Mass*1.18
	})

	switch {
	case largerThreat != nil && distance(p.X, p.Y, largerThreat.X, largerThreat.Y) < 260:
		p.Direction = normalizeDirection(p.X-largerThreat.X, p.Y-largerThreat.Y)
		if p.CellType == "blink" || p.CellType == "shield" {
			s.tryUseAbility(p)
		}
	case smallerTarget != nil && distance(p.X, p.Y, smallerTarget.X, smallerTarget.Y) < 320:
		p.Direction = normalizeDirection(smallerTarget.X-p.X, smallerTarget.Y-p.Y)
		if (p.CellType == "giant" && p.Mass > smallerTarget.Mass*1.12) || (p.CellType == "divider" && p.Mass > dividerMinSplitMass*1.2) {
			s.tryUseAbility(p)
		}
	case nearestFood != nil:
		p.Direction = normalizeDirection(nearestFood.X-p.X, nearestFood.Y-p.Y)
		if p.CellType == "magnet" {
			s.tryUseAbility(p)
		}
	default:
		p.Direction = normalizeDirection(mathrand.Float64()*2-1, mathrand.Float64()*2-1)
	}
	if p.CellType == "classic" {
		shouldDash := false
		if largerThreat != nil && distance(p.X, p.Y, largerThreat.X, largerThreat.Y) < 300 {
			shouldDash = true
		} else if smallerTarget != nil && distance(p.X, p.Y, smallerTarget.X, smallerTarget.Y) < 350 {
			shouldDash = true
		}
		p.IsAbilityActive = shouldDash // 위협이나 타겟이 있을 때만 가속 버튼 누르기
	}
}

func (s *gameState) findNearestFoodLocked(p *player) *food {
	var best *food
	bestDistance := math.MaxFloat64
	for _, f := range s.foods {
		dist := distance(p.X, p.Y, f.X, f.Y)
		if dist < bestDistance {
			bestDistance = dist
			best = f
		}
	}
	return best
}

func (s *gameState) findNearestPlayerLocked(p *player, predicate func(*player) bool) *player {
	var best *player
	bestDistance := math.MaxFloat64
	for _, other := range s.players {
		if !predicate(other) {
			continue
		}
		dist := distance(p.X, p.Y, other.X, other.Y)
		if dist < bestDistance {
			bestDistance = dist
			best = other
		}
	}
	return best
}

func randomID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hexFormat(buf)
}

func hexFormat(buf []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(buf)*2)
	for i, b := range buf {
		out[i*2] = digits[b>>4]
		out[i*2+1] = digits[b&0x0f]
	}
	return string(out)
}
