package lexicon

import (
	"github.com/qwwqe/tcsuite/repository"
)

type Lexicon interface {
	AddLexeme(lexeme string, frequency int)
	AddLexemes(lexemes []string, frequencies []int)
	GetLexemeFrequency(lexeme string) (frequency int, isPrefix bool, exists bool)
	// Register a repository with the lexicon.
	// Implementers should prepare any temporary data structures they need in this function.
	LoadRepository(Repository *repository.Repository) bool
}
