package lexicon

import (
	"github.com/qwwqe/tcsuite/repository"
)

type Lexicon interface {
	AddLexeme(lexeme string, frequency int) error
	AddLexemes(lexemes []string, frequencies []int) error
	GetLexemeFrequency(lexeme string) (frequency int, isPrefix bool, exists bool)
	// LoadRepository registers a repository with the lexicon.
	// Implementers should prepare any temporary data structures they need in this function.
	LoadRepository(repo repository.Repository) error
	NumEntries() int
}
