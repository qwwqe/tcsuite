package main

import (
	"time"

	//"github.com/qwwqe/tcsuite/content"
	f "github.com/qwwqe/tcsuite/fetcher"
	r "github.com/qwwqe/tcsuite/repository"
)

type FetchOptionSet struct {
	Fetcher    f.Fetcher
	InitialSet f.FetchOptions
	UpdateSet  f.FetchOptions
}

var fetchOptionSets = []FetchOptionSet{
	FetchOptionSet{
		Fetcher: &f.LibertyFetcher{},
		InitialSet: f.FetchOptions{
			MaxDepth:    5,
			Async:       true,
			Parallelism: 4,
		},
		UpdateSet: f.FetchOptions{
			AfterTime:   time.Now().Add(-1 * 24 * time.Hour),
			MaxDepth:    3,
			Async:       true,
			Parallelism: 4,
		},
	},

	FetchOptionSet{
		Fetcher: &f.WhoGovernsTwFetcher{},
		InitialSet: f.FetchOptions{
			Async:       true,
			Parallelism: 4,
		},
		UpdateSet: f.FetchOptions{
			MaxDepth:    3,
			Async:       true,
			Parallelism: 4,
		},
	},
}

func main() {
	repo := r.GetRepository(r.RepositoryOptions{
		RestoreRequestHistory: false,
	})

	for _, fOpts := range fetchOptionSets {
		fOpts.Fetcher.SetFetcherOptions(&f.FetcherOptions{
			Repository: repo,
		})
	}

	mode := "update"

	for _, fOpts := range fetchOptionSets {
		var fetchOpts f.FetchOptions
		switch mode {
		case "update":
			fetchOpts = fOpts.UpdateSet
		case "initial":
			fetchOpts = fOpts.InitialSet
		default:
			continue
		}
		fOpts.Fetcher.Fetch(fetchOpts)
	}

}
