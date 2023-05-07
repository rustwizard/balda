package game

type Player struct {
	UserID            int
	Exp               int
	Score             int
	Words             []string
	TimeoutTurnsCount int
}

type Place struct {
	PlaceID    int
	PlaceState int
	Player
}

type Places map[int]*Place

func (p Places) IsTakenWord(word string) bool {
	for _, player := range p {
		for _, pword := range player.Words {
			if pword == word {
				return true
			}
		}
	}
	return false
}
