package bot

import "testing"

func TestTrie_IsWord(t *testing.T) {
	trie := NewTrie([]string{"кот", "кошка", "собака"})

	if !trie.IsWord("кот") {
		t.Error("expected 'кот' to be a word")
	}
	if !trie.IsWord("кошка") {
		t.Error("expected 'кошка' to be a word")
	}
	if trie.IsWord("ко") {
		t.Error("expected 'ко' not to be a word")
	}
	if trie.IsWord("коты") {
		t.Error("expected 'коты' not to be a word")
	}
}

func TestTrie_IsPrefix(t *testing.T) {
	trie := NewTrie([]string{"кот", "кошка"})

	if !trie.IsPrefix("ко") {
		t.Error("expected 'ко' to be a prefix")
	}
	if !trie.IsPrefix("кот") {
		t.Error("expected 'кот' to be a prefix")
	}
	if trie.IsPrefix("соб") {
		t.Error("expected 'соб' not to be a prefix")
	}
}
