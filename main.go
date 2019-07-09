package main

import (
	"bufio"
	"fmt"
	"golang.org/x/text/language"
	"os"
	"strconv"
	"strings"
	"time"

	//"github.com/qwwqe/tcsuite/content"
	f "github.com/qwwqe/tcsuite/fetcher"
	l "github.com/qwwqe/tcsuite/lexicon"
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

var usage = "Usage: tcsuite <fetch | poplex> <lexicon file>\n"

func main() {
	if len(os.Args) == 1 {
		fmt.Printf(usage)
		os.Exit(0)
	}

	repo := r.GetRepository(r.RepositoryOptions{
		RestoreRequestHistory: false,
	})

	switch os.Args[1] {
	case "fetch":
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
	case "poplex":
		if len(os.Args) < 3 {
			fmt.Printf(usage)
			os.Exit(1)
		}

		lexiconName := "Traditional Chinese Comprehensive"
		lexiconLang := language.MustParse("zh-tw").String()
		lexicon := l.NewZhTwLexicon(lexiconName, lexiconLang)

		err := lexicon.LoadRepository(repo)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if lexicon.NumEntries() == 0 {
			file, err := os.Open(os.Args[2])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			defer file.Close()

			lexemes := make([]string, 0)
			frequencies := make([]int, 0)
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				text := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix("#", text) {
					continue
				}

				fields := strings.Fields(text)
				if len(fields) != 2 {
					continue
				}
				freq, err := strconv.Atoi(fields[1])
				if err != nil {
					continue
				}

				lexemes = append(lexemes, fields[0])
				frequencies = append(frequencies, freq)
			}
			err = lexicon.AddLexemes(lexemes, frequencies)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}

		fmt.Printf("Lexicon \"%s\" has %d entries.\n", lexiconName, lexicon.NumEntries())
	default:
		fmt.Printf(usage)
		os.Exit(1)
	}
}
