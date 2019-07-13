package corpus

type Corpus struct {
	Name     string
	Language string
	Words    []*Word
}

type Word struct {
	Word    string
	Lexical bool
}
