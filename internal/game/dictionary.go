package game

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rustwizard/balda/internal/rnd"
)

//go:embed assets/russian_nouns_with_definition.json
var data embed.FS

type Dictionary struct {
	Definition  map[string]string
	FiveLetters []string
}

var Dict *Dictionary

func NewDictionary() (*Dictionary, error) {
	dict := &Dictionary{
		Definition: make(map[string]string),
	}

	f, err := data.Open("assets/russian_nouns_with_definition.json")
	if err != nil {
		return dict, fmt.Errorf("game: dictionary: %w", err)
	}
	defer f.Close() //nolint:errcheck

	raw := make(map[string]string, 56000)

	dec := json.NewDecoder(f)
	for {
		var words map[string]interface{}
		err = dec.Decode(&words)
		if err != nil && !errors.Is(err, io.EOF) {
			return dict, fmt.Errorf("game: dictionary: decode: %w", err)
		}

		if errors.Is(err, io.EOF) {
			break
		}

		for k, v := range words {
			// Balda uses common nouns only — skip proper nouns (leading-uppercase
			// keys like "Диана", "Аллах"). Their lowercase common-noun homographs
			// (парка, земля, север) are separate keys and remain.
			if r, _ := utf8.DecodeRuneInString(k); unicode.IsUpper(r) {
				continue
			}

			def := v.(map[string]interface{})
			raw[normalizeWord(k)] = def["definition"].(string)
		}
	}

	// Filter out entries that break the classic Balda rules or feel unfair
	// to casual players (the bot plays by this dictionary too).
	var droppedArchaic, droppedPluralAlias int
	for w, def := range raw {
		if isArchaic(def) {
			droppedArchaic++
			continue
		}
		if isPluralAlias(w, def, raw) {
			droppedPluralAlias++
			continue
		}
		dict.Definition[w] = def
		if utf8.RuneCountInString(w) == 5 {
			dict.FiveLetters = append(dict.FiveLetters, w)
		}
	}
	slog.Info("game: dictionary loaded",
		slog.Int("kept", len(dict.Definition)),
		slog.Int("dropped_archaic", droppedArchaic),
		slog.Int("dropped_plural_alias", droppedPluralAlias),
	)

	return dict, nil
}

// isArchaic reports whether the dictionary entry is marked obsolete (устар.).
func isArchaic(def string) bool {
	return strings.Contains(def, "устар.")
}

// isPluralAlias reports whether the entry is a plural form that merely
// aliases another word ("мн. ... То же, что: ...") while its singular form
// exists in the dictionary. Classic Balda allows singular nouns (plus
// pluralia tantum), so such aliases are excluded. Plurals with their own
// distinct meaning (руки "рабочая сила", коты "тёплая обувь") are kept,
// because they are not cross-references.
func isPluralAlias(word, def string, dict map[string]string) bool {
	if !strings.HasPrefix(def, "мн.") {
		return false
	}
	if !strings.Contains(strings.ToLower(def), "то же, что") {
		return false
	}
	for _, c := range singularCandidates(word) {
		if _, ok := dict[c]; ok {
			return true
		}
	}
	return false
}

// singularCandidates returns plausible singular forms of a plural word.
func singularCandidates(w string) []string {
	r := []rune(w)
	if len(r) < 2 {
		return nil
	}
	stem := string(r[:len(r)-1])
	switch {
	case strings.HasSuffix(w, "йцы") && len(r) >= 3: // нанайцы -> нанаец
		return []string{string(r[:len(r)-3]) + "ец", stem}
	case strings.HasSuffix(w, "цы"): // немцы -> немец
		return []string{string(r[:len(r)-2]) + "ец", stem}
	case strings.HasSuffix(w, "ы"), strings.HasSuffix(w, "и"): // гольды -> гольд
		return []string{stem, stem + "ь", stem + "й"}
	case strings.HasSuffix(w, "а"): // дома -> дом
		return []string{stem, stem + "о"}
	}
	return nil
}

func (d *Dictionary) RandomFiveLetterWord() string {
	idx, _ := rnd.Int(len(d.FiveLetters))
	return d.FiveLetters[idx]
}

func init() {
	dict, err := NewDictionary()
	if err != nil {
		panic(err)
	}

	Dict = dict
}
