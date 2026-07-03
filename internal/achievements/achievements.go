package achievements

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
	"unicode/utf8"
)

// Definition describes a single achievement loaded from the database.
type Definition struct {
	ID            string
	Name          string
	Description   string
	IconURL       string
	ConditionType string
	Operator      string
	Threshold     int
	BitPosition   int
}

// Achievement is the runtime view of an achievement with unlock status.
type Achievement struct {
	Definition
	Unlocked bool
}

// Condition and operator constants used in achievement definitions.
const (
	ConditionTotalGames      = "total_games"
	ConditionConsecutiveWins = "consecutive_wins"
	ConditionScore           = "score"
	ConditionWordsCount      = "words_count"
	ConditionBestWordLength  = "best_word_length"
	ConditionWin             = "win"

	OperatorGTE = "gte"
	OperatorGT  = "gt"
	OperatorEQ  = "eq"
)

// PlayerGameStats holds per-game data needed to evaluate achievements.
type PlayerGameStats struct {
	TotalGames      int
	ConsecutiveWins int
	Score           int
	WordsCount      int
	BestWordLength  int
	IsWinner        bool
}

// Unlock represents a newly unlocked achievement.
type Unlock struct {
	PlayerID string
	Achievement
}

// Service holds achievement definitions and evaluates them against player stats.
// Definitions are loaded from persistent storage and can be reloaded at runtime.
type Service struct {
	loader func(ctx context.Context) ([]Definition, error)
	mu     sync.RWMutex
	defs   []Definition
	byID   map[string]Definition
	byBit  map[int]Definition
}

// NewService creates a service that loads definitions via the provided loader.
func NewService(loader func(ctx context.Context) ([]Definition, error)) *Service {
	return &Service{
		loader: loader,
		byID:   make(map[string]Definition),
		byBit:  make(map[int]Definition),
	}
}

// Load fetches definitions from storage and replaces the in-memory set.
func (s *Service) Load(ctx context.Context) error {
	defs, err := s.loader(ctx)
	if err != nil {
		return fmt.Errorf("achievements: load definitions: %w", err)
	}

	byID := make(map[string]Definition, len(defs))
	byBit := make(map[int]Definition, len(defs))
	for _, d := range defs {
		if d.BitPosition < 0 || d.BitPosition >= 64 {
			return fmt.Errorf("achievements: invalid bit_position %d for %q", d.BitPosition, d.ID)
		}
		byID[d.ID] = d
		byBit[d.BitPosition] = d
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.defs = defs
	s.byID = byID
	s.byBit = byBit
	return nil
}

// Start periodically reloads definitions until ctx is canceled.
func (s *Service) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Load(ctx); err != nil {
				slog.Error("achievements: periodic reload failed", slog.Any("error", err))
			}
		}
	}
}

// Reload is a synchronous convenience wrapper around Load.
func (s *Service) Reload(ctx context.Context) error {
	return s.Load(ctx)
}

func (s *Service) definitions() []Definition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Definition, len(s.defs))
	copy(out, s.defs)
	return out
}

// Calculate evaluates all currently known achievements for a single player.
// It returns the updated bitmask and the list of newly unlocked achievements.
func (s *Service) Calculate(oldFlags int64, stats PlayerGameStats) (newFlags int64, unlocked []Achievement) {
	flags := oldFlags
	for _, d := range s.definitions() {
		bit := int64(1) << d.BitPosition
		if flags&bit != 0 {
			continue
		}
		if !matches(d, stats) {
			continue
		}
		flags |= bit
		unlocked = append(unlocked, Achievement{Definition: d, Unlocked: true})
	}
	return flags, unlocked
}

// List returns all currently known achievements with their unlock status.
func (s *Service) List(flags int64) []Achievement {
	defs := s.definitions()
	out := make([]Achievement, len(defs))
	for i, d := range defs {
		out[i] = Achievement{
			Definition: d,
			Unlocked:   flags&(int64(1)<<d.BitPosition) != 0,
		}
	}
	return out
}

func matches(d Definition, stats PlayerGameStats) bool {
	var value int
	switch d.ConditionType {
	case ConditionTotalGames:
		value = stats.TotalGames
	case ConditionConsecutiveWins:
		value = stats.ConsecutiveWins
	case ConditionScore:
		value = stats.Score
	case ConditionWordsCount:
		value = stats.WordsCount
	case ConditionBestWordLength:
		value = stats.BestWordLength
	case ConditionWin:
		if stats.IsWinner && d.Threshold >= 1 {
			return true
		}
		return false
	default:
		return false
	}

	switch d.Operator {
	case OperatorGTE:
		return value >= d.Threshold
	case OperatorGT:
		return value > d.Threshold
	case OperatorEQ:
		return value == d.Threshold
	default:
		return value >= d.Threshold
	}
}

// WordLength returns the number of runes in a word.
func WordLength(w string) int {
	return utf8.RuneCountInString(w)
}
