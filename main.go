package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	mathrand "math/rand"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	port                     = "8000"
	configFileName           = "runtime-config.json"
	superAdminConfigFileName = "super-admin.local.json"
	defaultWorldSize         = 3600.0
	maxWorldSize             = 7200.0
	minWorldSize             = 1600.0
	foodTarget               = 320
	cactusTarget             = 28
	defaultWormholePairs     = 3
	playerStartMass          = 36.0
	tickRate                 = 30
	playerTimeout            = 60 * time.Second
	playerCullRange          = 1280.0
	foodCullRange            = 1460.0
	objectCullRange          = 1600.0
	defaultMinimumPlayers    = 20
	defaultBaseSpeed         = 285.0
	defaultSpeedDivisor      = 8.5
	defaultMinimumSpeed      = 92.0
	probioticTarget          = 100
	probioticSpawnEvery      = 3 * time.Second
	probioticGrowthDuration  = 32 * time.Second
	probioticShieldDuration  = 10 * time.Second
	probioticSpeedDuration   = 18 * time.Second
	probioticSpeedBoost      = 1.32
	worldResetInterval       = 30 * time.Minute
	worldResetWarningWindow  = 5 * time.Minute
	upgradeCost              = 12
	dividerSplitCooldown     = 1400 * time.Millisecond
	dividerMergeDelay        = 7 * time.Second
	dividerMinSplitMass      = 40.0
	dividerMaxFragments      = 16
	cactusTriggerRatio       = 0.38
	cactusFragmentMassMin    = 24.0

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
	mu                     sync.RWMutex
	players                map[string]*player
	foods                  []*food
	cacti                  []*cactus
	wormholes              []*wormhole
	chats                  []chatEntry
	config                 runtimeConfig
	spatialCache           *spatialGrid
	lastCactusRelocation   time.Time
	lastWormholeRelocation time.Time
	lastProbioticSpawn     time.Time
	nextWorldResetAt       time.Time
}

type runtimeConfig struct {
	MinimumPlayers          int     `json:"minimumPlayers"`
	ProbioticCount          int     `json:"probioticCount"`
	CactusCount             int     `json:"cactusCount"`
	WormholePairs           int     `json:"wormholePairs"`
	CactusRelocateSeconds   int     `json:"cactusRelocateSeconds"`
	WormholeRelocateSeconds int     `json:"wormholeRelocateSeconds"`
	WorldSize               float64 `json:"worldSize"`
	BaseSpeed               float64 `json:"baseSpeed"`
	SpeedDivisor            float64 `json:"speedDivisor"`
	MinimumSpeed            float64 `json:"minimumSpeed"`
}

type player struct {
	ID                  string       `json:"id"`
	SessionID           string       `json:"-"`
	OwnerID             string       `json:"ownerId"`
	Nickname            string       `json:"nickname"`
	CellType            string       `json:"cellType"`
	Ability             string       `json:"abilityName"`
	X                   float64      `json:"x"`
	Y                   float64      `json:"y"`
	Mass                float64      `json:"mass"`
	Radius              float64      `json:"radius"`
	Scale               float64      `json:"scale"`
	Color               string       `json:"color"`
	IsBot               bool         `json:"isBot"`
	Coins               int          `json:"coins"`
	Upgrades            upgradeState `json:"upgrades"`
	Direction           direction    `json:"-"`
	CooldownRemaining   int64        `json:"cooldownRemaining"`
	EffectRemaining     int64        `json:"effectRemaining"`
	ShieldRemaining     int64        `json:"shieldRemaining"`
	ProbioticRemaining  int64        `json:"probioticRemaining"`
	SpeedBoostRemaining int64        `json:"speedBoostRemaining"`
	RespawnRemaining    int64        `json:"respawnRemaining"`
	CooldownUntil       time.Time    `json:"-"`
	EffectUntil         time.Time    `json:"-"`
	ProbioticUntil      time.Time    `json:"-"`
	ShieldUntil         time.Time    `json:"-"`
	SpeedBoostUntil     time.Time    `json:"-"`
	CactusUntil         time.Time    `json:"-"`
	PortalUntil         time.Time    `json:"-"`
	MergeReadyAt        time.Time    `json:"-"`
	LastSeen            time.Time    `json:"-"`
	NextBotThinkAt      time.Time    `json:"-"`
	RespawnAt           time.Time    `json:"-"`
	Conn                *wsConn      `json:"-"`
	IsAbilityActive     bool         `json:"-"` // ✅ 추가: 현재 스킬 버튼 누름 여부
	Energy              float64      `json:"-"` // ✅ 추가: 오버클럭 에너지 (0 ~ 4000)
}

type direction struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type upgradeState struct {
	Classic bool `json:"classic"`
	Blink   bool `json:"blink"`
	Giant   bool `json:"giant"`
	Shield  bool `json:"shield"`
	Magnet  bool `json:"magnet"`
	Divider bool `json:"divider"`
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
	UseMerge   bool      `json:"useMerge"`
	Message    string    `json:"message,omitempty"`
	Upgrade    string    `json:"upgrade,omitempty"`
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
	ProbioticCount          *int     `json:"probioticCount"`
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

	s.updateBotOwnersLocked(now)

	for id, p := range s.players {
		if !p.IsBot && now.Sub(p.LastSeen) > playerTimeout {
			if p.Conn != nil {
				p.Conn.close()
			}
			delete(s.players, id)
			continue
		}

		if !p.RespawnAt.IsZero() && !isRespawningAt(now, p) {
			respawnPlayer(p, worldSize)
			p.LastSeen = now
			if p.IsBot {
				p.NextBotThinkAt = now
			}
		}
		if isRespawningAt(now, p) {
			continue
		}

		speed := s.movementSpeed(p.Mass)
		scaleMultiplier := 1.0
		upgraded := upgradeEnabledForCellType(p.Upgrades, p.CellType)
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
			maxEnergy := 4000.0
			activeDuration := 1.5
			rechargeDuration := 4.0
			if upgraded {
				maxEnergy = 6200.0
				activeDuration = 2.6
				rechargeDuration = 4.6
			}
			depleteRate := maxEnergy / (activeDuration * tickRate)
			rechargeRate := maxEnergy / (rechargeDuration * tickRate)

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
				if p.Energy > maxEnergy {
					p.Energy = maxEnergy
				}
			}
		}
		p.Scale = scaleMultiplier
		effectiveRadius := currentRadius(p)
		p.X = clamp(p.X+p.Direction.X*speed/tickRate, effectiveRadius, worldSize-effectiveRadius)
		p.Y = clamp(p.Y+p.Direction.Y*speed/tickRate, effectiveRadius, worldSize-effectiveRadius)
		if p.CellType == "magnet" && now.Before(p.EffectUntil) {
			s.pullNearbyFoodLocked(p, 220)
			if upgraded {
				s.pullSmallerPlayersLocked(p, 280)
			}
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
	s.resolvePlayerEatingV2(now)
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
		keep.Coins = 0
		keep.Upgrades = upgradeState{}
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

func (s *gameState) broadcastSnapshot() {
	s.mu.RLock()
	snapshotNow := time.Now()

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
		if isRespawningAt(snapshotNow, p) {
			continue
		}
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
		ownerID := ownerIDOf(p)
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

		viewerOwnerID := ownerIDOf(viewer)

		// 캐싱된 중심점 가져오기
		var centerX, centerY float64
		viewerRespawning := isRespawningAt(snapshotNow, viewer)
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
		if viewerRespawning {
			players = append(players, clonePlayer(viewer))
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
	snapshotNow := time.Now()
	viewer, ok := s.players[playerID]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("viewer not found")
	}
	viewerOwnerID := ownerIDOf(viewer)
	ownerFragments := buildOwnerFragmentsIndex(s.players)
	centerX, centerY := ownerCenterFromFragments(fragmentsForOwner(ownerFragments, viewerOwnerID), s.worldSize())

	viewerRespawning := isRespawningAt(snapshotNow, viewer)
	players := make([]*player, 0, len(s.players))
	for _, p := range s.players {
		if isRespawningAt(snapshotNow, p) {
			continue
		}
		targetOwnerID := ownerIDOf(p)
		if targetOwnerID != viewerOwnerID && !isWithinCullRange(centerX, centerY, p.X, p.Y, playerCullRange+currentRadius(p)) {
			continue
		}
		players = append(players, clonePlayer(p))
	}
	if viewerRespawning {
		players = append(players, clonePlayer(viewer))
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
	target := s.config.ProbioticCount
	if target < 0 {
		target = 0
	}
	if countBeneficialFoods(s.foods) >= target {
		return
	}
	if now.Sub(s.lastProbioticSpawn) < probioticSpawnEvery {
		return
	}
	s.foods = append(s.foods, createProbiotic(s.worldSize()))
	s.lastProbioticSpawn = now
}

func (s *gameState) reconcileProbioticsLocked() {
	target := s.config.ProbioticCount
	if target < 0 {
		target = 0
	}

	current := countBeneficialFoods(s.foods)
	if current > target {
		filtered := s.foods[:0]
		toRemove := current - target
		for _, item := range s.foods {
			if toRemove > 0 && isBeneficialFoodKind(item.Kind) {
				toRemove--
				continue
			}
			filtered = append(filtered, item)
		}
		for i := len(filtered); i < len(s.foods); i++ {
			s.foods[i] = nil
		}
		s.foods = filtered
		current = target
	}
	for current < target {
		s.foods = append(s.foods, createProbiotic(s.worldSize()))
		current++
	}
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
