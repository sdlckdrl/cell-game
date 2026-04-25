package main

import (
	"bytes"
	"encoding/binary"
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
	leechTarget              = 12
	defaultWormholePairs     = 3
	playerStartMass          = 36.0
	tickRate                 = 30
	snapshotRate             = 15
	leaderboardMetaSeconds   = 2
	chatsMetaSeconds         = 2
	configMetaSeconds        = 3
	resetMetaSeconds         = 2
	foodSnapshotEvery        = 2
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
	leechAttachDuration      = 30 * time.Second
	leechDrainFraction       = 0.30
	leechSwimSpeed           = 38.0
	leechInitialMass         = 18.0
	leechMaxMass             = 120.0
	leechAttachCooldown      = 1500 * time.Millisecond
	leechFedCooldown         = 10 * time.Second
	leechBurstDuration       = 1200 * time.Millisecond
	leechMaxSizeScale        = 1.10

	// 공간 분할(Spatial Partitioning) 관련 상수
	spatialCellSize = 500.0
	spatialGridCols = (int(maxWorldSize) / int(spatialCellSize)) + 1
	spatialGridRows = spatialGridCols

	coordQuantScale     = 8.0
	radiusQuantScale    = 8.0
	valueQuantScale     = 16.0
	scaleQuantScale     = 1024.0
	massQuantScale      = 16.0
	durationQuantStepMs = 100
)

var mimeTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".js":   "text/javascript; charset=utf-8",
	".json": "application/json; charset=utf-8",
}

var snapshotBufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 4096))
	},
}

var jsonBufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

var signatureMapPool = sync.Pool{
	New: func() any {
		return make(map[string]uint64, 128)
	},
}

const maxPooledBufferCap = 256 * 1024

type gameState struct {
	mu                     sync.RWMutex
	players                map[string]*player
	foods                  []*food
	cacti                  []*cactus
	leeches                []*leechVirus
	wormholes              []*wormhole
	chats                  []chatEntry
	config                 runtimeConfig
	spatialCache           *spatialGrid
	lastLeaderboardPayload []byte
	lastChatsPayload       []byte
	lastConfigPayload      []byte
	lastResetPayload       []byte
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
	LeechCount              int     `json:"leechCount"`
	LeechAttachSeconds      int     `json:"leechAttachSeconds"`
	LeechDrainPercent       float64 `json:"leechDrainPercent"`
	LeechFedCooldownSeconds int     `json:"leechFedCooldownSeconds"`
	LeechMaxMass            float64 `json:"leechMaxMass"`
	LeechSwimSpeed          float64 `json:"leechSwimSpeed"`
	LeechMaxSizeScale       float64 `json:"leechMaxSizeScale"`
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

type leechVirus struct {
	ID                string    `json:"id"`
	X                 float64   `json:"x"`
	Y                 float64   `json:"y"`
	Size              float64   `json:"size"`
	Mass              float64   `json:"mass"`
	Angle             float64   `json:"angle"`
	AttachedTo        string    `json:"attachedTo,omitempty"`
	AttachedRemaining int64     `json:"attachedRemaining,omitempty"`
	BurstRemaining    int64     `json:"burstRemaining,omitempty"`
	BaseMass          float64   `json:"-"`
	BaseSize          float64   `json:"-"`
	AttachOffset      float64   `json:"-"`
	AttachDistance    float64   `json:"-"`
	AttachedUntil     time.Time `json:"-"`
	BurstUntil        time.Time `json:"-"`
	DrainTotal        float64   `json:"-"`
	DrainedMass       float64   `json:"-"`
	LastDrainAt       time.Time `json:"-"`
	NextAttachAt      time.Time `json:"-"`
	SwimAngle         float64   `json:"-"`
	WigglePhase       float64   `json:"-"`
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
	Type               string        `json:"type"`
	Full               bool          `json:"full,omitempty"`
	Players            []*player     `json:"players"`
	RemovedPlayerIDs   []string      `json:"removedPlayerIds,omitempty"`
	Foods              []*food       `json:"foods,omitempty"`
	Cacti              []*cactus     `json:"cacti,omitempty"`
	Leeches            []*leechVirus `json:"leeches,omitempty"`
	Wormholes          []*wormhole   `json:"wormholes,omitempty"`
	RemovedFoodIDs     []string      `json:"removedFoodIds,omitempty"`
	RemovedCactusIDs   []string      `json:"removedCactusIds,omitempty"`
	RemovedLeechIDs    []string      `json:"removedLeechIds,omitempty"`
	RemovedWormholeIDs []string      `json:"removedWormholeIds,omitempty"`
}

type leaderboardMessage struct {
	Type        string         `json:"type"`
	Leaderboard []ownerSummary `json:"leaderboard"`
}

type chatsMessage struct {
	Type  string      `json:"type"`
	Chats []chatEntry `json:"chats"`
}

type configMessage struct {
	Type   string        `json:"type"`
	Config runtimeConfig `json:"config"`
}

type resetMessage struct {
	Type    string `json:"type"`
	ResetAt int64  `json:"resetAt"`
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
	LeechCount              *int     `json:"leechCount"`
	LeechAttachSeconds      *int     `json:"leechAttachSeconds"`
	LeechDrainPercent       *float64 `json:"leechDrainPercent"`
	LeechFedCooldownSeconds *int     `json:"leechFedCooldownSeconds"`
	LeechMaxMass            *float64 `json:"leechMaxMass"`
	LeechSwimSpeed          *float64 `json:"leechSwimSpeed"`
	LeechMaxSizeScale       *float64 `json:"leechMaxSizeScale"`
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
	conn    net.Conn
	mu      sync.Mutex
	stateMu sync.Mutex
	cache   snapshotDeltaCache
	strings connectionStringTable
}

type snapshotDeltaCache struct {
	fullSent  bool
	players   map[string]uint64
	foods     map[string]uint64
	cacti     map[string]uint64
	leeches   map[string]uint64
	wormholes map[string]uint64
}

type connectionStringTable struct {
	ownerIDs       map[string]uint16
	nicknames      map[string]uint16
	colors         map[string]uint16
	abilities      map[string]uint16
	cellTypes      map[string]uint16
	nextOwnerID    uint16
	nextNicknameID uint16
	nextColorID    uint16
	nextAbilityID  uint16
	nextCellTypeID uint16
}

type stringTableEntry struct {
	ID    uint16 `json:"id"`
	Value string `json:"value"`
}

type stringTableMessage struct {
	Type         string             `json:"type"`
	OwnerIDs     []stringTableEntry `json:"ownerIds,omitempty"`
	Nicknames    []stringTableEntry `json:"nicknames,omitempty"`
	Colors       []stringTableEntry `json:"colors,omitempty"`
	AbilityNames []stringTableEntry `json:"abilityNames,omitempty"`
	CellTypes    []stringTableEntry `json:"cellTypes,omitempty"`
}

// ----------------------------------------------------
// 공간 분할(Spatial Partitioning) 헬퍼 구조체 및 함수
// ----------------------------------------------------

type spatialGrid struct {
	players   [][][]*player
	foods     [][][]*food
	cacti     [][][]*cactus
	leeches   [][][]*leechVirus
	wormholes [][][]*wormhole
}

func newSpatialGrid() *spatialGrid {
	g := &spatialGrid{
		players:   make([][][]*player, spatialGridCols),
		foods:     make([][][]*food, spatialGridCols),
		cacti:     make([][][]*cactus, spatialGridCols),
		leeches:   make([][][]*leechVirus, spatialGridCols),
		wormholes: make([][][]*wormhole, spatialGridCols),
	}
	for i := 0; i < spatialGridCols; i++ {
		g.players[i] = make([][]*player, spatialGridRows)
		g.foods[i] = make([][]*food, spatialGridRows)
		g.cacti[i] = make([][]*cactus, spatialGridRows)
		g.leeches[i] = make([][]*leechVirus, spatialGridRows)
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
		leeches:                make([]*leechVirus, 0, leechTarget),
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
	state.seedLeeches()
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
	snapshotInterval := maxInt(1, tickRate/maxInt(1, snapshotRate))
	leaderboardInterval := maxInt(1, tickRate*leaderboardMetaSeconds)
	chatsInterval := maxInt(1, tickRate*chatsMetaSeconds)
	configInterval := maxInt(1, tickRate*configMetaSeconds)
	resetInterval := maxInt(1, tickRate*resetMetaSeconds)
	tickCount := 0
	snapshotCount := 0

	for range ticker.C {
		s.updateWorld()
		tickCount++
		if tickCount%snapshotInterval == 0 {
			snapshotCount++
			includeFoods := snapshotCount%foodSnapshotEvery == 0
			s.broadcastSnapshot(includeFoods)
		}
		if tickCount%leaderboardInterval == 0 {
			s.broadcastLeaderboardMeta()
		}
		if tickCount%chatsInterval == 0 {
			s.broadcastChatsMeta()
		}
		if tickCount%configInterval == 0 {
			s.broadcastConfigMeta()
		}
		if tickCount%resetInterval == 0 {
			s.broadcastResetMeta()
		}
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
			s.respawnPlayerLocked(p)
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

	s.updateLeechVirusesLocked(now)
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
		s.respawnPlayerLocked(keep)
		keep.LastSeen = now
		if keep.IsBot {
			keep.NextBotThinkAt = now
		}
	}

	s.foods = s.foods[:0]
	s.cacti = s.cacti[:0]
	s.leeches = s.leeches[:0]
	s.wormholes = s.wormholes[:0]
	s.seedFoods()
	s.reconcileCactiLocked()
	s.reconcileLeechesLocked()
	s.reconcileWormholesLocked()
	s.lastCactusRelocation = now
	s.lastWormholeRelocation = now
	s.lastProbioticSpawn = now
}

func (s *gameState) broadcastSnapshot(includeFoods bool) {
	s.mu.RLock()
	snapshotNow := time.Now()

	// 1. O(N²) 방지를 위한 공간 분할(Grid) 캐시 구축
	grid := s.spatialCache
	for cx := 0; cx < spatialGridCols; cx++ {
		for cy := 0; cy < spatialGridRows; cy++ {
			grid.players[cx][cy] = grid.players[cx][cy][:0]
			grid.foods[cx][cy] = grid.foods[cx][cy][:0]
			grid.cacti[cx][cy] = grid.cacti[cx][cy][:0]
			grid.leeches[cx][cy] = grid.leeches[cx][cy][:0]
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
	for _, leech := range s.leeches {
		cx, cy := getCellIndex(leech.X, leech.Y)
		grid.leeches[cx][cy] = append(grid.leeches[cx][cy], leech)
	}
	for _, w := range s.wormholes {
		cx, cy := getCellIndex(w.X, w.Y)
		grid.wormholes[cx][cy] = append(grid.wormholes[cx][cy], w)
	}

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
		var leeches []*leechVirus
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
				for _, leech := range grid.leeches[cx][cy] {
					if !isWithinCullRange(centerX, centerY, leech.X, leech.Y, objectCullRange+leech.Size*2.4) {
						continue
					}
					leeches = append(leeches, cloneLeechVirus(leech, snapshotNow))
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
			conn:    viewer.Conn,
			message: s.buildDeltaSnapshotMessage(viewer.Conn, players, foods, cacti, leeches, wormholes, false, includeFoods),
		})
	}
	s.mu.RUnlock() // JSON 생성 후 ReadLock 조기 해제

	// 5. 실제 웹소켓 전송 (락 해제 상태에서 수행)
	for _, target := range targets {
		tablePayload, err := target.conn.buildStringTablePayloadForPlayers(target.message.Players)
		if err != nil {
			continue
		}
		if len(tablePayload) > 0 {
			if err := target.conn.writeText(tablePayload); err != nil {
				target.conn.close()
				continue
			}
		}
		if err := writeBinarySnapshot(target.conn, target.message); err != nil {
			target.conn.close()
		}
	}
}

func (s *gameState) broadcastLeaderboardMeta() {
	s.mu.RLock()
	targets := make([]*wsConn, 0, len(s.players))
	for _, p := range s.players {
		if p.Conn != nil {
			targets = append(targets, p.Conn)
		}
	}
	s.mu.RUnlock()

	payload, changed, err := s.buildLeaderboardPayloadIfChanged()
	if err != nil || !changed || len(payload) == 0 {
		return
	}
	for _, conn := range targets {
		if err := conn.writeText(payload); err != nil {
			conn.close()
		}
	}
}

func (s *gameState) broadcastChatsMeta() {
	s.mu.RLock()
	targets := make([]*wsConn, 0, len(s.players))
	for _, p := range s.players {
		if p.Conn != nil {
			targets = append(targets, p.Conn)
		}
	}
	s.mu.RUnlock()

	payload, changed, err := s.buildChatsPayloadIfChanged()
	if err != nil || !changed || len(payload) == 0 {
		return
	}
	for _, conn := range targets {
		if err := conn.writeText(payload); err != nil {
			conn.close()
		}
	}
}

func (s *gameState) broadcastConfigMeta() {
	s.mu.RLock()
	targets := make([]*wsConn, 0, len(s.players))
	for _, p := range s.players {
		if p.Conn != nil {
			targets = append(targets, p.Conn)
		}
	}
	s.mu.RUnlock()

	payload, changed, err := s.buildConfigPayloadIfChanged()
	if err != nil || !changed || len(payload) == 0 {
		return
	}
	for _, conn := range targets {
		if err := conn.writeText(payload); err != nil {
			conn.close()
		}
	}
}

func (s *gameState) broadcastResetMeta() {
	s.mu.RLock()
	targets := make([]*wsConn, 0, len(s.players))
	for _, p := range s.players {
		if p.Conn != nil {
			targets = append(targets, p.Conn)
		}
	}
	s.mu.RUnlock()

	payload, changed, err := s.buildResetPayloadIfChanged()
	if err != nil || !changed || len(payload) == 0 {
		return
	}
	for _, conn := range targets {
		if err := conn.writeText(payload); err != nil {
			conn.close()
		}
	}
}

func (s *gameState) sendMetaTo(conn *wsConn) error {
	for _, builder := range []func() ([]byte, error){
		s.buildLeaderboardPayload,
		s.buildChatsPayload,
		s.buildConfigPayload,
		s.buildResetPayload,
	} {
		payload, err := builder()
		if err != nil {
			return err
		}
		if len(payload) == 0 {
			continue
		}
		if err := conn.writeText(payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *gameState) sendSnapshotTo(playerID string, conn *wsConn) error {
	tablePayload, message, err := s.buildSnapshotPayload(playerID, conn)
	if err != nil {
		return err
	}
	if len(tablePayload) > 0 {
		if err := conn.writeText(tablePayload); err != nil {
			return err
		}
	}
	return writeBinarySnapshot(conn, message)
}

func (s *gameState) buildLeaderboardPayload() ([]byte, error) {
	s.mu.RLock()
	message := leaderboardMessage{
		Type:        "leaderboard",
		Leaderboard: buildOwnerLeaderboard(s.players),
	}
	s.mu.RUnlock()
	return marshalJSONPooled(message)
}

func (s *gameState) buildChatsPayload() ([]byte, error) {
	s.mu.RLock()
	message := chatsMessage{
		Type:  "chats",
		Chats: cloneChats(s.chats),
	}
	s.mu.RUnlock()
	return marshalJSONPooled(message)
}

func (s *gameState) buildConfigPayload() ([]byte, error) {
	s.mu.RLock()
	message := configMessage{
		Type:   "config",
		Config: s.config,
	}
	s.mu.RUnlock()
	return marshalJSONPooled(message)
}

func (s *gameState) buildResetPayload() ([]byte, error) {
	s.mu.RLock()
	message := resetMessage{
		Type:    "reset",
		ResetAt: s.nextWorldResetAt.UnixMilli(),
	}
	s.mu.RUnlock()
	return marshalJSONPooled(message)
}

func (s *gameState) buildLeaderboardPayloadIfChanged() ([]byte, bool, error) {
	return s.payloadIfChanged(s.buildLeaderboardPayload, &s.lastLeaderboardPayload)
}

func (s *gameState) buildChatsPayloadIfChanged() ([]byte, bool, error) {
	return s.payloadIfChanged(s.buildChatsPayload, &s.lastChatsPayload)
}

func (s *gameState) buildConfigPayloadIfChanged() ([]byte, bool, error) {
	return s.payloadIfChanged(s.buildConfigPayload, &s.lastConfigPayload)
}

func (s *gameState) buildResetPayloadIfChanged() ([]byte, bool, error) {
	return s.payloadIfChanged(s.buildResetPayload, &s.lastResetPayload)
}

func (s *gameState) payloadIfChanged(builder func() ([]byte, error), last *[]byte) ([]byte, bool, error) {
	payload, err := builder()
	if err != nil {
		return nil, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if bytes.Equal(*last, payload) {
		return nil, false, nil
	}
	*last = append((*last)[:0], payload...)
	return payload, true, nil
}

// 최초 1회 Join 시 호출되는 단일 페이로드 생성 로직 (기존 로직 유지)
func (s *gameState) buildSnapshotPayload(playerID string, conn *wsConn) ([]byte, snapshotMessage, error) {
	s.mu.RLock()
	snapshotNow := time.Now()
	viewer, ok := s.players[playerID]
	if !ok {
		s.mu.RUnlock()
		return nil, snapshotMessage{}, fmt.Errorf("viewer not found")
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

	leeches := make([]*leechVirus, 0, len(s.leeches))
	for _, leech := range s.leeches {
		if !isWithinCullRange(centerX, centerY, leech.X, leech.Y, objectCullRange+leech.Size*2.4) {
			continue
		}
		leeches = append(leeches, cloneLeechVirus(leech, snapshotNow))
	}

	wormholes := make([]*wormhole, 0, len(s.wormholes))
	for _, hole := range s.wormholes {
		if !isWithinCullRange(centerX, centerY, hole.X, hole.Y, objectCullRange+hole.PullRange) {
			continue
		}
		copyHole := *hole
		wormholes = append(wormholes, &copyHole)
	}
	s.mu.RUnlock()

	message := s.buildDeltaSnapshotMessage(conn, players, foods, cacti, leeches, wormholes, true, true)
	tablePayload, err := conn.buildStringTablePayloadForPlayers(message.Players)
	if err != nil {
		return nil, snapshotMessage{}, err
	}
	return tablePayload, message, nil
}

func (s *gameState) buildDeltaSnapshotMessage(conn *wsConn, players []*player, foods []*food, cacti []*cactus, leeches []*leechVirus, wormholes []*wormhole, forceFull bool, includeFoods bool) snapshotMessage {
	playerDelta, foodDelta, cactusDelta, leechDelta, wormholeDelta, full := conn.computeSnapshotDelta(players, foods, cacti, leeches, wormholes, forceFull, includeFoods)
	return snapshotMessage{
		Type:               "snapshot",
		Full:               full,
		Players:            playerDelta.changed,
		RemovedPlayerIDs:   playerDelta.removedIDs,
		Foods:              foodDelta.changed,
		Cacti:              cactusDelta.changed,
		Leeches:            leechDelta.changed,
		Wormholes:          wormholeDelta.changed,
		RemovedFoodIDs:     foodDelta.removedIDs,
		RemovedCactusIDs:   cactusDelta.removedIDs,
		RemovedLeechIDs:    leechDelta.removedIDs,
		RemovedWormholeIDs: wormholeDelta.removedIDs,
	}
}

type objectDelta[T any] struct {
	changed    []*T
	removedIDs []string
}

func (c *wsConn) computeSnapshotDelta(players []*player, foods []*food, cacti []*cactus, leeches []*leechVirus, wormholes []*wormhole, forceFull bool, includeFoods bool) (objectDelta[player], objectDelta[food], objectDelta[cactus], objectDelta[leechVirus], objectDelta[wormhole], bool) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	full := forceFull || !c.cache.fullSent
	if full {
		c.cache.fullSent = true
	}

	playerDelta := diffPlayerSet(&c.cache.players, players, full)
	var foodDelta objectDelta[food]
	if includeFoods || full {
		foodDelta = diffFoodSet(&c.cache.foods, foods, full)
	}
	cactusDelta := diffCactusSet(&c.cache.cacti, cacti, full)
	leechDelta := diffLeechSet(&c.cache.leeches, leeches, full)
	wormholeDelta := diffWormholeSet(&c.cache.wormholes, wormholes, full)
	return playerDelta, foodDelta, cactusDelta, leechDelta, wormholeDelta, full
}

func diffPlayerSet(cache *map[string]uint64, current []*player, full bool) objectDelta[player] {
	previous := *cache
	next := acquireSignatureMap(len(current))
	changed := make([]*player, 0, len(current))
	if full {
		previous = nil
	}

	for _, item := range current {
		signature := playerSignature(item)
		next[item.ID] = signature
		if full || previous == nil || previous[item.ID] != signature {
			changed = append(changed, item)
		}
	}

	removed := diffRemovedIDs(previous, next)
	releaseSignatureMap(*cache)
	*cache = next
	return objectDelta[player]{changed: changed, removedIDs: removed}
}

func diffFoodSet(cache *map[string]uint64, current []*food, full bool) objectDelta[food] {
	previous := *cache
	next := acquireSignatureMap(len(current))
	changed := make([]*food, 0, len(current))
	if full {
		previous = nil
	}

	for _, item := range current {
		signature := foodSignature(item)
		next[item.ID] = signature
		if full || previous == nil || previous[item.ID] != signature {
			changed = append(changed, item)
		}
	}

	removed := diffRemovedIDs(previous, next)
	releaseSignatureMap(*cache)
	*cache = next
	return objectDelta[food]{changed: changed, removedIDs: removed}
}

func diffCactusSet(cache *map[string]uint64, current []*cactus, full bool) objectDelta[cactus] {
	previous := *cache
	next := acquireSignatureMap(len(current))
	changed := make([]*cactus, 0, len(current))
	if full {
		previous = nil
	}

	for _, item := range current {
		signature := cactusSignature(item)
		next[item.ID] = signature
		if full || previous == nil || previous[item.ID] != signature {
			changed = append(changed, item)
		}
	}

	removed := diffRemovedIDs(previous, next)
	releaseSignatureMap(*cache)
	*cache = next
	return objectDelta[cactus]{changed: changed, removedIDs: removed}
}

func diffLeechSet(cache *map[string]uint64, current []*leechVirus, full bool) objectDelta[leechVirus] {
	previous := *cache
	next := acquireSignatureMap(len(current))
	changed := make([]*leechVirus, 0, len(current))
	if full {
		previous = nil
	}

	for _, item := range current {
		signature := leechSignature(item)
		next[item.ID] = signature
		if full || previous == nil || previous[item.ID] != signature {
			changed = append(changed, item)
		}
	}

	removed := diffRemovedIDs(previous, next)
	releaseSignatureMap(*cache)
	*cache = next
	return objectDelta[leechVirus]{changed: changed, removedIDs: removed}
}

func diffWormholeSet(cache *map[string]uint64, current []*wormhole, full bool) objectDelta[wormhole] {
	previous := *cache
	next := acquireSignatureMap(len(current))
	changed := make([]*wormhole, 0, len(current))
	if full {
		previous = nil
	}

	for _, item := range current {
		signature := wormholeSignature(item)
		next[item.ID] = signature
		if full || previous == nil || previous[item.ID] != signature {
			changed = append(changed, item)
		}
	}

	removed := diffRemovedIDs(previous, next)
	releaseSignatureMap(*cache)
	*cache = next
	return objectDelta[wormhole]{changed: changed, removedIDs: removed}
}

func diffRemovedIDs(previous map[string]uint64, next map[string]uint64) []string {
	if len(previous) == 0 {
		return nil
	}
	removed := make([]string, 0)
	for id := range previous {
		if _, ok := next[id]; !ok {
			removed = append(removed, id)
		}
	}
	return removed
}

func playerSignature(item *player) uint64 {
	signature := uint64(1469598103934665603)
	signature = mixStringSignature(signature, item.OwnerID)
	signature = mixStringSignature(signature, item.Nickname)
	signature = mixStringSignature(signature, item.CellType)
	signature = mixStringSignature(signature, item.Ability)
	signature = mixStringSignature(signature, item.Color)
	signature = mixSignature(signature, quantizeSignature(item.X))
	signature = mixSignature(signature, quantizeSignature(item.Y))
	signature = mixSignature(signature, quantizeSignature(item.Mass))
	signature = mixSignature(signature, quantizeSignature(item.Radius))
	signature = mixSignature(signature, quantizeSignature(item.Scale))
	signature = mixSignature(signature, uint64(quantizeDurationMillis(item.CooldownRemaining)))
	signature = mixSignature(signature, uint64(quantizeDurationMillis(item.EffectRemaining)))
	signature = mixSignature(signature, uint64(quantizeDurationMillis(item.ShieldRemaining)))
	signature = mixSignature(signature, uint64(quantizeDurationMillis(item.ProbioticRemaining)))
	signature = mixSignature(signature, uint64(quantizeDurationMillis(item.SpeedBoostRemaining)))
	signature = mixSignature(signature, uint64(quantizeDurationMillis(item.RespawnRemaining)))
	signature = mixSignature(signature, uint64(item.Coins))
	signature = mixSignature(signature, uint64(upgradeBits(item.Upgrades)))
	if item.IsBot {
		signature = mixSignature(signature, 1)
	}
	return signature
}

func foodSignature(item *food) uint64 {
	signature := uint64(1469598103934665603)
	signature = mixSignature(signature, quantizeSignature(item.X))
	signature = mixSignature(signature, quantizeSignature(item.Y))
	signature = mixSignature(signature, quantizeSignature(item.Radius))
	signature = mixSignature(signature, quantizeSignature(item.Value))
	return mixStringSignature(signature, item.Kind)
}

func cactusSignature(item *cactus) uint64 {
	signature := uint64(1469598103934665603)
	signature = mixSignature(signature, quantizeSignature(item.X))
	signature = mixSignature(signature, quantizeSignature(item.Y))
	signature = mixSignature(signature, quantizeSignature(item.Size))
	return mixSignature(signature, quantizeSignature(item.Height))
}

func leechSignature(item *leechVirus) uint64 {
	signature := uint64(1469598103934665603)
	signature = mixSignature(signature, quantizeSignature(item.X))
	signature = mixSignature(signature, quantizeSignature(item.Y))
	signature = mixSignature(signature, quantizeSignature(item.Size))
	signature = mixSignature(signature, quantizeSignature(item.Mass))
	signature = mixSignature(signature, quantizeSignature(item.Angle))
	signature = mixStringSignature(signature, item.AttachedTo)
	signature = mixSignature(signature, uint64(quantizeDurationMillis(item.AttachedRemaining)))
	signature = mixSignature(signature, uint64(quantizeDurationMillis(item.BurstRemaining)))
	return signature
}

func wormholeSignature(item *wormhole) uint64 {
	signature := uint64(1469598103934665603)
	signature = mixSignature(signature, quantizeSignature(item.X))
	signature = mixSignature(signature, quantizeSignature(item.Y))
	signature = mixSignature(signature, quantizeSignature(item.Radius))
	signature = mixSignature(signature, quantizeSignature(item.PullRange))
	signature = mixStringSignature(signature, item.Kind)
	return mixStringSignature(signature, item.PairID)
}

func quantizeSignature(value float64) uint64 {
	return uint64(int64(math.Round(value * 100)))
}

func mixSignature(signature uint64, value uint64) uint64 {
	return (signature ^ value) * 1099511628211
}

func mixStringSignature(signature uint64, value string) uint64 {
	for i := 0; i < len(value); i++ {
		signature = mixSignature(signature, uint64(value[i]))
	}
	return signature
}

func upgradeBits(upgrades upgradeState) byte {
	var bits byte
	if upgrades.Classic {
		bits |= 1 << 0
	}
	if upgrades.Blink {
		bits |= 1 << 1
	}
	if upgrades.Giant {
		bits |= 1 << 2
	}
	if upgrades.Shield {
		bits |= 1 << 3
	}
	if upgrades.Magnet {
		bits |= 1 << 4
	}
	if upgrades.Divider {
		bits |= 1 << 5
	}
	return bits
}

func writeBinarySnapshot(conn *wsConn, message snapshotMessage) error {
	buf := acquireSnapshotBuffer()
	defer releaseSnapshotBuffer(buf)

	if err := encodeSnapshotBinary(buf, conn, message); err != nil {
		return err
	}
	return conn.writeBinary(buf.Bytes())
}

func encodeSnapshotBinary(buf *bytes.Buffer, conn *wsConn, message snapshotMessage) error {
	buf.Reset()
	buf.Write([]byte{'S', 'N', 'P', '2'})

	var flags byte
	if message.Full {
		flags |= 1
	}
	buf.WriteByte(flags)

	counts := []int{
		len(message.Players),
		len(message.RemovedPlayerIDs),
		len(message.Foods),
		len(message.RemovedFoodIDs),
		len(message.Cacti),
		len(message.RemovedCactusIDs),
		len(message.Leeches),
		len(message.RemovedLeechIDs),
		len(message.Wormholes),
		len(message.RemovedWormholeIDs),
	}
	for _, count := range counts {
		if err := writeU16(buf, count); err != nil {
			return err
		}
	}

	for _, player := range message.Players {
		if err := writePlayerBinary(buf, conn, player); err != nil {
			return err
		}
	}
	for _, id := range message.RemovedPlayerIDs {
		if err := writeStringBinary(buf, id); err != nil {
			return err
		}
	}
	for _, food := range message.Foods {
		if err := writeFoodBinary(buf, food); err != nil {
			return err
		}
	}
	for _, id := range message.RemovedFoodIDs {
		if err := writeStringBinary(buf, id); err != nil {
			return err
		}
	}
	for _, cactus := range message.Cacti {
		if err := writeCactusBinary(buf, cactus); err != nil {
			return err
		}
	}
	for _, id := range message.RemovedCactusIDs {
		if err := writeStringBinary(buf, id); err != nil {
			return err
		}
	}
	for _, leech := range message.Leeches {
		if err := writeLeechBinary(buf, leech); err != nil {
			return err
		}
	}
	for _, id := range message.RemovedLeechIDs {
		if err := writeStringBinary(buf, id); err != nil {
			return err
		}
	}
	for _, wormhole := range message.Wormholes {
		if err := writeWormholeBinary(buf, wormhole); err != nil {
			return err
		}
	}
	for _, id := range message.RemovedWormholeIDs {
		if err := writeStringBinary(buf, id); err != nil {
			return err
		}
	}

	return nil
}

func writePlayerBinary(buf *bytes.Buffer, conn *wsConn, player *player) error {
	if err := writeStringBinary(buf, player.ID); err != nil {
		return err
	}
	writeU16Unsafe(buf, conn.stringID("owner", player.OwnerID))
	writeU16Unsafe(buf, conn.stringID("nickname", player.Nickname))
	writeU16Unsafe(buf, conn.stringID("cellType", player.CellType))
	writeU16Unsafe(buf, conn.stringID("ability", player.Ability))
	writeU16Unsafe(buf, conn.stringID("color", player.Color))
	writeQuantU16(buf, player.X, coordQuantScale)
	writeQuantU16(buf, player.Y, coordQuantScale)
	writeQuantU32(buf, player.Mass, massQuantScale)
	writeQuantU16(buf, player.Radius, radiusQuantScale)
	writeQuantU16(buf, player.Scale, scaleQuantScale)
	if player.IsBot {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	if err := writeU16(buf, player.Coins); err != nil {
		return err
	}
	buf.WriteByte(upgradeBits(player.Upgrades))
	writeU16Unsafe(buf, quantizeDurationMillis(player.CooldownRemaining))
	writeU16Unsafe(buf, quantizeDurationMillis(player.EffectRemaining))
	writeU16Unsafe(buf, quantizeDurationMillis(player.ShieldRemaining))
	writeU16Unsafe(buf, quantizeDurationMillis(player.ProbioticRemaining))
	writeU16Unsafe(buf, quantizeDurationMillis(player.SpeedBoostRemaining))
	writeU16Unsafe(buf, quantizeDurationMillis(player.RespawnRemaining))
	return nil
}

func acquireSnapshotBuffer() *bytes.Buffer {
	buf := snapshotBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func releaseSnapshotBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	if buf.Cap() > maxPooledBufferCap {
		return
	}
	buf.Reset()
	snapshotBufferPool.Put(buf)
}

func acquireSignatureMap(expectedSize int) map[string]uint64 {
	m := signatureMapPool.Get().(map[string]uint64)
	clear(m)
	return m
}

func releaseSignatureMap(m map[string]uint64) {
	if m == nil {
		return
	}
	clear(m)
	signatureMapPool.Put(m)
}

func acquireJSONBuffer() *bytes.Buffer {
	buf := jsonBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func releaseJSONBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	if buf.Cap() > 64*1024 {
		return
	}
	buf.Reset()
	jsonBufferPool.Put(buf)
}

func marshalJSONPooled(value any) ([]byte, error) {
	buf := acquireJSONBuffer()
	defer releaseJSONBuffer(buf)

	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}

	payload := append([]byte(nil), buf.Bytes()...)
	if len(payload) > 0 && payload[len(payload)-1] == '\n' {
		payload = payload[:len(payload)-1]
	}
	return payload, nil
}

func (c *wsConn) buildStringTablePayloadForPlayers(players []*player) ([]byte, error) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	msg := stringTableMessage{Type: "stringTable"}
	for _, player := range players {
		c.appendStringUpdate(&msg.OwnerIDs, &c.strings.ownerIDs, &c.strings.nextOwnerID, player.OwnerID)
		c.appendStringUpdate(&msg.Nicknames, &c.strings.nicknames, &c.strings.nextNicknameID, player.Nickname)
		c.appendStringUpdate(&msg.CellTypes, &c.strings.cellTypes, &c.strings.nextCellTypeID, player.CellType)
		c.appendStringUpdate(&msg.AbilityNames, &c.strings.abilities, &c.strings.nextAbilityID, player.Ability)
		c.appendStringUpdate(&msg.Colors, &c.strings.colors, &c.strings.nextColorID, player.Color)
	}

	if len(msg.OwnerIDs) == 0 && len(msg.Nicknames) == 0 && len(msg.CellTypes) == 0 && len(msg.AbilityNames) == 0 && len(msg.Colors) == 0 {
		return nil, nil
	}
	return marshalJSONPooled(msg)
}

func (c *wsConn) appendStringUpdate(entries *[]stringTableEntry, table *map[string]uint16, nextID *uint16, value string) {
	if *table == nil {
		*table = make(map[string]uint16)
	}
	if _, exists := (*table)[value]; exists {
		return
	}
	*nextID = *nextID + 1
	(*table)[value] = *nextID
	*entries = append(*entries, stringTableEntry{ID: *nextID, Value: value})
}

func (c *wsConn) stringID(kind string, value string) uint16 {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	switch kind {
	case "owner":
		return ensureStringID(&c.strings.ownerIDs, &c.strings.nextOwnerID, value)
	case "nickname":
		return ensureStringID(&c.strings.nicknames, &c.strings.nextNicknameID, value)
	case "cellType":
		return ensureStringID(&c.strings.cellTypes, &c.strings.nextCellTypeID, value)
	case "ability":
		return ensureStringID(&c.strings.abilities, &c.strings.nextAbilityID, value)
	case "color":
		return ensureStringID(&c.strings.colors, &c.strings.nextColorID, value)
	default:
		return 0
	}
}

func ensureStringID(table *map[string]uint16, nextID *uint16, value string) uint16 {
	if *table == nil {
		*table = make(map[string]uint16)
	}
	if id, exists := (*table)[value]; exists {
		return id
	}
	*nextID = *nextID + 1
	(*table)[value] = *nextID
	return *nextID
}

func writeFoodBinary(buf *bytes.Buffer, item *food) error {
	if err := writeStringBinary(buf, item.ID); err != nil {
		return err
	}
	writeQuantU16(buf, item.X, coordQuantScale)
	writeQuantU16(buf, item.Y, coordQuantScale)
	writeQuantU16(buf, item.Radius, radiusQuantScale)
	writeQuantU16(buf, item.Value, valueQuantScale)
	return writeStringBinary(buf, item.Kind)
}

func writeCactusBinary(buf *bytes.Buffer, item *cactus) error {
	if err := writeStringBinary(buf, item.ID); err != nil {
		return err
	}
	writeQuantU16(buf, item.X, coordQuantScale)
	writeQuantU16(buf, item.Y, coordQuantScale)
	writeQuantU16(buf, item.Size, radiusQuantScale)
	writeQuantU16(buf, item.Height, radiusQuantScale)
	return nil
}

func writeLeechBinary(buf *bytes.Buffer, item *leechVirus) error {
	if err := writeStringBinary(buf, item.ID); err != nil {
		return err
	}
	writeQuantU16(buf, item.X, coordQuantScale)
	writeQuantU16(buf, item.Y, coordQuantScale)
	writeQuantU16(buf, item.Size, radiusQuantScale)
	writeQuantU16(buf, item.Mass, massQuantScale)
	writeQuantU16(buf, item.Angle+math.Pi*2, scaleQuantScale)
	if err := writeStringBinary(buf, item.AttachedTo); err != nil {
		return err
	}
	writeU16Unsafe(buf, quantizeDurationMillis(item.AttachedRemaining))
	writeU16Unsafe(buf, quantizeDurationMillis(item.BurstRemaining))
	return nil
}

func writeWormholeBinary(buf *bytes.Buffer, item *wormhole) error {
	if err := writeStringBinary(buf, item.ID); err != nil {
		return err
	}
	if err := writeStringBinary(buf, item.Kind); err != nil {
		return err
	}
	if err := writeStringBinary(buf, item.PairID); err != nil {
		return err
	}
	writeQuantU16(buf, item.X, coordQuantScale)
	writeQuantU16(buf, item.Y, coordQuantScale)
	writeQuantU16(buf, item.Radius, radiusQuantScale)
	writeQuantU16(buf, item.PullRange, radiusQuantScale)
	return nil
}

func writeStringBinary(buf *bytes.Buffer, value string) error {
	if err := writeU16(buf, len(value)); err != nil {
		return err
	}
	_, err := buf.WriteString(value)
	return err
}

func writeU16(buf *bytes.Buffer, value int) error {
	if value < 0 || value > math.MaxUint16 {
		return fmt.Errorf("value out of uint16 range: %d", value)
	}
	writeU16Unsafe(buf, uint16(value))
	return nil
}

func writeU16Unsafe(buf *bytes.Buffer, value uint16) {
	var raw [2]byte
	binary.LittleEndian.PutUint16(raw[:], value)
	buf.Write(raw[:])
}

func writeU32(buf *bytes.Buffer, value int64) {
	if value < 0 {
		value = 0
	}
	if value > math.MaxUint32 {
		value = math.MaxUint32
	}
	var raw [4]byte
	binary.LittleEndian.PutUint32(raw[:], uint32(value))
	buf.Write(raw[:])
}

func writeQuantU16(buf *bytes.Buffer, value float64, scale float64) {
	writeU16Unsafe(buf, quantizeToU16(value, scale))
}

func writeQuantU32(buf *bytes.Buffer, value float64, scale float64) {
	var raw [4]byte
	binary.LittleEndian.PutUint32(raw[:], quantizeToU32(value, scale))
	buf.Write(raw[:])
}

func quantizeToU16(value float64, scale float64) uint16 {
	scaled := math.Round(math.Max(0, value) * scale)
	if scaled > math.MaxUint16 {
		scaled = math.MaxUint16
	}
	return uint16(scaled)
}

func quantizeToU32(value float64, scale float64) uint32 {
	scaled := math.Round(math.Max(0, value) * scale)
	if scaled > math.MaxUint32 {
		scaled = math.MaxUint32
	}
	return uint32(scaled)
}

func quantizeDurationMillis(value int64) uint16 {
	if value <= 0 {
		return 0
	}
	scaled := (value + durationQuantStepMs - 1) / durationQuantStepMs
	if scaled > math.MaxUint16 {
		scaled = math.MaxUint16
	}
	return uint16(scaled)
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

func (s *gameState) seedLeeches() {
	s.reconcileLeechesLocked()
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

func (s *gameState) reconcileLeechesLocked() {
	target := s.config.LeechCount
	if target < 0 {
		target = 0
	}

	for len(s.leeches) > target {
		s.leeches[len(s.leeches)-1] = nil
		s.leeches = s.leeches[:len(s.leeches)-1]
	}
	for len(s.leeches) < target {
		s.leeches = append(s.leeches, createLeechVirus(s.worldSize()))
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
