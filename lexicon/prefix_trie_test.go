package lexicon

import (
	"testing"
)

var testLexemes = []string{
	"教育",
	"教育學",
	"總統",
	"總統大選",
}

var testFrequencies = []int{
	10,
	5,
	50,
	25,
}

func TestPrefixTrie(t *testing.T) {
	trie := NewPrefixTrie()
	trie.AddLexemes(testLexemes, testFrequencies)
	freq, isPrefix, exists := trie.GetFrequency("教育")
	if freq != 10 {
		t.Errorf("PrefixTrie.GetFrequency(\"教育\") = %d, _, _; want 10, _, _", freq)
	}
	if !isPrefix {
		t.Errorf("PrefixTrie.GetFrequency(\"教育\") = _, %v, _; want _, true, _", isPrefix)
	}
	if !exists {
		t.Errorf("PrefixTrie.GetFrequency(\"教育\") = _, _, %v; want _, _, true", exists)
	}

	freq, isPrefix, exists = trie.GetFrequency("教育學")
	if freq != 5 {
		t.Errorf("PrefixTrie.GetFrequency(\"教育學\") = %d, _, _; want 5, _, _", freq)
	}
	if isPrefix {
		t.Errorf("PrefixTrie.GetFrequency(\"教育學\") = _, %v, _; want _, false, _", isPrefix)
	}
	if !exists {
		t.Errorf("PrefixTrie.GetFrequency(\"教育學\") = _, _, %v; want _, _, true", exists)
	}

	freq, isPrefix, exists = trie.GetFrequency("教育學貓")
	if freq != -1 {
		t.Errorf("PrefixTrie.GetFrequency(\"教育學貓\") = %d, _, _; want 5, _, _", freq)
	}
	if isPrefix {
		t.Errorf("PrefixTrie.GetFrequency(\"教育學貓\") = _, %v, _; want _, false, _", isPrefix)
	}
	if exists {
		t.Errorf("PrefixTrie.GetFrequency(\"教育學貓\") = _, _, %v; want _, _, false", exists)
	}

	freq, isPrefix, exists = trie.GetFrequency("教")
	if freq != -1 {
		t.Errorf("PrefixTrie.GetFrequency(\"教\") = %d, _, _; want -1, _, _", freq)
	}
	if !isPrefix {
		t.Errorf("PrefixTrie.GetFrequency(\"教\") = _, %v, _; want _, true, _", isPrefix)
	}
	if exists {
		t.Errorf("PrefixTrie.GetFrequency(\"教\") = _, _, %v; want _, _, false", exists)
	}

	freq, isPrefix, exists = trie.GetFrequency("貓")
	if freq != -1 {
		t.Errorf("PrefixTrie.GetFrequency(\"貓\") = %d, _, _; want -1, _, _", freq)
	}
	if isPrefix {
		t.Errorf("PrefixTrie.GetFrequency(\"貓\") = _, %v, _; want _, false, _", isPrefix)
	}
	if exists {
		t.Errorf("PrefixTrie.GetFrequency(\"貓\") = _, _, %v; want _, _, false", exists)
	}

	freq, isPrefix, exists = trie.GetFrequency("")
	if freq != -1 {
		t.Errorf("PrefixTrie.GetFrequency(\"\") = %d, _, _; want -1, _, _", freq)
	}
	if !isPrefix {
		t.Errorf("PrefixTrie.GetFrequency(\"\") = _, %v, _; want _, true, _", isPrefix)
	}
	if exists {
		t.Errorf("PrefixTrie.GetFrequency(\"\") = _, _, %v; want _, _, false", exists)
	}

}
