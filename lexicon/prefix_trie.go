package lexicon

import (
//"io"
)

//type PrefixTrie *prefixTrie

type PrefixTrie interface {
	AddLexeme(string, int)
	AddLexemes([]string, []int)
	GetFrequency(string) (int, bool, bool)
	NumEntries() int
}

type prefixTrie struct {
	root    *pftNode
	entries int
}

type pftNode struct {
	//	value     rune
	frequency int
	children  map[rune]*pftNode
}

func NewPrefixTrie() PrefixTrie {
	return &prefixTrie{
		root: &pftNode{
			frequency: -1,
			children:  map[rune]*pftNode{},
		},
	}
}

func (t *prefixTrie) AddLexeme(lexeme string, frequency int) {
	t.addLexeme(lexeme, frequency)
}

func (t *prefixTrie) AddLexemes(lexemes []string, frequencies []int) {
	for i, lexeme := range lexemes {
		if i < len(frequencies) {
			t.addLexeme(lexeme, frequencies[i])
		} else {
			break
		}
	}
}

func (t *prefixTrie) GetFrequency(lexeme string) (frequency int, isPrefix bool, exists bool) {
	// if len(lexeme) == 0 {
	// 	return -1, false, false
	//}

	curNode := t.root

	for _, r := range lexeme {
		nextNode, ok := curNode.children[r]
		if !ok {
			return -1, false, false
		}

		curNode = nextNode
	}

	return curNode.frequency, len(curNode.children) > 0, curNode.frequency >= 0
}

func (t *prefixTrie) NumEntries() int {
	return t.entries
}

func (t *prefixTrie) addLexeme(lexeme string, frequency int) {
	curNode := t.root

	for _, r := range lexeme {
		nextNode, ok := curNode.children[r]
		if !ok {
			nextNode = &pftNode{
				//				value:     r,
				frequency: -1,
				children:  map[rune]*pftNode{},
			}
			curNode.children[r] = nextNode
		}

		curNode = nextNode
	}

	if curNode != t.root {
		if curNode.frequency == -1 {
			t.entries++
		}
		curNode.frequency = frequency
	}

}
