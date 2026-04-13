package centrifugo

const (
	ChannelLobby = "lobby"
)

func ChannelGame(gameID string) string {
	return "game:" + gameID
}

type EvGameCreated struct {
	Type   string      `json:"type"`
	GameID string      `json:"game_id"`
	Status string      `json:"status"`
	Players []string   `json:"player_ids"`
}

type EvGameStarted struct {
	Type      string   `json:"type"`
	GameID    string   `json:"game_id"`
	Status    string   `json:"status"`
	PlayerIDs []string `json:"player_ids"`
	StartedAt int64    `json:"started_at"`
}
