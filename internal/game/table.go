package game

import (
	"errors"
	"unicode/utf8"
)

const (
	InitWordLengthMax = 5
)

var ErrInitWordLength = errors.New("game: table: init word max length")

type Letter struct {
	RowID uint8 // row index of the 5x5 matrix
	ColID uint8 // column index of the 5x5 matrix
	Char  string
}

type LettersTable struct {
	Table [5][5]*Letter
}

func NewLettersTable(w string) (*LettersTable, error) {
	lt := &LettersTable{Table: [5][5]*Letter{}}
	if utf8.RuneCountInString(w) > InitWordLengthMax {
		return lt, ErrInitWordLength
	}

	i := 0
	for _, v := range w {
		lt.Table[3][i] = &Letter{
			RowID: 3,
			ColID: uint8(i),
			Char:  string(v),
		}
		i++
	}

	return lt, nil
}

func (lt *LettersTable) InitialWord() string {
	var word string
	for _, v := range lt.Table[3] {
		word += v.Char
	}
	return word
}

// TODO: write tests
func (lt *LettersTable) IsTakenPlaceForLetter(l *Letter) bool {
	char := lt.Table[l.RowID][l.ColID]
	if char != nil {
		return true
	}
	return false
}

func (lt *LettersTable) PutLetterOnTable(l *Letter) error {
	if lt.IsTakenPlaceForLetter(l) {
		return errors.New("table: letter place is taken")
	}
	lt.Table[l.RowID][l.ColID] = l
	return nil
}
