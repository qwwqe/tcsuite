package init

import (
	l "github.com/qwwqe/tcsuite/lexicon"
	r "github.com/qwwqe/tcsuite/repository"
	"golang.org/x/text/language"
	"testing"
)

func BenchmarkPopulatePrefixTrie(b *testing.B) {
	repo := r.GetRepository(r.RepositoryOptions{
		RestoreRequestHistory: false,
	})

	lexiconName := "Traditional Chinese Comprehensive"
	lexiconLang := language.MustParse("zh-tw").String()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lexicon := l.NewZhTwLexicon(lexiconName, lexiconLang)
		err := lexicon.LoadRepository(repo)
		if err != nil {
			b.Fatal(err)
		}
		b.Logf("Lexicon %s has %d entries.\n", lexiconName, lexicon.NumEntries())
	}
}
