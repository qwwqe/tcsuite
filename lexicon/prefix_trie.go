package lexicon

import (
	"io"
)

type PrefixTrie struct {
	root pftNode
}

type pftNode struct {
	//	value     rune
	frequency int
	children  map[rune]*pftNode
}

func (t *PrefixTrie) AddLexeme(lexeme string, frequency int) {
	t.addLexeme(lexeme, frequency)
}

func (t *PrefixTrie) AddLexemes(lexemes []string, frequencies []int) {
	for i, lexeme := range lexemes {
		if i < len(frequencies) {
			t.addLexeme(lexeme, frequencies[i])
		} else {
			break
		}
	}
}

func (t *PrefixTrie) GetFrequency(lexeme string) (frequency int, isPrefix bool, exists bool) {
	if len(lexeme) == 0 {
		return -1, false, false
	}

	curNode := &t.root

	for _, r := range lexeme {
		nextNode, ok := curNode[r]
		if !ok {
			return -1, false, false
		}

		curNode = nextNode
	}

	return curNode.frequency, len(curNode.children) > 0, curNode.frequency >= 0
}

func (t *PrefixTrie) addLexeme(lexeme string, frequency int) {
	curNode := &t.root

	for _, r := range lexeme {
		nextNode, ok := curNode[r]
		if !ok {
			nextNode = &pftNode{
				//				value:     r,
				frequency: -1,
			}
			curNode[r] = nextNode
		}

		curNode = nextNode
	}

	if curNode != &t.root {
		curNode.frequency = frequency
	}

	/*
		reader := strings.NewReader(lexeme)

		for {
			r, _, err := reader.ReadRune()
			if err != nil {
				if err == io.EOF {
					if curNode != &t.root {
						curNode.frequency = frequency
					}
				} else {
					fmt.Println("Prefix trie: %v\n", err)
				}

				break
			}

			nextNode, ok := curNode[r]
			if !ok {
				nextNode = &pftNode{
					value:     r,
					frequency: -1,
				}
				curNode[r] = nextNode
			}

			curNode = nextNode
		}
	*/
}
