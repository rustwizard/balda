package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBestWord(t *testing.T) {
	cases := []struct {
		name  string
		words []string
		want  string
	}{
		{"empty", nil, ""},
		{"single", []string{"кот"}, "кот"},
		{"longest wins", []string{"дом", "автостоп", "мир"}, "автостоп"},
		{"tie keeps first", []string{"кот", "дом"}, "кот"},
		{"cyrillic rune length", []string{"abcd", "ёлка"}, "abcd"}, // both 4 runes, first wins
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, bestWord(tc.words))
		})
	}
}

func TestFavoriteLetter(t *testing.T) {
	cases := []struct {
		name  string
		words []string
		want  string
	}{
		{"empty", nil, ""},
		{"single word", []string{"абажур"}, "а"},
		{"most frequent", []string{"кот", "дом", "автостоп"}, "о"},
		{"yo folds into ye", []string{"ёлка", "ель"}, "е"},
		{"case insensitive", []string{"АБА", "абажур"}, "а"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, favoriteLetter(tc.words))
		})
	}
}
