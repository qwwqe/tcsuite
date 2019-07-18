package content

type FetchedContent struct {
	Id        int
	Title     string
	Date      string
	Author    string
	Abstract  string
	Body      string
	Tags      []string
	CanonName string
	Uri       string
	Language  string
}
