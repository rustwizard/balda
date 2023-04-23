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

// TODO: write tests
func (p Places) IsTakenWord(word []Letter) bool {
	w := makeWord(word)
	for _, player := range p {
		for _, pword := range player.Words {
			if pword == w {
				return true
			}
		}
	}
	return false
}
