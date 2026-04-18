package main

import (
	"crypto/rand"
	"fmt"
	"math"
	mathrand "math/rand"
	"strings"
	"time"
)

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
