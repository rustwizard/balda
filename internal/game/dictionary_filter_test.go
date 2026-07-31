package game

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsArchaic(t *testing.T) {
	assert.True(t, isArchaic("м. устар. То же, что: нанаец."))
	assert.True(t, isArchaic("ж. устар. редк. Старинная обувь."))
	assert.False(t, isArchaic("м. 1) Жилое здание."))
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
