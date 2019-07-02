package lexicon

import (
	"github.com/qwwqe/tcsuite/repository"
)

type ZhTwLexicon struct {
}

func (l *ZhTwLexicon) AddLexeme(lexeme string, frequency int) {

}

func (l *ZhTwLexicon) AddLexemes(lexemes []string, frequencies []int) {

}

func (l *ZhTwLexicon) GetLexemeFrequency(lexeme string) (frequency int, isPrefix bool, exists bool) {

}

func (l *ZhTwLexicon) LoadRepository(Repository *repository.Repository) bool {

}
