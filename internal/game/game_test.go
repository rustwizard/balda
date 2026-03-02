package game

import "testing"

func TestGapsBetweenLetters(t *testing.T) {
	tests := []struct {
		name string
		word []Letter
		want bool
	}{
		{
			name: "empty word",
			word: []Letter{},
			want: true,
		},
		{
			name: "single letter",
			word: []Letter{{RowID: 2, ColID: 2, Char: "а"}},
			want: true,
		},
		{
			name: "two adjacent letters horizontally",
			word: []Letter{
				{RowID: 2, ColID: 1, Char: "б"},
				{RowID: 2, ColID: 2, Char: "а"},
			},
			want: false,
		},
		{
			name: "two adjacent letters vertically",
			word: []Letter{
				{RowID: 1, ColID: 2, Char: "б"},
				{RowID: 2, ColID: 2, Char: "а"},
			},
			want: false,
		},
		{
			name: "horizontal word no gaps",
			word: []Letter{
				{RowID: 2, ColID: 0, Char: "с"},
				{RowID: 2, ColID: 1, Char: "л"},
				{RowID: 2, ColID: 2, Char: "о"},
				{RowID: 2, ColID: 3, Char: "в"},
				{RowID: 2, ColID: 4, Char: "о"},
			},
			want: false,
		},
		{
			name: "vertical word no gaps",
			word: []Letter{
				{RowID: 0, ColID: 2, Char: "с"},
				{RowID: 1, ColID: 2, Char: "т"},
				{RowID: 2, ColID: 2, Char: "о"},
				{RowID: 3, ColID: 2, Char: "л"},
			},
			want: false,
		},
		{
			name: "L-shaped path no gaps",
			word: []Letter{
				{RowID: 0, ColID: 0, Char: "с"},
				{RowID: 1, ColID: 0, Char: "т"},
				{RowID: 2, ColID: 0, Char: "о"},
				{RowID: 2, ColID: 1, Char: "л"},
			},
			want: false,
		},
		{
			name: "gap of two cells",
			word: []Letter{
				{RowID: 2, ColID: 0, Char: "с"},
				{RowID: 2, ColID: 2, Char: "о"}, // skips ColID 1
			},
			want: true,
		},
		{
			name: "diagonal jump — not adjacent",
			word: []Letter{
				{RowID: 1, ColID: 1, Char: "а"},
				{RowID: 2, ColID: 2, Char: "б"}, // diagonal: rowDiff+colDiff == 2
			},
			want: true,
		},
		{
			name: "same cell repeated — distance 0",
			word: []Letter{
				{RowID: 2, ColID: 2, Char: "а"},
				{RowID: 2, ColID: 2, Char: "б"},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GapsBetweenLetters(tc.word)
			if got != tc.want {
				t.Errorf("GapsBetweenLetters(%v) = %v, want %v", tc.word, got, tc.want)
			}
		})
	}
}
