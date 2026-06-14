package game

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
			normK := normalizeWord(k)
			dict.Definition[normK] = def["definition"].(string)

			if utf8.RuneCountInString(normK) == 5 {
				dict.FiveLetters = append(dict.FiveLetters, normK)
			}
		}
	}

	return dict, nil
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
