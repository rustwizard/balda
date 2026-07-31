package game

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsArchaic(t *testing.T) {
	// Fully obsolete words — устар. marks the first meaning.
	assert.True(t, isArchaic("м. устар. То же, что: нанаец."))
	assert.True(t, isArchaic("ж. устар. редк. Старинная обувь."))
	assert.True(t, isArchaic("1. ж. устар. Инструмент для резки."))
	// Common words with an archaic sub-meaning are kept.
	assert.False(t, isArchaic("1. ж. Несколько прядей волос. 2. ж. 1) Сельскохозяйственное орудие. 2) устар. Изображение такого орудия в руках у скелета. 3. ж. Длинная узкая отмель."))
	assert.False(t, isArchaic("м. 1) Жилое здание. 2) устар. Старая развалина."))
	assert.False(t, isArchaic("мн. Верхняя одежда."))
}

func TestIsPluralAlias(t *testing.T) {
	dict := map[string]string{
		"гольд":   "м. устар. То же, что: нанаец.",
		"нанаец":  "м. Житель Приамурья.",
		"ножница": "ж. устар. редк. То же, что: ножки (для мебели).",
		"кот":     "м. Домашнее животное.",
		"рука":    "ж. Конечность человека.",
		"дом":     "м. Жилое здание.",
	}

	cases := []struct {
		name string
		word string
		def  string
		want bool
	}{
		{"plural alias with singular in dict", "гольды", "мн. устар. То же, что: нанайцы.", true},
		{"singular entry", "дом", "м. Жилое здание.", false},
		{"plural with own meaning (работники)", "руки", "мн. разг. 1) Рабочая сила.", false},
		{"plural homograph with own meaning (обувь)", "коты", "мн. местн. Теплая, преимущественно женская обувь.", false},
		{"pluralia tantum, no cross-reference", "ножницы", "1. мн. 1) Инструмент для резки.", false},
		{"cross-ref but no singular in dict", "плесы", "мн. То же, что: глубоководные участки.", false},
		{"not a plural entry", "кот", "м. Домашнее животное.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isPluralAlias(tc.word, tc.def, dict))
		})
	}
}

func TestSingularCandidates(t *testing.T) {
	assert.Equal(t, []string{"нанаец", "нанайц"}, singularCandidates("нанайцы"))
	assert.Equal(t, []string{"гольд", "гольдь", "гольдй"}, singularCandidates("гольды"))
	assert.Equal(t, []string{"дом", "домо"}, singularCandidates("дома"))
	assert.Nil(t, singularCandidates("я"))
}

func TestDictionaryContainsCustomWords(t *testing.T) {
	assert.Contains(t, Dict.Definition, "сота")
	assert.Equal(t, "ж. Одна ячейка пчелиных сот.", Dict.Definition["сота"])
}
