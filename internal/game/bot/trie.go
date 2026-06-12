package bot

// trieNode is a single node in a prefix tree.
type trieNode struct {
	children map[rune]*trieNode
	isWord   bool
}

// Trie stores normalized Russian words for fast prefix and exact lookups.
type Trie struct {
	root *trieNode
}

// NewTrie builds a trie from the given words.
func NewTrie(words []string) *Trie {
	t := &Trie{root: &trieNode{children: make(map[rune]*trieNode)}}
	for _, w := range words {
		t.Insert(w)
	}
	return t
}

// Insert adds a word to the trie.
func (t *Trie) Insert(word string) {
	node := t.root
	for _, r := range word {
		if node.children == nil {
			node.children = make(map[rune]*trieNode)
		}
		next, ok := node.children[r]
		if !ok {
			next = &trieNode{children: make(map[rune]*trieNode)}
			node.children[r] = next
		}
		node = next
	}
	node.isWord = true
}

// IsWord reports whether word exists in the trie.
func (t *Trie) IsWord(word string) bool {
	node := t.root
	for _, r := range word {
		next, ok := node.children[r]
		if !ok {
			return false
		}
		node = next
	}
	return node.isWord
}

// IsPrefix reports whether any stored word starts with prefix.
func (t *Trie) IsPrefix(prefix string) bool {
	node := t.root
	for _, r := range prefix {
		next, ok := node.children[r]
		if !ok {
			return false
		}
		node = next
	}
	return true
}
