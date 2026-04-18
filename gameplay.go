package main

import (
	"math"
	mathrand "math/rand"
	"sort"
	"time"
)

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

			if canEatPlayer(now, a, b, gap) {
				// A가 B를 포식
				a.Mass += b.Mass * 0.85
				a.Radius = massToRadius(a.Mass)
				attackerOwnerID := a.OwnerID
				if attackerOwnerID == "" {
					attackerOwnerID = a.ID
				}
				s.awardCoinsLocked(attackerOwnerID, int(math.Max(1, math.Round(b.Mass/42))))
				s.handleConsumedPlayerLocked(b)
			} else if canEatPlayer(now, b, a, gap) {
				// B가 A를 포식
				b.Mass += a.Mass * 0.85
				b.Radius = massToRadius(b.Mass)
				attackerOwnerID := b.OwnerID
				if attackerOwnerID == "" {
					attackerOwnerID = b.ID
				}
				s.awardCoinsLocked(attackerOwnerID, int(math.Max(1, math.Round(a.Mass/42))))
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

func (s *gameState) resolvePlayerEatingV2(now time.Time) {
	players := make([]*player, 0, len(s.players))
	for _, p := range s.players {
		players = append(players, p)
	}

	worldSize := s.worldSize()
	for i := 0; i < len(players); i++ {
		for j := i + 1; j < len(players); j++ {
			a := players[i]
			b := players[j]

			if _, ok := s.players[a.ID]; !ok {
				continue
			}
			if _, ok := s.players[b.ID]; !ok {
				continue
			}
			if isRespawningAt(now, a) || isRespawningAt(now, b) {
				continue
			}
			if a.Mass <= 0 || b.Mass <= 0 || a.Radius <= 0 || b.Radius <= 0 {
				continue
			}
			if a.OwnerID != "" && a.OwnerID == b.OwnerID {
				continue
			}

			gap := distance(a.X, a.Y, b.X, b.Y)
			switch {
			case canEatPlayer(now, a, b, gap):
				a.Mass += b.Mass * 0.85
				a.Radius = massToRadius(a.Mass)
				attackerOwnerID := a.OwnerID
				if attackerOwnerID == "" {
					attackerOwnerID = a.ID
				}
				s.awardCoinsLocked(attackerOwnerID, int(math.Max(1, math.Round(b.Mass/42))))
				s.handleConsumedPlayerLocked(b)
			case canEatPlayer(now, b, a, gap):
				b.Mass += a.Mass * 0.85
				b.Radius = massToRadius(b.Mass)
				attackerOwnerID := b.OwnerID
				if attackerOwnerID == "" {
					attackerOwnerID = b.ID
				}
				s.awardCoinsLocked(attackerOwnerID, int(math.Max(1, math.Round(a.Mass/42))))
				s.handleConsumedPlayerLocked(a)
			default:
				applySoftCollisionResponse(a, b, gap, worldSize, now)
			}
		}
	}
}

func applySoftCollisionResponse(a, b *player, gap, worldSize float64, now time.Time) {
	if gap <= 0 {
		return
	}

	radiusA := currentRadius(a)
	radiusB := currentRadius(b)
	minGap := radiusA + radiusB
	if gap >= minGap {
		return
	}

	combatA := effectiveCombatMass(a)
	combatB := effectiveCombatMass(b)
	lighter := math.Min(combatA, combatB)
	heavier := math.Max(combatA, combatB)
	if lighter <= 0 {
		return
	}

	similarityRatio := heavier / lighter
	hasDefensiveState :=
		(a.CellType == "shield" && now.Before(a.EffectUntil)) ||
			(b.CellType == "shield" && now.Before(b.EffectUntil)) ||
			(a.CellType == "giant" && now.Before(a.EffectUntil)) ||
			(b.CellType == "giant" && now.Before(b.EffectUntil)) ||
			now.Before(a.ShieldUntil) ||
			now.Before(b.ShieldUntil)

	pushStrength := 0.0
	switch {
	case hasDefensiveState:
		pushStrength = 0.5
	case similarityRatio <= 1.18:
		pushStrength = 0.12
	default:
		return
	}

	overlap := minGap - gap
	totalMass := a.Mass + b.Mass
	if totalMass <= 0 {
		totalMass = 1
	}

	pushA := (b.Mass / totalMass) * overlap * pushStrength
	pushB := (a.Mass / totalMass) * overlap * pushStrength
	dirX := (a.X - b.X) / gap
	dirY := (a.Y - b.Y) / gap

	a.X = clamp(a.X+dirX*pushA, radiusA, worldSize-radiusA)
	a.Y = clamp(a.Y+dirY*pushA, radiusA, worldSize-radiusA)
	b.X = clamp(b.X-dirX*pushB, radiusB, worldSize-radiusB)
	b.Y = clamp(b.Y-dirY*pushB, radiusB, worldSize-radiusB)
}

// ----------------------------------------------------
// 최적화된 브로드캐스트 함수 (Spatial Partitioning)
// ----------------------------------------------------

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
	p.RespawnAt = time.Time{}
	p.IsAbilityActive = false
}

func scheduleRespawnPlayerLocked(p *player, now time.Time) {
	p.Mass = 0
	p.Radius = 0
	p.Scale = 1
	p.Direction = direction{}
	p.CooldownUntil = time.Time{}
	p.EffectUntil = time.Time{}
	p.ProbioticUntil = time.Time{}
	p.ShieldUntil = time.Time{}
	p.SpeedBoostUntil = time.Time{}
	p.CactusUntil = time.Time{}
	p.PortalUntil = time.Time{}
	p.MergeReadyAt = time.Time{}
	p.IsAbilityActive = false
	p.RespawnAt = now.Add(5 * time.Second)
}

func isRespawningAt(now time.Time, p *player) bool {
	return !p.RespawnAt.IsZero() && now.Before(p.RespawnAt)
}

func clonePlayer(p *player) *player {
	now := time.Now()

	// 1. 밀리초(ms) 단위로 변환한 값을 먼저 변수에 담습니다.
	cooldownRemainingMs := int64(maxDuration(0, p.CooldownUntil.Sub(now)) / time.Millisecond)
	effectRemainingMs := int64(maxDuration(0, p.EffectUntil.Sub(now)) / time.Millisecond)
	shieldRemainingMs := int64(maxDuration(0, p.ShieldUntil.Sub(now)) / time.Millisecond)
	probioticRemainingMs := int64(maxDuration(0, p.ProbioticUntil.Sub(now)) / time.Millisecond)
	speedBoostRemainingMs := int64(maxDuration(0, p.SpeedBoostUntil.Sub(now)) / time.Millisecond)
	respawnRemainingMs := int64(maxDuration(0, p.RespawnAt.Sub(now)) / time.Millisecond)

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
		ID:                  p.ID,
		OwnerID:             p.OwnerID, // 이전에 GetOwnerID()로 통합하셨다면 p.GetOwnerID() 사용
		Nickname:            p.Nickname,
		CellType:            p.CellType,
		Ability:             p.Ability,
		X:                   p.X,
		Y:                   p.Y,
		Mass:                p.Mass,
		Radius:              p.Radius,
		Scale:               p.Scale,
		Color:               p.Color,
		IsBot:               p.IsBot,
		Coins:               p.Coins,
		Upgrades:            p.Upgrades,
		CooldownRemaining:   cooldownRemainingMs, // ✅ 수정된 변수 적용
		EffectRemaining:     effectRemainingMs,   // ✅ 수정된 변수 적용
		ShieldRemaining:     shieldRemainingMs,
		ProbioticRemaining:  probioticRemainingMs,
		SpeedBoostRemaining: speedBoostRemainingMs,
		RespawnRemaining:    respawnRemainingMs,
	}
}

func (s *gameState) ownedPlayersLocked(ownerID string) []*player {
	return fragmentsForOwner(buildOwnerFragmentsIndex(s.players), ownerID)
}

func (s *gameState) ownerCenterLocked(ownerID string) (float64, float64) {
	return ownerCenterFromFragments(fragmentsForOwner(buildOwnerFragmentsIndex(s.players), ownerID), s.worldSize())
}

func ownerIDOf(p *player) string {
	if p.OwnerID != "" {
		return p.OwnerID
	}
	return p.ID
}

func buildOwnerFragmentsIndex(players map[string]*player) map[string][]*player {
	index := make(map[string][]*player)
	for _, p := range players {
		ownerID := ownerIDOf(p)
		index[ownerID] = append(index[ownerID], p)
	}
	return index
}

func fragmentsForOwner(index map[string][]*player, ownerID string) []*player {
	if fragments, ok := index[ownerID]; ok {
		return fragments
	}
	return []*player{}
}

func ownerCenterFromFragments(fragments []*player, worldSize float64) (float64, float64) {
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

func primaryOwnedFragmentFromFragments(ownerID string, fragments []*player) *player {
	var fallback *player
	for _, fragment := range fragments {
		if fragment.ID == ownerID {
			return fragment
		}
		if fallback == nil || fragment.Mass > fallback.Mass {
			fallback = fragment
		}
	}
	return fallback
}

func (s *gameState) handleConsumedPlayerLocked(victim *player) {
	ownerID := victim.OwnerID
	if ownerID == "" {
		ownerID = victim.ID
	}
	fragments := s.ownedPlayersLocked(ownerID)
	if len(fragments) <= 1 {
		scheduleRespawnPlayerLocked(victim, time.Now())
		return
	}

	if victim.ID != ownerID {
		delete(s.players, victim.ID)
		return
	}

	successor := largestOwnedFragmentExcluding(fragments, victim.ID)
	if successor == nil {
		scheduleRespawnPlayerLocked(victim, time.Now())
		return
	}

	victim.X = successor.X
	victim.Y = successor.Y
	victim.Mass = successor.Mass
	victim.Radius = successor.Radius
	victim.Scale = successor.Scale
	victim.Direction = successor.Direction
	victim.Coins = successor.Coins
	victim.Upgrades = successor.Upgrades
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
	now := time.Now()

	for _, p := range players {
		if isRespawningAt(now, p) {
			continue
		}
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

func (s *gameState) forceMergeOwnerLocked(ownerID string) {
	fragments := s.ownedPlayersLocked(ownerID)
	for len(fragments) > 1 {
		s.mergeOwnedPairLocked(ownerID, fragments[0], fragments[1])
		fragments = s.ownedPlayersLocked(ownerID)
	}
}

func upgradeEnabledForCellType(upgrades upgradeState, cellType string) bool {
	switch cellType {
	case "classic":
		return upgrades.Classic
	case "blink":
		return upgrades.Blink
	case "giant":
		return upgrades.Giant
	case "shield":
		return upgrades.Shield
	case "magnet":
		return upgrades.Magnet
	case "divider":
		return upgrades.Divider
	default:
		return false
	}
}

func setUpgradeForCellType(upgrades upgradeState, cellType string, enabled bool) upgradeState {
	switch cellType {
	case "classic":
		upgrades.Classic = enabled
	case "blink":
		upgrades.Blink = enabled
	case "giant":
		upgrades.Giant = enabled
	case "shield":
		upgrades.Shield = enabled
	case "magnet":
		upgrades.Magnet = enabled
	case "divider":
		upgrades.Divider = enabled
	}
	return upgrades
}

func (s *gameState) ownerProgressLocked(ownerID string) (int, upgradeState) {
	fragments := s.ownedPlayersLocked(ownerID)
	if len(fragments) == 0 {
		return 0, upgradeState{}
	}
	return fragments[0].Coins, fragments[0].Upgrades
}

func (s *gameState) syncOwnerProgressLocked(ownerID string, coins int, upgrades upgradeState) {
	for _, fragment := range s.ownedPlayersLocked(ownerID) {
		fragment.Coins = coins
		fragment.Upgrades = upgrades
	}
}

func (s *gameState) awardCoinsLocked(ownerID string, gain int) {
	if gain <= 0 {
		return
	}
	coins, upgrades := s.ownerProgressLocked(ownerID)
	s.syncOwnerProgressLocked(ownerID, coins+gain, upgrades)
}

func (s *gameState) handleUpgradePurchaseLocked(p *player, requested string) bool {
	if p == nil {
		return false
	}
	ownerID := p.OwnerID
	if ownerID == "" {
		ownerID = p.ID
	}
	cellType := sanitizeCellType(requested)
	if cellType != p.CellType {
		cellType = p.CellType
	}
	coins, upgrades := s.ownerProgressLocked(ownerID)
	if upgradeEnabledForCellType(upgrades, cellType) || coins < upgradeCost {
		return false
	}
	upgrades = setUpgradeForCellType(upgrades, cellType, true)
	s.syncOwnerProgressLocked(ownerID, coins-upgradeCost, upgrades)
	return true
}

func (s *gameState) tryUseAbility(p *player) {
	now := time.Now()
	worldSize := s.worldSize()
	if now.Before(p.CooldownUntil) {
		return
	}
	upgraded := upgradeEnabledForCellType(p.Upgrades, p.CellType)

	switch p.CellType {
	case "classic":
		// Classic uses a held overclock state driven by Energy, not a server cooldown.
		return
	case "blink":
		blinkDistance := 180.0
		if upgraded {
			blinkDistance = 340.0
		}
		length := math.Hypot(p.Direction.X, p.Direction.Y)
		if length < 0.1 {
			return
		}
		p.X = clamp(p.X+(p.Direction.X/length)*blinkDistance, p.Radius, worldSize-p.Radius)
		p.Y = clamp(p.Y+(p.Direction.Y/length)*blinkDistance, p.Radius, worldSize-p.Radius)
		p.CooldownUntil = now.Add(6 * time.Second)
	case "giant":
		duration := 5 * time.Second
		p.EffectUntil = now.Add(duration)
		p.CooldownUntil = now.Add(10 * time.Second)
	case "shield":
		duration := 3 * time.Second
		if upgraded {
			duration = 5 * time.Second
		}
		p.EffectUntil = now.Add(duration)
		p.CooldownUntil = now.Add(12 * time.Second)
	case "magnet":
		duration := 4 * time.Second
		if upgraded {
			duration = 10 * time.Second
		}
		p.EffectUntil = now.Add(duration)
		p.CooldownUntil = now.Add(9 * time.Second)
	case "divider":
		s.tryDividerAbilityLocked(p, now)
	default:
		p.CooldownUntil = now.Add(2 * time.Second)
	}
}

func (s *gameState) tryUpgradeMergeLocked(p *player) {
	if p == nil || p.CellType != "divider" || !p.Upgrades.Divider {
		return
	}
	ownerID := p.OwnerID
	if ownerID == "" {
		ownerID = p.ID
	}
	if len(s.ownedPlayersLocked(ownerID)) < 2 {
		return
	}
	s.forceMergeOwnerLocked(ownerID)
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
			ID:              randomID(),
			SessionID:       "",
			OwnerID:         ownerID,
			Nickname:        fragment.Nickname,
			CellType:        fragment.CellType,
			Ability:         fragment.Ability,
			X:               clamp(fragment.X-dir.X*(fragment.Radius+28), fragment.Radius, worldSize-fragment.Radius),
			Y:               clamp(fragment.Y-dir.Y*(fragment.Radius+28), fragment.Radius, worldSize-fragment.Radius),
			Mass:            childMass,
			Radius:          massToRadius(childMass),
			Scale:           1,
			Color:           fragment.Color,
			IsBot:           fragment.IsBot,
			Direction:       fragment.Direction,
			CooldownUntil:   now.Add(dividerSplitCooldown),
			MergeReadyAt:    now.Add(dividerMergeDelay),
			LastSeen:        fragment.LastSeen,
			NextBotThinkAt:  fragment.NextBotThinkAt,
			IsAbilityActive: fragment.IsAbilityActive,
			Coins:           fragment.Coins,
			Upgrades:        fragment.Upgrades,
			ProbioticUntil:  fragment.ProbioticUntil,
			ShieldUntil:     fragment.ShieldUntil,
			SpeedBoostUntil: fragment.SpeedBoostUntil,
			Energy:          fragment.Energy,
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

func canEatPlayer(now time.Time, attacker, defender *player, gap float64) bool {
	if attacker == nil || defender == nil {
		return false
	}
	if isRespawningAt(now, attacker) || isRespawningAt(now, defender) {
		return false
	}
	if attacker.Mass <= 0 || defender.Mass <= 0 || attacker.Radius <= 0 || defender.Radius <= 0 {
		return false
	}

	attackerRadius := currentRadius(attacker)
	defenderRadius := currentRadius(defender)
	requiredCenterDepth := attackerRadius - defenderRadius*0.5
	if gap > requiredCenterDepth {
		return false
	}

	if now.Before(defender.ShieldUntil) {
		return false
	}

	if effectiveCombatMass(attacker) <= effectiveCombatMass(defender)*1.1 {
		return false
	}

	if defender.CellType == "giant" && now.Before(defender.EffectUntil) {
		requiredMass := defender.Mass * 1.1 * math.Max(1, defender.Scale)
		if effectiveCombatMass(attacker) < requiredMass {
			return false
		}
	}

	if defender.CellType == "shield" && now.Before(defender.EffectUntil) {
		return false
	}

	if attacker.CellType == "giant" && now.Before(attacker.EffectUntil) && !attacker.Upgrades.Giant {
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

func (s *gameState) pullSmallerPlayersLocked(p *player, radius float64) {
	now := time.Now()
	ownerID := ownerIDOf(p)
	worldSize := s.worldSize()
	for _, other := range s.players {
		otherOwnerID := ownerIDOf(other)
		if otherOwnerID == ownerID || other.CellType == "shield" && now.Before(other.EffectUntil) || now.Before(other.ShieldUntil) {
			continue
		}
		if effectiveCombatMass(other) >= effectiveCombatMass(p)*0.92 {
			continue
		}
		dist := distance(p.X, p.Y, other.X, other.Y)
		if dist <= 0.001 || dist > radius {
			continue
		}
		pull := (1 - dist/radius) * 28
		dirX := (p.X - other.X) / dist
		dirY := (p.Y - other.Y) / dist
		other.Direction = direction{X: dirX * 0.45, Y: dirY * 0.45}
		other.X = clamp(other.X+dirX*pull/tickRate, currentRadius(other), worldSize-currentRadius(other))
		other.Y = clamp(other.Y+dirY*pull/tickRate, currentRadius(other), worldSize-currentRadius(other))
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
			ID:              randomID(),
			SessionID:       "",
			OwnerID:         ownerID,
			Nickname:        p.Nickname,
			CellType:        p.CellType,
			Ability:         p.Ability,
			X:               clamp(p.X+offsetX*spawnDistance, childRadius, worldSize-childRadius),
			Y:               clamp(p.Y+offsetY*spawnDistance, childRadius, worldSize-childRadius),
			Mass:            perChildMass,
			Radius:          childRadius,
			Scale:           1,
			Color:           p.Color,
			IsBot:           p.IsBot,
			Direction:       direction{X: offsetX, Y: offsetY},
			MergeReadyAt:    now.Add(dividerMergeDelay),
			LastSeen:        p.LastSeen,
			NextBotThinkAt:  p.NextBotThinkAt,
			IsAbilityActive: false,
			Coins:           p.Coins,
			Upgrades:        p.Upgrades,
			ProbioticUntil:  p.ProbioticUntil,
			ShieldUntil:     p.ShieldUntil,
			SpeedBoostUntil: p.SpeedBoostUntil,
			Energy:          p.Energy,
		}
		s.players[child.ID] = child
	}

	recoil := 42.0
	p.X = clamp(p.X-dir.X*recoil, currentRadius(p), worldSize-currentRadius(p))
	p.Y = clamp(p.Y-dir.Y*recoil, currentRadius(p), worldSize-currentRadius(p))
}
