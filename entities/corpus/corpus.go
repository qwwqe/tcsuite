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

func (w Word) String() string {
	return w.Word
}
