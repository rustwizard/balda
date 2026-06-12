package game

// PlayerType distinguishes human players from bots.
type PlayerType int

const (
	PlayerTypeHuman PlayerType = iota
	PlayerTypeBot
)
