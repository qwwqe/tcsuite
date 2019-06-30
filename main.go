package main

import (
	//"fmt"
	"time"

	//"github.com/qwwqe/tcsuite/content"
	"github.com/qwwqe/tcsuite/fetcher"
	r "github.com/qwwqe/tcsuite/repository"
)

func main() {
	lf := fetcher.LibertyFetcher{}

	repo := r.GetRepository(r.RepositoryOptions{
		RestoreRequestHistory: false,
	})
	lf.FetcherOptions.Repository = repo

	lf.Fetch(fetcher.FetchOptions{
		AfterTime:   time.Now().Add(-1 * 24 * time.Hour),
		MaxDepth:    3,
		Async:       true,
		Parallelism: 4,
	})
}
