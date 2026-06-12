package game

import "testing"

func TestLettersTable_InitialWord_ShortWord(t *testing.T) {
	lt, err := NewLettersTable("кот")
	if err != nil {
		t.Fatalf("new letters table: %v", err)
	}

	word := lt.InitialWord()
	if word != "кот" {
		t.Fatalf("expected 'кот', got %q", word)
	}
}

func TestLettersTable_InitialWord_FiveLetters(t *testing.T) {
	lt, err := NewLettersTable("котка")
	if err != nil {
		t.Fatalf("new letters table: %v", err)
	}

	word := lt.InitialWord()
	if word != "котка" {
		t.Fatalf("expected 'котка', got %q", word)
	}
}
