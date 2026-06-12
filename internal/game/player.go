package game

// PlayerType distinguishes human players from bots.
type PlayerType int

const (
	PlayerTypeUnknown PlayerType = iota
	PlayerTypeHuman
	PlayerTypeBot
)
