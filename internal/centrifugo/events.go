package centrifugo

const (
	ChannelLobby         = "lobby"
	GameStatusInProgress = "in_progress"
	GameStatusFinished   = "finished"
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

// LobbyPlayer is a player entry inside a lobby GameEntry.
type LobbyPlayer struct {
	UID    string `json:"uid"`
	Exp    int    `json:"exp"`
	Rating int    `json:"rating"`
}

// GameEntry is a single game item inside EvLobbyUpdate.
type GameEntry struct {
	ID        string        `json:"id"`
	PlayerIDs []string      `json:"player_ids"`
	Players   []LobbyPlayer `json:"players"`
	Status    string        `json:"status"`
	StartedAt int64         `json:"started_at"`
}

// EvLobbyUpdate is published to the lobby channel whenever the game list changes.
// The client replaces its local list with the received Games slice.
type EvLobbyUpdate struct {
	Type  string      `json:"type"` // "lobby_update"
	Games []GameEntry `json:"games"`
}

type EvGameStarted struct {
	Type      string   `json:"type"`
	GameID    string   `json:"game_id"`
	Status    string   `json:"status"`
	PlayerIDs []string `json:"player_ids"`
	StartedAt int64    `json:"started_at"`
}

// PlayerState holds a player's uid, total EXP and current game score for EvGameState.
type PlayerState struct {
	UID        string   `json:"uid"`
	Exp        int      `json:"exp"`
	Rating     int      `json:"rating"`
	Score      int      `json:"score"`
	WordsCount int      `json:"words_count"`
	Words      []string `json:"words"`
	ExpGained  int      `json:"exp_gained,omitempty"`
}

// EvGameState carries the full board snapshot sent after game_started and after each move.
type EvGameState struct {
	Type           string        `json:"type"`
	GameID         string        `json:"game_id"`
	Board          [5][5]string  `json:"board"`
	CurrentTurnUID string        `json:"current_turn_uid"`
	Players        []PlayerState `json:"players"`
	Status         string        `json:"status"`
	MoveNumber     int           `json:"move_number"`
}

// EvTurnChange is published when the turn advances due to a timeout.
// It is a lightweight alternative to a full game_state snapshot: the board has
// not changed, only the active player and the timer need to be updated.
type EvTurnChange struct {
	Type           string `json:"type"` // always "turn_change"
	GameID         string `json:"game_id"`
	CurrentTurnUID string `json:"current_turn_uid"`
	Reason         string `json:"reason"` // "timeout"
}

// EvSkipWarn is published each time the current player skips a turn.
// SkipsLeft reaches 0 on the final skip; game_over follows immediately after.
type EvSkipWarn struct {
	Type      string `json:"type"` // "skip_warn"
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
	Players   []PlayerState `json:"players"`
	Reason    string        `json:"reason,omitempty"`
	// Board is the final board snapshot: on a natural finish the last move
	// never triggers a game_state event, so without it clients would not see
	// the final letter.
	Board [5][5]string `json:"board"`
}

// EvEndProposal is published when the current player proposes to end the game.
type EvEndProposal struct {
	Type        string `json:"type"` // "end_proposal"
	GameID      string `json:"game_id"`
	ProposerUID string `json:"proposer_uid"`
}

// EvEndProposalResult is published when the opponent responds to the end proposal.
// When Accepted is false, RemainingMs carries the remaining turn time in milliseconds.
type EvEndProposalResult struct {
	Type        string `json:"type"` // "end_proposal_result"
	GameID      string `json:"game_id"`
	Accepted    bool   `json:"accepted"`
	RemainingMs int64  `json:"remaining_ms,omitempty"`
}

// EvAchievementUnlocked is published to the game channel when a player unlocks
// a new achievement.
type EvAchievementUnlocked struct {
	Type          string `json:"type"` // "achievement_unlocked"
	GameID        string `json:"game_id"`
	PlayerUID     string `json:"player_uid"`
	AchievementID string `json:"achievement_id"`
	Name          string `json:"name"`
}

// EvMatchFound is published to the lobby channel when quick matchmaking puts
// players into a game — either a human pair or a bot fallback after the queue
// timeout. It carries the board snapshot and per-player subscription tokens so
// clients can enter the game screen without racing the first turn events.
type EvMatchFound struct {
	Type           string             `json:"type"` // "match_found"
	GameID         string             `json:"game_id"`
	VsBot          bool               `json:"vs_bot"`
	Board          [5][5]string       `json:"board"`
	CurrentTurnUID string             `json:"current_turn_uid"`
	Players        []MatchFoundPlayer `json:"players"`
}

// MatchFoundPlayer is one matched player; GameToken is empty for the bot.
type MatchFoundPlayer struct {
	UID       string `json:"uid"`
	Exp       int    `json:"exp"`
	Rating    int    `json:"rating"`
	GameToken string `json:"game_token,omitempty"`
}
