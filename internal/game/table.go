package game

import (
	"errors"
	"unicode/utf8"
)

const (
	InitWordLengthMax = 5
)

var (
	ErrInitWordLength   = errors.New("table: init word max length")
	ErrLetterPlaceTaken = errors.New("table: letter place is taken")
	ErrWrongLetterPlace = errors.New("table: wrong place for letter")
)

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
		lt.Table[2][i] = &Letter{
			RowID: 2,
			ColID: uint8(i),
			Char:  string(v),
		}
		i++
	}

	return lt, nil
}

func (lt *LettersTable) InitialWord() string {
	var word string
	for _, v := range lt.Table[2] {
		word += v.Char
	}
	return word
}

func (lt *LettersTable) isPlaceForLetterTaken(l *Letter) bool {
	char := lt.Table[l.RowID][l.ColID]
	if char != nil {
		return true
	}
	return false
}

func (lt *LettersTable) PutLetterOnTable(l *Letter) error {
	if l.RowID >= 5 || l.ColID >= 5 {
		return ErrWrongLetterPlace
	}

	if lt.isPlaceForLetterTaken(l) {
		return ErrLetterPlaceTaken
	}

	switch l.RowID {
	case 0, 1:
		if lt.downCharEmpty(l) {
			return ErrWrongLetterPlace
		}
	case 3, 4:
		if lt.upperCharEmpty(l) {
			return ErrWrongLetterPlace
		}
	}

	lt.Table[l.RowID][l.ColID] = l

	return nil
}

func (lt *LettersTable) downCharEmpty(l *Letter) bool {
	char := lt.Table[l.RowID+1][l.ColID]
	if char != nil {
		return false
	}
	return true
}

func (lt *LettersTable) upperCharEmpty(l *Letter) bool {
	char := lt.Table[l.RowID-1][l.ColID]
	if char != nil {
		return false
	}
	return true
}
