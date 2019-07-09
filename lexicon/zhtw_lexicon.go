package lexicon

import (
	"github.com/qwwqe/tcsuite/repository"
)

type zhTwLexicon struct {
	name       string
	language   string
	prefixTrie PrefixTrie
	repository repository.Repository
}

// Create a new lexicon object, returning either a handle
// to a new lexicon or an existing one of the name provided.
func NewLexicon(name string, language string) Lexicon {
	return &zhTwLexicon{
		name:       name,
		language:   language,
		prefixTrie: NewPrefixTrie(),
	}
}

func (l *zhTwLexicon) AddLexeme(lexeme string, frequency int) error {
	err := l.repository.AddLexeme(l.name, l.language, lexeme, frequency)
	if err != nil {
		return err
	}
	l.prefixTrie.AddLexeme(lexeme, frequency)
	return nil
}

func (l *zhTwLexicon) AddLexemes(lexemes []string, frequencies []int) error {
	err := l.repository.AddLexemes(l.name, l.language, lexemes, frequencies)
	if err != nil {
		return err
	}
	l.prefixTrie.AddLexemes(lexemes, frequencies)
	return nil
}

func (l *zhTwLexicon) GetLexemeFrequency(lexeme string) (frequency int, isPrefix bool, exists bool) {
	return l.prefixTrie.GetFrequency(lexeme)
}

func (l *zhTwLexicon) LoadRepository(repository repository.Repository) error {
	l.repository = repository
	lexemes, frequencies, err := l.repository.GetLexemes(l.name, l.language)
	if len(lexemes) == 0 || err != nil {
		return err
	}

	l.prefixTrie.AddLexemes(lexemes, frequencies)
	return nil
}

func (l *zhTwLexicon) NumEntries() int {
	return l.prefixTrie.NumEntries()
}
