package main

import (
	"fmt"
	"math"
	mathrand "math/rand"
	"time"
)

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

func (s *gameState) updateBotOwnersLocked(now time.Time) {
	ownerFragments := buildOwnerFragmentsIndex(s.players)
	for ownerID, fragments := range ownerFragments {
		if len(fragments) == 0 || !fragments[0].IsBot || isRespawningAt(now, fragments[0]) {
			continue
		}
		s.updateBotOwnerLocked(ownerID, fragments, now)
	}
}

func (s *gameState) updateBotOwnerLocked(ownerID string, fragments []*player, now time.Time) {
	if len(fragments) == 0 {
		return
	}

	main := primaryOwnedFragmentFromFragments(ownerID, fragments)
	if main == nil {
		return
	}

	for _, fragment := range fragments {
		fragment.LastSeen = now
	}
	if now.Before(main.NextBotThinkAt) {
		return
	}

	nextThinkAt := now.Add(time.Duration(400+mathrand.Intn(700)) * time.Millisecond)
	for _, fragment := range fragments {
		fragment.NextBotThinkAt = nextThinkAt
	}

	centerX, centerY := ownerCenterFromFragments(fragments, s.worldSize())
	nearestFood := s.findNearestFoodFromPointLocked(centerX, centerY)
	smallerTarget := s.findNearestEnemyFromPointLocked(ownerID, centerX, centerY, func(other *player) bool {
		return effectiveCombatMass(other) < effectiveCombatMass(main)*0.9
	})
	largerThreat := s.findNearestEnemyFromPointLocked(ownerID, centerX, centerY, func(other *player) bool {
		return effectiveCombatMass(other) > effectiveCombatMass(main)*1.18
	})

	moveDir := direction{}
	triggerAbility := false
	holdAbility := false

	switch {
	case largerThreat != nil && distance(centerX, centerY, largerThreat.X, largerThreat.Y) < 260:
		moveDir = normalizeDirection(centerX-largerThreat.X, centerY-largerThreat.Y)
		if main.CellType == "blink" || main.CellType == "shield" {
			triggerAbility = true
		}
	case smallerTarget != nil && distance(centerX, centerY, smallerTarget.X, smallerTarget.Y) < 320:
		moveDir = normalizeDirection(smallerTarget.X-centerX, smallerTarget.Y-centerY)
		if (main.CellType == "giant" && effectiveCombatMass(main) > effectiveCombatMass(smallerTarget)*1.12) ||
			(main.CellType == "divider" && main.Mass > dividerMinSplitMass*1.2) {
			triggerAbility = true
		}
	case nearestFood != nil:
		moveDir = normalizeDirection(nearestFood.X-centerX, nearestFood.Y-centerY)
		if main.CellType == "magnet" {
			triggerAbility = true
		}
	default:
		moveDir = normalizeDirection(mathrand.Float64()*2-1, mathrand.Float64()*2-1)
	}

	if main.CellType == "classic" {
		if largerThreat != nil && distance(centerX, centerY, largerThreat.X, largerThreat.Y) < 300 {
			holdAbility = true
		} else if smallerTarget != nil && distance(centerX, centerY, smallerTarget.X, smallerTarget.Y) < 350 {
			holdAbility = true
		}
	}

	for _, fragment := range fragments {
		fragment.Direction = moveDir
		fragment.IsAbilityActive = holdAbility
	}

	if triggerAbility {
		s.tryUseAbility(main)
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

func (s *gameState) findNearestFoodFromPointLocked(x, y float64) *food {
	var best *food
	bestDistance := math.MaxFloat64
	for _, f := range s.foods {
		dist := distance(x, y, f.X, f.Y)
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

func (s *gameState) findNearestEnemyFromPointLocked(ownerID string, x, y float64, predicate func(*player) bool) *player {
	var best *player
	bestDistance := math.MaxFloat64
	for _, other := range s.players {
		otherOwnerID := other.OwnerID
		if otherOwnerID == "" {
			otherOwnerID = other.ID
		}
		if otherOwnerID == ownerID || !predicate(other) {
			continue
		}
		dist := distance(x, y, other.X, other.Y)
		if dist < bestDistance {
			bestDistance = dist
			best = other
		}
	}
	return best
}
