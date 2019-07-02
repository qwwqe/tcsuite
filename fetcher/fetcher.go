package fetcher

import (
	"github.com/qwwqe/tcsuite/repository"
	"time"
)

type Fetcher interface {
	Fetch(options FetchOptions) error
	SetFetcherOptions(options *FetcherOptions)
	GetFetcherOptions() *FetcherOptions
}

var Fetchers = []Fetcher{
	&LibertyFetcher{}, &WhoGovernsTwFetcher{},
}

type FetchOptions struct {
	ArticleLimit int
	BeforeTime   time.Time
	AfterTime    time.Time
	State        interface{}

	DeparturePoint string // starting url

	MaxDepth    int
	Async       bool
	Parallelism int

	Uri string // setting this disables the above options
}

type FetcherOptions struct {
	Repository repository.Repository
	//CanonName  string
}
