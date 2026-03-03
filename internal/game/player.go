package game

type Player struct {
	ID                  string
	Exp                 int
	Score               int
	Words               []string
	ConsecutiveTimeouts int
	Kicked              bool
}
