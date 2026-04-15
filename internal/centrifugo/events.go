package centrifugo

const (
	ChannelLobby = "lobby"
)

func ChannelGame(gameID string) string {
	return "game:" + gameID
}

type EvGameCreated struct {
	Type    string   `json:"type"`
	GameID  string   `json:"game_id"`
	Status  string   `json:"status"`
	Players []string `json:"player_ids"`
}

type EvGameStarted struct {
	Type      string   `json:"type"`
	GameID    string   `json:"game_id"`
	Status    string   `json:"status"`
	PlayerIDs []string `json:"player_ids"`
	StartedAt int64    `json:"started_at"`
}

// PlayerScore holds a player's uid and current score for EvGameState.
type PlayerScore struct {
	UID        string `json:"uid"`
	Score      int    `json:"score"`
	WordsCount int    `json:"words_count"`
}

// EvGameState carries the full board snapshot sent after game_started and after each move.
type EvGameState struct {
	Type           string        `json:"type"`
	GameID         string        `json:"game_id"`
	Board          [5][5]string  `json:"board"`
	CurrentTurnUID string        `json:"current_turn_uid"`
	Players        []PlayerScore `json:"players"`
	Status         string        `json:"status"`
	MoveNumber     int           `json:"move_number"`
}

// EvTurnChange is published when the turn advances due to a timeout.
// It is a lightweight alternative to a full game_state snapshot: the board has
// not changed, only the active player and the timer need to be updated.
type EvTurnChange struct {
	Type           string `json:"type"`    // always "turn_change"
	GameID         string `json:"game_id"`
	CurrentTurnUID string `json:"current_turn_uid"`
	Reason         string `json:"reason"` // "timeout"
}

// EvSkipWarn is published each time the current player skips a turn.
// SkipsLeft reaches 0 on the final skip; game_over follows immediately after.
type EvSkipWarn struct {
	Type      string `json:"type"`       // "skip_warn"
	GameID    string `json:"game_id"`
	PlayerUID string `json:"player_uid"` // who skipped
	SkipsUsed int    `json:"skips_used"`
	SkipsLeft int    `json:"skips_left"`
}

// EvGameOver is published to the game channel when the game ends.
type EvGameOver struct {
	Type      string        `json:"type"`
	GameID    string        `json:"game_id"`
	WinnerUID string        `json:"winner_uid,omitempty"`
	Players   []PlayerScore `json:"players"`
}
