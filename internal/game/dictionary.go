package game

import (
	"embed"
	"encoding/json"
	"fmt"
	"github.com/rustwizard/balda/internal/rnd"
	"io"
	"unicode/utf8"
)

//go:embed assets/russian_nouns_with_definition.json
var data embed.FS

type Dictionary struct {
	Definition  map[string]string
	FiveLetters map[int]string
}

var Dict *Dictionary

func NewDictionary() (*Dictionary, error) {
	dict := &Dictionary{
		Definition:  make(map[string]string),
		FiveLetters: make(map[int]string),
	}

	f, err := data.Open("assets/russian_nouns_with_definition.json")
	if err != nil {
		return dict, fmt.Errorf("game: dictionary: %w", err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	for {
		var words map[string]interface{}
		err = dec.Decode(&words)
		if err != nil && err != io.EOF {
			return dict, fmt.Errorf("game: dictionary: decode: %w", err)
		}

		if err == io.EOF {
			break
		}

		i := 0
		for k, v := range words {
			def := v.(map[string]interface{})
			dict.Definition[k] = def["definition"].(string)

			if utf8.RuneCountInString(k) == 5 {
				dict.FiveLetters[i] = k
				i++
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
