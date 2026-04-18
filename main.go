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
	configFileName        = "runtime-config.json"
	superAdminConfigFileName = "super-admin.local.json"
	defaultWorldSize      = 3600.0
	maxWorldSize          = 7200.0
	minWorldSize          = 1600.0
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
	probioticTarget         = 5
	probioticSpawnEvery     = 18 * time.Second
	probioticGrowthDuration = 32 * time.Second
	probioticShieldDuration = 10 * time.Second
	probioticSpeedDuration  = 18 * time.Second
	probioticSpeedBoost     = 1.32
	worldResetInterval      = 30 * time.Minute
	worldResetWarningWindow = 5 * time.Minute
	dividerSplitCooldown  = 1400 * time.Millisecond
	dividerMergeDelay     = 7 * time.Second
	dividerMinSplitMass   = 40.0
	dividerMaxFragments   = 16
	cactusTriggerRatio    = 0.38
	cactusFragmentMassMin = 24.0

	// 공간 분할(Spatial Partitioning) 관련 상수
	spatialCellSize = 500.0
	spatialGridCols = (int(maxWorldSize) / int(spatialCellSize)) + 1
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
	chats        []chatEntry
	config       runtimeConfig
	spatialCache *spatialGrid
	lastCactusRelocation   time.Time
	lastWormholeRelocation time.Time
	lastProbioticSpawn     time.Time
	nextWorldResetAt       time.Time
}

type runtimeConfig struct {
	MinimumPlayers         int     `json:"minimumPlayers"`
	CactusCount            int     `json:"cactusCount"`
	WormholePairs          int     `json:"wormholePairs"`
	CactusRelocateSeconds  int     `json:"cactusRelocateSeconds"`
	WormholeRelocateSeconds int    `json:"wormholeRelocateSeconds"`
	WorldSize              float64 `json:"worldSize"`
	BaseSpeed              float64 `json:"baseSpeed"`
	SpeedDivisor           float64 `json:"speedDivisor"`
	MinimumSpeed           float64 `json:"minimumSpeed"`
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
	ProbioticUntil    time.Time `json:"-"`
	ShieldUntil       time.Time `json:"-"`
	SpeedBoostUntil   time.Time `json:"-"`
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
	Kind   string  `json:"kind,omitempty"`
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
	Message    string    `json:"message,omitempty"`
}

type snapshotMessage struct {
	Type        string         `json:"type"`
	Players     []*player      `json:"players"`
	Foods       []*food        `json:"foods"`
	Cacti       []*cactus      `json:"cacti"`
	Wormholes   []*wormhole    `json:"wormholes"`
	Leaderboard []ownerSummary `json:"leaderboard"`
	Chats       []chatEntry    `json:"chats"`
	Config      runtimeConfig  `json:"config"`
	ResetAt     int64          `json:"resetAt"`
}

type ownerSummary struct {
	OwnerID  string  `json:"ownerId"`
	Nickname string  `json:"nickname"`
	Mass     float64 `json:"mass"`
	IsBot    bool    `json:"isBot"`
}

type chatEntry struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Message  string `json:"message"`
	IsBot    bool   `json:"isBot"`
}

type adminStatusResponse struct {
	HumanPlayers int           `json:"humanPlayers"`
	BotPlayers   int           `json:"botPlayers"`
	TotalPlayers int           `json:"totalPlayers"`
	Config       runtimeConfig `json:"config"`
}

type adminConfigRequest struct {
	MinimumPlayers          *int     `json:"minimumPlayers"`
	CactusCount             *int     `json:"cactusCount"`
	WormholePairs           *int     `json:"wormholePairs"`
	CactusRelocateSeconds   *int     `json:"cactusRelocateSeconds"`
	WormholeRelocateSeconds *int     `json:"wormholeRelocateSeconds"`
	WorldSize               *float64 `json:"worldSize"`
	BaseSpeed               *float64 `json:"baseSpeed"`
	SpeedDivisor            *float64 `json:"speedDivisor"`
	MinimumSpeed            *float64 `json:"minimumSpeed"`
}

type superAdminConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
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

	config := defaultRuntimeConfig()
	if loaded, err := loadRuntimeConfig(); err != nil {
		log.Printf("failed to load runtime config: %v", err)
	} else {
		config = loaded
	}

	state := &gameState{
		players:                make(map[string]*player),
		foods:                  make([]*food, 0, foodTarget),
		cacti:                  make([]*cactus, 0, cactusTarget),
		wormholes:              make([]*wormhole, 0, defaultWormholePairs*2),
		chats:                  make([]chatEntry, 0, 20),
		config:                 config,
		spatialCache:           newSpatialGrid(),
		lastCactusRelocation:   time.Now(),
		lastWormholeRelocation: time.Now(),
		lastProbioticSpawn:     time.Now(),
		nextWorldResetAt:       nextWorldResetTime(time.Now()),
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
	worldSize := s.worldSize()

	p := &player{
		ID:        playerID,
		SessionID: sessionID,
		OwnerID:   playerID,
		Nickname:  nickname,
		CellType:  cellType,
		Ability:   abilityName(cellType),
		X:         spawnCoordinate(worldSize, 400),
		Y:         spawnCoordinate(worldSize, 400),
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
	humanOwners := make(map[string]struct{})
	botOwners := make(map[string]struct{})
	for _, p := range s.players {
		ownerID := p.OwnerID
		if ownerID == "" {
			ownerID = p.ID
		}
		if p.IsBot {
			botOwners[ownerID] = struct{}{}
		} else {
			humanOwners[ownerID] = struct{}{}
		}
	}
	humans := len(humanOwners)
	bots := len(botOwners)
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
	if req.CactusCount != nil {
		s.config.CactusCount = int(math.Max(0, float64(*req.CactusCount)))
	}
	if req.WormholePairs != nil {
		s.config.WormholePairs = int(math.Max(0, float64(*req.WormholePairs)))
	}
	if req.CactusRelocateSeconds != nil {
		s.config.CactusRelocateSeconds = int(math.Max(0, float64(*req.CactusRelocateSeconds)))
	}
	if req.WormholeRelocateSeconds != nil {
		s.config.WormholeRelocateSeconds = int(math.Max(0, float64(*req.WormholeRelocateSeconds)))
	}
	if req.WorldSize != nil {
		s.config.WorldSize = sanitizeWorldSize(*req.WorldSize)
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
	s.config = normalizeRuntimeConfig(s.config)
	s.clampWorldObjectsLocked()
	s.reconcileCactiLocked()
	s.reconcileWormholesLocked()
	s.reconcileBotsLocked()
	s.lastCactusRelocation = time.Now()
	s.lastWormholeRelocation = time.Now()
	config := s.config
	s.mu.Unlock()

	if err := saveRuntimeConfig(config); err != nil {
		http.Error(w, "failed to persist config", http.StatusInternalServerError)
		return
	}

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
		if msg.Type == "chat" {
			s.handleChatMessage(playerID, ws, msg.Message)
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
	if s.nextWorldResetAt.IsZero() {
		s.nextWorldResetAt = nextWorldResetTime(now)
	}
	if !now.Before(s.nextWorldResetAt) {
		s.resetWorldLocked(now)
		s.nextWorldResetAt = nextWorldResetTime(now.Add(time.Second))
	}
	s.maybeRelocateHazardsLocked(now)
	s.maybeSpawnProbioticsLocked(now)
	worldSize := s.worldSize()
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
		scaleMultiplier := 1.0
		if now.Before(p.ProbioticUntil) {
			scaleMultiplier *= 2
		}
		if now.Before(p.SpeedBoostUntil) {
			speed *= probioticSpeedBoost
		}
		switch {
		case p.CellType == "giant" && now.Before(p.EffectUntil):
			speed *= 0.68
			scaleMultiplier *= 1.9
		case p.CellType == "classic": // ✅ 오버클럭 에너지 제어 로직
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
		}
		p.Scale = scaleMultiplier
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
				if isBeneficialFoodKind(f.Kind) {
					ownerID := p.OwnerID
					if ownerID == "" {
						ownerID = p.ID
					}
					for _, fragment := range s.ownedPlayersLocked(ownerID) {
						switch f.Kind {
						case "probiotic-speed":
							fragment.SpeedBoostUntil = now.Add(probioticSpeedDuration)
						case "probiotic-shield":
							fragment.ShieldUntil = now.Add(probioticShieldDuration)
						default:
							fragment.ProbioticUntil = now.Add(probioticGrowthDuration)
						}
					}
				} else {
					p.Mass += f.Value
					p.Radius = massToRadius(p.Mass)
				}

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

func nextWorldResetTime(now time.Time) time.Time {
	base := now.Truncate(worldResetInterval)
	next := base.Add(worldResetInterval)
	if !next.After(now) {
		next = next.Add(worldResetInterval)
	}
	return next
}

func (s *gameState) resetWorldLocked(now time.Time) {
	owners := make(map[string]*player)
	for _, p := range s.players {
		ownerID := p.OwnerID
		if ownerID == "" {
			ownerID = p.ID
		}

		current, exists := owners[ownerID]
		if !exists || (current.SessionID == "" && p.SessionID != "") || p.Mass > current.Mass {
			owners[ownerID] = p
		}
	}

	for ownerID, keep := range owners {
		for _, fragment := range s.ownedPlayersLocked(ownerID) {
			if fragment.ID != keep.ID {
				delete(s.players, fragment.ID)
			}
		}
		respawnPlayer(keep, s.worldSize())
		keep.LastSeen = now
		if keep.IsBot {
			keep.NextBotThinkAt = now
		}
	}

	s.foods = s.foods[:0]
	s.cacti = s.cacti[:0]
	s.wormholes = s.wormholes[:0]
	s.seedFoods()
	s.reconcileCactiLocked()
	s.reconcileWormholesLocked()
	s.lastCactusRelocation = now
	s.lastWormholeRelocation = now
	s.lastProbioticSpawn = now
}

func (s *gameState) resolvePlayerEating() {
	players := make([]*player, 0, len(s.players))
	for _, p := range s.players {
		players = append(players, p)
	}

	now := time.Now() // 현재 시간 한 번만 호출
	worldSize := s.worldSize()

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
	chats := cloneChats(s.chats)
	config := s.config
	resetAt := s.nextWorldResetAt.UnixMilli()

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
				Chats:       chats,
				Config:      config,
				ResetAt:     resetAt,
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
	chats := cloneChats(s.chats)
	config := s.config
	resetAt := s.nextWorldResetAt.UnixMilli()
	s.mu.RUnlock()

	return json.Marshal(snapshotMessage{
		Type:        "snapshot",
		Players:     players,
		Foods:       foods,
		Cacti:       cacti,
		Wormholes:   wormholes,
		Leaderboard: leaderboard,
		Chats:       chats,
		Config:      config,
		ResetAt:     resetAt,
	})
}

func (s *gameState) seedFoods() {
	for countFoodsByKind(s.foods, "food") < foodTarget {
		s.foods = append(s.foods, createFood(s.worldSize()))
	}
}

func (s *gameState) seedCacti() {
	for len(s.cacti) < cactusTarget {
		s.cacti = append(s.cacti, createCactus(s.worldSize()))
	}
}

func (s *gameState) reconcileCactiLocked() {
	target := s.config.CactusCount
	if target < 0 {
		target = 0
	}

	for len(s.cacti) > target {
		s.cacti[len(s.cacti)-1] = nil
		s.cacti = s.cacti[:len(s.cacti)-1]
	}
	for len(s.cacti) < target {
		s.cacti = append(s.cacti, createCactus(s.worldSize()))
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
		entry := createWormhole(s.worldSize(), "blackhole", pairID)
		exit := createWormhole(s.worldSize(), "whitehole", pairID)
		for distance(entry.X, entry.Y, exit.X, exit.Y) < 700 {
			exit = createWormhole(s.worldSize(), "whitehole", pairID)
		}
		s.wormholes = append(s.wormholes, entry, exit)
	}
}

func (s *gameState) topUpFoods() {
	for countFoodsByKind(s.foods, "food") < foodTarget {
		s.foods = append(s.foods, createFood(s.worldSize()))
	}
}

func (s *gameState) maybeSpawnProbioticsLocked(now time.Time) {
	if countBeneficialFoods(s.foods) >= probioticTarget {
		return
	}
	if now.Sub(s.lastProbioticSpawn) < probioticSpawnEvery {
		return
	}
	s.foods = append(s.foods, createProbiotic(s.worldSize()))
	s.lastProbioticSpawn = now
}

func (s *gameState) maybeRelocateHazardsLocked(now time.Time) {
	if s.config.CactusRelocateSeconds > 0 && now.Sub(s.lastCactusRelocation) >= time.Duration(s.config.CactusRelocateSeconds)*time.Second {
		s.relocateCactiLocked()
		s.lastCactusRelocation = now
	}
	if s.config.WormholeRelocateSeconds > 0 && now.Sub(s.lastWormholeRelocation) >= time.Duration(s.config.WormholeRelocateSeconds)*time.Second {
		s.relocateWormholesLocked()
		s.lastWormholeRelocation = now
	}
}

func (s *gameState) relocateCactiLocked() {
	target := len(s.cacti)
	s.cacti = s.cacti[:0]
	for i := 0; i < target; i++ {
		s.cacti = append(s.cacti, createCactus(s.worldSize()))
	}
}

func (s *gameState) relocateWormholesLocked() {
	targetPairs := len(s.wormholes) / 2
	s.wormholes = s.wormholes[:0]
	for i := 0; i < targetPairs; i++ {
		pairID := randomID()
		entry := createWormhole(s.worldSize(), "blackhole", pairID)
		exit := createWormhole(s.worldSize(), "whitehole", pairID)
		for distance(entry.X, entry.Y, exit.X, exit.Y) < 700 {
			exit = createWormhole(s.worldSize(), "whitehole", pairID)
		}
		s.wormholes = append(s.wormholes, entry, exit)
	}
}

func countFoodsByKind(foods []*food, kind string) int {
	count := 0
	for _, item := range foods {
		currentKind := item.Kind
		if currentKind == "" {
			currentKind = "food"
		}
		if currentKind == kind {
			count++
		}
	}
	return count
}

func countBeneficialFoods(foods []*food) int {
	count := 0
	for _, item := range foods {
		if isBeneficialFoodKind(item.Kind) {
			count++
		}
	}
	return count
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

func appConfigPath(name string) (string, error) {
	root, err := appBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}

func configSearchPaths(name string) []string {
	paths := make([]string, 0, 2)
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, name))
	}
	if exePath, err := appConfigPath(name); err == nil {
		for _, existing := range paths {
			if strings.EqualFold(existing, exePath) {
				return paths
			}
		}
		paths = append(paths, exePath)
	}
	return paths
}

func defaultRuntimeConfig() runtimeConfig {
	return runtimeConfig{
		MinimumPlayers:          defaultMinimumPlayers,
		CactusCount:             cactusTarget,
		WormholePairs:           defaultWormholePairs,
		CactusRelocateSeconds:   0,
		WormholeRelocateSeconds: 0,
		WorldSize:               defaultWorldSize,
		BaseSpeed:               defaultBaseSpeed,
		SpeedDivisor:            defaultSpeedDivisor,
		MinimumSpeed:            defaultMinimumSpeed,
	}
}

func runtimeConfigPath() (string, error) {
	return appConfigPath(configFileName)
}

func loadRuntimeConfig() (runtimeConfig, error) {
	path, err := runtimeConfigPath()
	if err != nil {
		return defaultRuntimeConfig(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultRuntimeConfig(), nil
		}
		return defaultRuntimeConfig(), err
	}

	config := defaultRuntimeConfig()
	if err := json.Unmarshal(data, &config); err != nil {
		return defaultRuntimeConfig(), err
	}
	return normalizeRuntimeConfig(config), nil
}

func saveRuntimeConfig(config runtimeConfig) error {
	path, err := runtimeConfigPath()
	if err != nil {
		return err
	}

	normalized := normalizeRuntimeConfig(config)
	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func normalizeRuntimeConfig(config runtimeConfig) runtimeConfig {
	config.MinimumPlayers = int(math.Max(0, float64(config.MinimumPlayers)))
	config.CactusCount = int(math.Max(0, float64(config.CactusCount)))
	config.WormholePairs = int(math.Max(0, float64(config.WormholePairs)))
	config.CactusRelocateSeconds = int(math.Max(0, float64(config.CactusRelocateSeconds)))
	config.WormholeRelocateSeconds = int(math.Max(0, float64(config.WormholeRelocateSeconds)))
	config.WorldSize = sanitizeWorldSize(config.WorldSize)
	config.BaseSpeed = math.Max(50, config.BaseSpeed)
	config.SpeedDivisor = math.Max(1, config.SpeedDivisor)
	config.MinimumSpeed = math.Max(10, config.MinimumSpeed)
	return config
}

func requireSuperAuth(w http.ResponseWriter, r *http.Request) bool {
	credentials, err := loadSuperAdminConfig()
	if err != nil {
		log.Printf("super admin auth unavailable: %v", err)
		http.Error(w, "super admin credentials are not configured", http.StatusServiceUnavailable)
		return false
	}

	expectedUser := credentials.Username
	expectedPassword := credentials.Password
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

func loadSuperAdminConfig() (superAdminConfig, error) {
	paths := configSearchPaths(superAdminConfigFileName)
	for _, path := range paths {
		if data, readErr := os.ReadFile(path); readErr == nil {
			var config superAdminConfig
			if err := json.Unmarshal(data, &config); err != nil {
				return superAdminConfig{}, fmt.Errorf("invalid %s at %s: %w", superAdminConfigFileName, path, err)
			}
			config.Username = strings.TrimSpace(config.Username)
			config.Password = strings.TrimSpace(config.Password)
			if config.Username == "" || config.Password == "" {
				return superAdminConfig{}, fmt.Errorf("%s at %s must include username and password", superAdminConfigFileName, path)
			}
			return config, nil
		} else if !os.IsNotExist(readErr) {
			return superAdminConfig{}, readErr
		}
	}

	config := superAdminConfig{
		Username: strings.TrimSpace(os.Getenv("SUPER_USERNAME")),
		Password: strings.TrimSpace(os.Getenv("SUPER_PASSWORD")),
	}
	if config.Username == "" || config.Password == "" {
		return superAdminConfig{}, fmt.Errorf("%s not found in working directory or executable directory, and SUPER_USERNAME/SUPER_PASSWORD are empty", superAdminConfigFileName)
	}
	return config, nil
}

func superAuthToken(username, password string) string {
	hash := sha1.Sum([]byte(username + ":" + password + ":super"))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func createFood(worldSize float64) *food {
	return &food{
		ID:     randomID(),
		X:      spawnCoordinate(worldSize, 30),
		Y:      spawnCoordinate(worldSize, 30),
		Radius: 6 + mathrand.Float64()*3,
		Value:  2 + mathrand.Float64()*2,
		Kind:   "food",
	}
}

func createProbiotic(worldSize float64) *food {
	kind := randomBeneficialFoodKind()
	radius := 12 + mathrand.Float64()*3
	if kind == "probiotic-shield" {
		radius += 1.5
	}
	return &food{
		ID:     randomID(),
		X:      spawnCoordinate(worldSize, 80),
		Y:      spawnCoordinate(worldSize, 80),
		Radius: radius,
		Value:  0,
		Kind:   kind,
	}
}

func randomBeneficialFoodKind() string {
	roll := mathrand.Float64()
	switch {
	case roll < 0.46:
		return "probiotic-growth"
	case roll < 0.82:
		return "probiotic-speed"
	default:
		return "probiotic-shield"
	}
}

func isBeneficialFoodKind(kind string) bool {
	switch kind {
	case "probiotic", "probiotic-growth", "probiotic-speed", "probiotic-shield":
		return true
	default:
		return false
	}
}

func createCactus(worldSize float64) *cactus {
	size := 20 + mathrand.Float64()*18
	return &cactus{
		ID:     randomID(),
		X:      spawnCoordinate(worldSize, 120),
		Y:      spawnCoordinate(worldSize, 120),
		Size:   size,
		Height: size * (1.4 + mathrand.Float64()*0.6),
	}
}

func createWormhole(worldSize float64, kind, pairID string) *wormhole {
	radius := 34 + mathrand.Float64()*10
	return &wormhole{
		ID:        randomID(),
		Kind:      kind,
		PairID:    pairID,
		X:         spawnCoordinate(worldSize, 240),
		Y:         spawnCoordinate(worldSize, 240),
		Radius:    radius,
		PullRange: radius * 4.6,
	}
}

func respawnPlayer(p *player, worldSize float64) {
	ownerID := p.OwnerID
	if ownerID == "" {
		ownerID = p.ID
	}
	p.Energy = 4000
	p.Mass = playerStartMass
	p.Radius = massToRadius(playerStartMass)
	p.X = spawnCoordinate(worldSize, 400)
	p.Y = spawnCoordinate(worldSize, 400)
	p.Scale = 1
	p.OwnerID = ownerID
	p.Direction = direction{}
	p.CooldownUntil = time.Time{}
	p.EffectUntil = time.Time{}
	p.ProbioticUntil = time.Time{}
	p.ShieldUntil = time.Time{}
	p.SpeedBoostUntil = time.Time{}
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
		worldSize := s.worldSize()
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
		respawnPlayer(victim, s.worldSize())
		return
	}

	if victim.ID != ownerID {
		delete(s.players, victim.ID)
		return
	}

	successor := largestOwnedFragmentExcluding(fragments, victim.ID)
	if successor == nil {
		respawnPlayer(victim, s.worldSize())
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
	victim.ProbioticUntil = successor.ProbioticUntil
	victim.ShieldUntil = successor.ShieldUntil
	victim.SpeedBoostUntil = successor.SpeedBoostUntil
	victim.CactusUntil = successor.CactusUntil
	victim.PortalUntil = successor.PortalUntil
	victim.MergeReadyAt = successor.MergeReadyAt
	victim.IsAbilityActive = successor.IsAbilityActive
	victim.Energy = successor.Energy
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
					elapsedAfterReady := math.Min(
						now.Sub(a.MergeReadyAt).Seconds(),
						now.Sub(b.MergeReadyAt).Seconds(),
					)
					mergeDistanceRatio := 0.58 + clamp(elapsedAfterReady/5.8, 0, 1)*0.16
					if distance(a.X, a.Y, b.X, b.Y) > (currentRadius(a)+currentRadius(b))*mergeDistanceRatio {
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
	worldSize := s.worldSize()
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
			distToCenter := distance(fragment.X, fragment.Y, centerX, centerY)
			if distToCenter > 1 {
				dirX := (centerX - fragment.X) / distToCenter
				dirY := (centerY - fragment.Y) / distToCenter
				movementIntent := math.Hypot(fragment.Direction.X, fragment.Direction.Y)
				mergePullBoost := 1.0
				if now.Before(fragment.MergeReadyAt) {
					progress := 1 - clamp(fragment.MergeReadyAt.Sub(now).Seconds()/dividerMergeDelay.Seconds(), 0, 1)
					mergePullBoost = 0.04 + progress*0.96
				} else {
					postReadyProgress := clamp(now.Sub(fragment.MergeReadyAt).Seconds()/5.8, 0, 1)
					mergePullBoost = 1.48 + postReadyProgress*1.42
				}
				idleBoost := 2.25
				if movementIntent < 0.18 {
					idleBoost = 3.4
				} else if movementIntent < 0.45 {
					idleBoost = 2.55
				} else if movementIntent < 0.8 {
					idleBoost = 2.05
				}
				idleBoost *= mergePullBoost
				pull := math.Min(108, distToCenter*0.25) * idleBoost / tickRate
				radius := currentRadius(fragment)
				fragment.X = clamp(fragment.X+dirX*pull, radius, worldSize-radius)
				fragment.Y = clamp(fragment.Y+dirY*pull, radius, worldSize-radius)
			}
		}

		for i := 0; i < len(fragments); i++ {
			a := fragments[i]
			if _, ok := s.players[a.ID]; !ok {
				continue
			}
			for j := i + 1; j < len(fragments); j++ {
				b := fragments[j]
				if _, ok := s.players[b.ID]; !ok {
					continue
				}

				radiusA := currentRadius(a)
				radiusB := currentRadius(b)
				dist := distance(a.X, a.Y, b.X, b.Y)
				softGapRatio := 0.78
				if now.Before(a.MergeReadyAt) || now.Before(b.MergeReadyAt) {
					remainingA := clamp(a.MergeReadyAt.Sub(now).Seconds()/dividerMergeDelay.Seconds(), 0, 1)
					remainingB := clamp(b.MergeReadyAt.Sub(now).Seconds()/dividerMergeDelay.Seconds(), 0, 1)
					remaining := math.Max(remainingA, remainingB)
					softGapRatio = 0.84 + remaining*0.32
				} else {
					elapsedAfterReady := math.Min(
						now.Sub(a.MergeReadyAt).Seconds(),
						now.Sub(b.MergeReadyAt).Seconds(),
					)
					postReadyProgress := clamp(elapsedAfterReady/5.8, 0, 1)
					softGapRatio = 0.74 - postReadyProgress*0.12
				}
				softGap := (radiusA + radiusB) * softGapRatio
				if dist >= softGap {
					continue
				}

				if dist < 0.001 {
					dist = 0.001
				}
				dirX := (a.X - b.X) / dist
				dirY := (a.Y - b.Y) / dist
				separation := (softGap - dist) * 0.5
				pushA := separation
				pushB := separation
				totalMass := a.Mass + b.Mass
				if totalMass > 0 {
					pushA *= b.Mass / totalMass
					pushB *= a.Mass / totalMass
				}

				a.X = clamp(a.X+dirX*pushA, radiusA, worldSize-radiusA)
				a.Y = clamp(a.Y+dirY*pushA, radiusA, worldSize-radiusA)
				b.X = clamp(b.X-dirX*pushB, radiusB, worldSize-radiusB)
				b.Y = clamp(b.Y-dirY*pushB, radiusB, worldSize-radiusB)
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

func sanitizeChatMessage(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.ReplaceAll(trimmed, "\r", " ")
	trimmed = strings.ReplaceAll(trimmed, "\n", " ")
	if len(trimmed) > 96 {
		trimmed = trimmed[:96]
	}
	return strings.TrimSpace(trimmed)
}

func sanitizeCellType(value string) string {
	switch value {
	case "blink", "giant", "shield", "magnet", "divider":
		return value
	default:
		return "classic"
	}
}

func cloneChats(entries []chatEntry) []chatEntry {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]chatEntry, len(entries))
	copy(cloned, entries)
	return cloned
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

func sanitizeWorldSize(size float64) float64 {
	if math.IsNaN(size) || math.IsInf(size, 0) {
		return defaultWorldSize
	}
	return clamp(size, minWorldSize, maxWorldSize)
}

func spawnCoordinate(worldSize, padding float64) float64 {
	if worldSize <= padding*2 {
		return worldSize * 0.5
	}
	return padding + mathrand.Float64()*(worldSize-padding*2)
}

func (s *gameState) worldSize() float64 {
	if s == nil {
		return defaultWorldSize
	}
	return sanitizeWorldSize(s.config.WorldSize)
}

func (s *gameState) clampWorldObjectsLocked() {
	worldSize := s.worldSize()
	for _, p := range s.players {
		radius := currentRadius(p)
		p.X = clamp(p.X, radius, worldSize-radius)
		p.Y = clamp(p.Y, radius, worldSize-radius)
	}
	for _, f := range s.foods {
		f.X = clamp(f.X, f.Radius, worldSize-f.Radius)
		f.Y = clamp(f.Y, f.Radius, worldSize-f.Radius)
	}
	for _, c := range s.cacti {
		padding := c.Size * 1.3
		c.X = clamp(c.X, padding, worldSize-padding)
		c.Y = clamp(c.Y, padding, worldSize-padding)
	}
	for _, w := range s.wormholes {
		padding := w.PullRange
		w.X = clamp(w.X, padding, worldSize-padding)
		w.Y = clamp(w.Y, padding, worldSize-padding)
	}
}

func (s *gameState) tryUseAbility(p *player) {
	now := time.Now()
	worldSize := s.worldSize()
	if now.Before(p.CooldownUntil) {
		return
	}

	switch p.CellType {
	case "classic":
		// Classic uses a held overclock state driven by Energy, not a server cooldown.
		return
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
	worldSize := s.worldSize()
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
	worldSize := s.worldSize()
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
			IsAbilityActive: fragment.IsAbilityActive,
			ProbioticUntil: fragment.ProbioticUntil,
			ShieldUntil:    fragment.ShieldUntil,
			SpeedBoostUntil: fragment.SpeedBoostUntil,
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

func effectiveCombatMass(p *player) float64 {
	return p.Mass * math.Max(1, p.Scale)
}

func canEatPlayer(attacker, defender *player, gap float64) bool {
	attackerRadius := currentRadius(attacker)
	defenderRadius := currentRadius(defender)
	requiredCenterDepth := attackerRadius - defenderRadius*0.5
	if gap > requiredCenterDepth {
		return false
	}

	if time.Now().Before(defender.ShieldUntil) {
		return false
	}

	if effectiveCombatMass(attacker) <= effectiveCombatMass(defender)*1.1 {
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
	worldSize := s.worldSize()
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

		ownerID := p.OwnerID
		if ownerID == "" {
			ownerID = p.ID
		}
		s.teleportOwnerThroughWormholeLocked(ownerID, p, exit, offset, now, worldSize)
		return
	}
}

func (s *gameState) teleportOwnerThroughWormholeLocked(ownerID string, trigger *player, exit *wormhole, offset direction, now time.Time, worldSize float64) {
	fragments := s.ownedPlayersLocked(ownerID)
	if len(fragments) == 0 {
		return
	}

	centerX, centerY := s.ownerCenterLocked(ownerID)
	maxRadius := 0.0
	for _, fragment := range fragments {
		maxRadius = math.Max(maxRadius, currentRadius(fragment))
	}

	targetCenterX := exit.X + offset.X*(exit.Radius+maxRadius+56)
	targetCenterY := exit.Y + offset.Y*(exit.Radius+maxRadius+56)
	portalLockUntil := now.Add(1500 * time.Millisecond)

	for _, fragment := range fragments {
		radius := currentRadius(fragment)
		relX := fragment.X - centerX
		relY := fragment.Y - centerY
		fragment.X = clamp(targetCenterX+relX, radius, worldSize-radius)
		fragment.Y = clamp(targetCenterY+relY, radius, worldSize-radius)
		fragment.PortalUntil = portalLockUntil
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
	worldSize := s.worldSize()
	if now.Before(p.CactusUntil) {
		return
	}

	for _, c := range s.cacti {
		cactusRadius := c.Size * 1.18
		dist := distance(p.X, p.Y, c.X, c.Y)
		playerRadius := currentRadius(p)
		overlap := playerRadius + cactusRadius - dist
		requiredOverlap := math.Min(playerRadius, cactusRadius) * cactusTriggerRatio
		if overlap <= requiredOverlap {
			continue
		}

		dir := normalizeDirection(p.X-c.X, p.Y-c.Y)
		if dir.X == 0 && dir.Y == 0 {
			dir = direction{X: 1}
		}

		p.CactusUntil = now.Add(1500 * time.Millisecond)

		if p.Mass >= 120 {
			s.forceSplitFromCactusLocked(p, dir, now)
		} else {
			escape := playerRadius + cactusRadius + 10
			p.X = clamp(c.X+dir.X*escape, currentRadius(p), worldSize-currentRadius(p))
			p.Y = clamp(c.Y+dir.Y*escape, currentRadius(p), worldSize-currentRadius(p))
		}
		return
	}
}

func (s *gameState) forceSplitFromCactusLocked(p *player, dir direction, now time.Time) {
	worldSize := s.worldSize()
	loss := math.Min(p.Mass*0.48, 520)
	remainingMass := math.Max(playerStartMass, p.Mass-loss)
	splitMass := p.Mass - remainingMass
	ownerID := p.OwnerID
	if ownerID == "" {
		ownerID = p.ID
	}
	fragments := s.ownedPlayersLocked(ownerID)
	availableSlots := dividerMaxFragments - len(fragments)
	if availableSlots <= 0 || splitMass < cactusFragmentMassMin {
		recoil := 42.0
		p.X = clamp(p.X-dir.X*recoil, currentRadius(p), worldSize-currentRadius(p))
		p.Y = clamp(p.Y-dir.Y*recoil, currentRadius(p), worldSize-currentRadius(p))
		return
	}

	childCount := int(math.Min(float64(availableSlots), math.Max(2, math.Floor(splitMass/cactusFragmentMassMin))))
	for childCount > 1 && splitMass/float64(childCount) < cactusFragmentMassMin {
		childCount--
	}

	p.Mass = remainingMass
	p.Radius = massToRadius(p.Mass)
	p.MergeReadyAt = now.Add(dividerMergeDelay)

	perChildMass := splitMass / float64(childCount)
	baseAngle := math.Atan2(dir.Y, dir.X)
	for i := 0; i < childCount; i++ {
		angle := baseAngle + (float64(i)-float64(childCount-1)/2)*0.42
		offsetX := math.Cos(angle)
		offsetY := math.Sin(angle)
		childRadius := massToRadius(perChildMass)
		spawnDistance := p.Radius + childRadius + 22
		child := &player{
			ID:               randomID(),
			SessionID:        "",
			OwnerID:          ownerID,
			Nickname:         p.Nickname,
			CellType:         p.CellType,
			Ability:          p.Ability,
			X:                clamp(p.X+offsetX*spawnDistance, childRadius, worldSize-childRadius),
			Y:                clamp(p.Y+offsetY*spawnDistance, childRadius, worldSize-childRadius),
			Mass:             perChildMass,
			Radius:           childRadius,
			Scale:            1,
			Color:            p.Color,
			IsBot:            p.IsBot,
			Direction:        direction{X: offsetX, Y: offsetY},
			MergeReadyAt:     now.Add(dividerMergeDelay),
			LastSeen:         p.LastSeen,
			NextBotThinkAt:   p.NextBotThinkAt,
			IsAbilityActive:  false,
			ProbioticUntil:   p.ProbioticUntil,
			ShieldUntil:      p.ShieldUntil,
			SpeedBoostUntil:  p.SpeedBoostUntil,
			Energy:           p.Energy,
		}
		s.players[child.ID] = child
	}

	recoil := 42.0
	p.X = clamp(p.X-dir.X*recoil, currentRadius(p), worldSize-currentRadius(p))
	p.Y = clamp(p.Y-dir.Y*recoil, currentRadius(p), worldSize-currentRadius(p))
}

func (s *gameState) reconcileBotsLocked() {
	humanOwners := make(map[string]bool)
	botOwners := make(map[string]bool)
	usedBotNicknames := make(map[string]struct{})
	var botMains []*player

	// 1. 조각이 아닌 '고유 소유자(본체)' 기준으로 인원수 계산
	for _, p := range s.players {
		ownerID := p.OwnerID
		if ownerID == "" {
			ownerID = p.ID
		}

		if p.IsBot {
			usedBotNicknames[p.Nickname] = struct{}{}
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
		bot := newBotPlayer(len(botMains)+1, usedBotNicknames, s.worldSize())
		s.players[bot.ID] = bot
		botMains = append(botMains, bot)
		usedBotNicknames[bot.Nickname] = struct{}{}
	}
}

func newBotPlayer(index int, usedNicknames map[string]struct{}, worldSize float64) *player {
	cellType := randomBotCellType()
	now := time.Now()
	mass := playerStartMass + mathrand.Float64()*18
	id := randomID()
	return &player{
		ID:             id,
		SessionID:      "",
		OwnerID:        id,
		Nickname:       randomPreferredBotNickname(index, usedNicknames),
		CellType:       cellType,
		Ability:        abilityName(cellType),
		X:              spawnCoordinate(worldSize, 400),
		Y:              spawnCoordinate(worldSize, 400),
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

func randomPreferredBotNickname(index int, usedNicknames map[string]struct{}) string {
	englishFirst := []string{"Nova", "Lumi", "Aero", "Milo", "Rin", "Nex", "Sora", "Kai", "Yuna", "Theo", "Lyn", "Iris"}
	englishLast := []string{"Fox", "Ray", "Bit", "Run", "Pulse", "Mint", "Zero", "Core", "Dash", "Pop", "Wave", "Byte"}
	korean := []string{
		"하린", "도윤", "서윤", "민준", "예준", "지우", "시아", "서진", "준호", "유나",
		"수아", "지안", "연우", "하율", "은호", "시우", "채원", "지호", "주원", "하람",
		"다온", "태오", "이안", "나린", "아린", "가온", "라온", "유진", "서아", "하은",
		"로아", "지유", "도하", "시온", "민서", "예린", "태린", "하진", "윤서", "규리",
		"다인", "선우", "도현", "세아", "현우", "다윤", "주하", "윤아", "예담", "하민",
	}

	for attempt := 0; attempt < 96; attempt++ {
		var candidate string
		switch mathrand.Intn(4) {
		case 0, 1:
			name := korean[mathrand.Intn(len(korean))]
			if mathrand.Float64() < 0.42 {
				candidate = fmt.Sprintf("%s%d", name, 1+((index+mathrand.Intn(98))%99))
			} else {
				candidate = name
			}
		case 2:
			nameA := korean[mathrand.Intn(len(korean))]
			nameB := korean[mathrand.Intn(len(korean))]
			candidate = nameA + nameB
		default:
			if mathrand.Float64() < 0.45 {
				candidate = fmt.Sprintf("%s%d", englishFirst[mathrand.Intn(len(englishFirst))], 10+((index+mathrand.Intn(70))%90))
			} else {
				candidate = englishFirst[mathrand.Intn(len(englishFirst))] + englishLast[mathrand.Intn(len(englishLast))]
			}
		}

		if _, exists := usedNicknames[candidate]; !exists {
			return candidate
		}
	}

	return fmt.Sprintf("Bot-%03d", index)
}

func randomBotNickname(index int, usedNicknames map[string]struct{}) string {
	englishFirst := []string{"Nova", "Lumi", "Aero", "Milo", "Rin", "Nex", "Sora", "Kai", "Yuna", "Theo", "Lyn", "Iris"}
	englishLast := []string{"Fox", "Ray", "Bit", "Run", "Pulse", "Mint", "Zero", "Core", "Dash", "Pop", "Wave", "Byte"}
	korean := []string{"하루", "서준", "지안", "도윤", "민서", "유나", "시우", "은호", "나린", "태오", "하린", "수아"}
	chinese := []string{"小龙", "雨晨", "子轩", "可欣", "星宇", "明哲", "安琪", "梓涵", "天佑", "欣怡", "浩然", "若曦"}
	japanese := []string{"ハル", "ユイ", "ソラ", "レン", "ミオ", "アオイ", "リク", "ユナ", "ナギ", "カイ", "ヒナ", "サラ"}

	for attempt := 0; attempt < 64; attempt++ {
		var candidate string
		switch mathrand.Intn(4) {
		case 0:
			if mathrand.Float64() < 0.45 {
				candidate = fmt.Sprintf("%s%d", englishFirst[mathrand.Intn(len(englishFirst))], 10+((index+mathrand.Intn(70))%90))
			} else {
				candidate = englishFirst[mathrand.Intn(len(englishFirst))] + englishLast[mathrand.Intn(len(englishLast))]
			}
		case 1:
			name := korean[mathrand.Intn(len(korean))]
			if mathrand.Float64() < 0.3 {
				candidate = fmt.Sprintf("%s%d", name, 1+((index+mathrand.Intn(98))%99))
			} else {
				candidate = name
			}
		case 2:
			name := chinese[mathrand.Intn(len(chinese))]
			if mathrand.Float64() < 0.25 {
				candidate = fmt.Sprintf("%s%d", name, 1+((index+mathrand.Intn(98))%99))
			} else {
				candidate = name
			}
		default:
			name := japanese[mathrand.Intn(len(japanese))]
			if mathrand.Float64() < 0.25 {
				candidate = fmt.Sprintf("%s%d", name, 1+((index+mathrand.Intn(98))%99))
			} else {
				candidate = name
			}
		}

		if _, exists := usedNicknames[candidate]; !exists {
			return candidate
		}
	}

	return fmt.Sprintf("Bot-%03d", index)
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
