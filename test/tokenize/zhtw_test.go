package tokenize

import (
	"github.com/qwwqe/tcsuite/entities/languages"
	l "github.com/qwwqe/tcsuite/lexicon"
	r "github.com/qwwqe/tcsuite/repository"
	"github.com/qwwqe/tcsuite/tokenizer"
	"github.com/qwwqe/tcsuite/tokenizer/zhtw"
	"testing"
)

var text = "本次地震發生位置約位於日本本州西部近海。"
var correctTokenization = []string{
	"本", "次", "地震", "發生", "位置", "約", "位於", "日本", "本州", "西部", "近海", "。",
}

func TestTokenize(t *testing.T) {
	repo := r.GetRepository(r.RepositoryOptions{
		RestoreRequestHistory: false,
	})

	lexiconName := "Traditional Chinese Comprehensive"
	lexiconLang := languages.ZH_TW

	lexicon := l.NewZhTwLexicon(lexiconName, lexiconLang)
	err := lexicon.LoadRepository(repo)
	if err != nil {
		t.Fatal(err)
	}

	tok := zhtw.NewTokenizer(&tokenizer.Options{
		MaxDepth: 3,
	})

	tokens, err := tok.Tokenize(text, lexicon)
	if err != nil {
		t.Fatal(err)
	}

	if len(tokens) != len(correctTokenization) {
		tokenStrings := []string{}
		for _, token := range tokens {
			tokenStrings = append(tokenStrings, token.Word)
		}
		t.Errorf("zhtw.Tokenize(): Got\n%v\nwant\n%v", tokenStrings, correctTokenization)
	}

	for i, token := range tokens {
		if i < len(correctTokenization) && correctTokenization[i] != token.Word {
			t.Errorf("zhtw.Tokenize(): token[i] = %s, want %s", token.Word, correctTokenization[i])
		}
	}
}

func BenchmarkTokenizer(b *testing.B) {
	repo := r.GetRepository(r.RepositoryOptions{
		RestoreRequestHistory: false,
	})

	lexiconName := "Traditional Chinese Comprehensive"
	lexiconLang := languages.ZH_TW

	lexicon := l.NewZhTwLexicon(lexiconName, lexiconLang)
	err := lexicon.LoadRepository(repo)
	if err != nil {
		b.Fatal(err)
	}

	tok := zhtw.NewTokenizer(&tokenizer.Options{
		MaxDepth: 3,
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		fetchedContent, err := repo.GetFetchedContent(53040)
		if err != nil {
			b.Fatal(err)
		}

		_, err = tok.Tokenize(fetchedContent.Body, lexicon)
		if err != nil {
			b.Fatal(err)
		}
	}

}
