package tokenizer

import (
	"github.com/qwwqe/tcsuite/entities/corpus"
	l "github.com/qwwqe/tcsuite/lexicon"
	//	"io"
)

type Interface interface {
	Tokenize(string, l.Lexicon) ([]*corpus.Word, error)
}

type Options struct {
	MaxDepth int
}
