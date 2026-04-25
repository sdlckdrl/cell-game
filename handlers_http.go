package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
	startRadius := massToRadius(playerStartMass)

	s.mu.Lock()
	spawnX, spawnY := s.findSafeSpawnLocked(startRadius)
	p := &player{
		ID:        playerID,
		SessionID: sessionID,
		OwnerID:   playerID,
		Nickname:  nickname,
		CellType:  cellType,
		Ability:   abilityLabel(cellType),
		X:         spawnX,
		Y:         spawnY,
		Mass:      playerStartMass,
		Radius:    startRadius,
		Scale:     1,
		Color:     randomColor(),
		LastSeen:  time.Now(),
	}
	p.Energy = 4000
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
	nextConfig := s.config
	if req.MinimumPlayers != nil {
		nextConfig.MinimumPlayers = int(math.Max(0, float64(*req.MinimumPlayers)))
	}
	if req.ProbioticCount != nil {
		nextConfig.ProbioticCount = int(math.Max(0, float64(*req.ProbioticCount)))
	}
	if req.CactusCount != nil {
		nextConfig.CactusCount = int(math.Max(0, float64(*req.CactusCount)))
	}
	if req.WormholePairs != nil {
		nextConfig.WormholePairs = int(math.Max(0, float64(*req.WormholePairs)))
	}
	if req.CactusRelocateSeconds != nil {
		nextConfig.CactusRelocateSeconds = int(math.Max(0, float64(*req.CactusRelocateSeconds)))
	}
	if req.WormholeRelocateSeconds != nil {
		nextConfig.WormholeRelocateSeconds = int(math.Max(0, float64(*req.WormholeRelocateSeconds)))
	}
	if req.LeechCount != nil {
		nextConfig.LeechCount = int(math.Max(0, float64(*req.LeechCount)))
	}
	if req.LeechAttachSeconds != nil {
		nextConfig.LeechAttachSeconds = *req.LeechAttachSeconds
	}
	if req.LeechDrainPercent != nil {
		nextConfig.LeechDrainPercent = *req.LeechDrainPercent
	}
	if req.LeechFedCooldownSeconds != nil {
		nextConfig.LeechFedCooldownSeconds = *req.LeechFedCooldownSeconds
	}
	if req.LeechMaxMass != nil {
		nextConfig.LeechMaxMass = *req.LeechMaxMass
	}
	if req.LeechSwimSpeed != nil {
		nextConfig.LeechSwimSpeed = *req.LeechSwimSpeed
	}
	if req.LeechMaxSizeScale != nil {
		nextConfig.LeechMaxSizeScale = *req.LeechMaxSizeScale
	}
	if req.WorldSize != nil {
		nextConfig.WorldSize = sanitizeWorldSize(*req.WorldSize)
	}
	if req.BaseSpeed != nil {
		nextConfig.BaseSpeed = math.Max(50, *req.BaseSpeed)
	}
	if req.SpeedDivisor != nil {
		nextConfig.SpeedDivisor = math.Max(1, *req.SpeedDivisor)
	}
	if req.MinimumSpeed != nil {
		nextConfig.MinimumSpeed = math.Max(10, *req.MinimumSpeed)
	}

	nextConfig = normalizeRuntimeConfig(nextConfig)
	if err := saveRuntimeConfig(nextConfig); err != nil {
		s.mu.Unlock()
		http.Error(w, "failed to persist config", http.StatusInternalServerError)
		return
	}

	s.config = nextConfig
	s.clampWorldObjectsLocked()
	s.reconcileProbioticsLocked()
	s.reconcileCactiLocked()
	s.reconcileLeechesLocked()
	s.reconcileWormholesLocked()
	s.reconcileBotsLocked()
	s.lastProbioticSpawn = time.Now()
	s.lastCactusRelocation = time.Now()
	s.lastWormholeRelocation = time.Now()
	config := s.config
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, config)
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	root, err := appBaseDir()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	fullPath, ok := resolvePublicStaticPath(root, r.URL.Path)
	if !ok {
		http.NotFound(w, r)
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

func resolvePublicStaticPath(root, requestPath string) (string, bool) {
	if requestPath == "" || requestPath == "/" {
		requestPath = "/index.html"
	}

	relativePath := strings.TrimPrefix(requestPath, "/")
	cleanPath := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanPath == "." || cleanPath == "" {
		cleanPath = "index.html"
	}
	if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", false
	}

	cleanSlashPath := filepath.ToSlash(cleanPath)
	if !isPublicStaticPath(cleanSlashPath) {
		return "", false
	}

	fullPath := filepath.Join(root, cleanPath)
	relativeToRoot, err := filepath.Rel(root, fullPath)
	if err != nil {
		return "", false
	}
	if relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return "", false
	}

	return fullPath, true
}

func isPublicStaticPath(path string) bool {
	switch path {
	case "index.html", "privacy.html", "styles.css":
		return true
	}

	if !strings.HasPrefix(path, "src/") {
		return false
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".css", ".map", ".png", ".jpg", ".jpeg", ".svg", ".webp", ".ico":
		return true
	default:
		return false
	}
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
	if cwd, err := os.Getwd(); err == nil {
		if _, statErr := os.Stat(filepath.Join(cwd, "index.html")); statErr == nil {
			return cwd, nil
		}
	}

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
		ProbioticCount:          probioticTarget,
		CactusCount:             cactusTarget,
		WormholePairs:           defaultWormholePairs,
		CactusRelocateSeconds:   0,
		WormholeRelocateSeconds: 0,
		LeechCount:              leechTarget,
		LeechAttachSeconds:      int(leechAttachDuration / time.Second),
		LeechDrainPercent:       leechDrainFraction * 100,
		LeechFedCooldownSeconds: int(leechFedCooldown / time.Second),
		LeechMaxMass:            leechMaxMass,
		LeechSwimSpeed:          leechSwimSpeed,
		LeechMaxSizeScale:       leechMaxSizeScale,
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
	config.ProbioticCount = int(math.Max(0, float64(config.ProbioticCount)))
	config.CactusCount = int(math.Max(0, float64(config.CactusCount)))
	config.WormholePairs = int(math.Max(0, float64(config.WormholePairs)))
	config.CactusRelocateSeconds = int(math.Max(0, float64(config.CactusRelocateSeconds)))
	config.WormholeRelocateSeconds = int(math.Max(0, float64(config.WormholeRelocateSeconds)))
	config.LeechCount = int(clamp(float64(config.LeechCount), 0, 80))
	config.LeechAttachSeconds = int(clamp(float64(config.LeechAttachSeconds), 5, 120))
	config.LeechDrainPercent = math.Max(0, config.LeechDrainPercent)
	config.LeechFedCooldownSeconds = int(clamp(float64(config.LeechFedCooldownSeconds), 0, 60))
	if config.LeechMaxMass <= 0 {
		config.LeechMaxMass = leechMaxMass
	}
	config.LeechSwimSpeed = clamp(config.LeechSwimSpeed, 5, 180)
	if config.LeechMaxSizeScale <= 0 {
		config.LeechMaxSizeScale = leechMaxSizeScale
	}
	config.LeechMaxSizeScale = math.Max(1.0, config.LeechMaxSizeScale)
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
