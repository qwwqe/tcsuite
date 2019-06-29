package fetcher

type Fetcher interface {
	Fetch(n int, state interface{}) ([]FetchedContent, error)
	FetchByUri(uri string) (FetchedContent, error)
	Name() string
}

var Fetchers = []Fetcher{
	LibertyFetcher{},
}

type FetchedContent struct {
	Title     string
	Date      string
	Author    string
	Abstract  string
	Body      string
	Tags      []string
	CanonName string
	Uri       string
}
