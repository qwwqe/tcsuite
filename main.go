package main

import (
	"bufio"
	"fmt"
	"log"
	"runtime"
	"runtime/pprof"
	//"golang.org/x/text/language"
	"os"
	"strconv"
	"strings"
	"time"

	//"github.com/qwwqe/tcsuite/content"
	"github.com/qwwqe/tcsuite/entities/languages"
	f "github.com/qwwqe/tcsuite/fetcher"
	l "github.com/qwwqe/tcsuite/lexicon"
	r "github.com/qwwqe/tcsuite/repository"
	t "github.com/qwwqe/tcsuite/tokenizer"
	"github.com/qwwqe/tcsuite/tokenizer/zhtw"
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

var usage = "Usage: tcsuite <fetch | poplex | tokenize> < | lexicon file | content_id>\n"

var cpuProfile = "cpuprofile"
var memProfile = "memprofile"

func main() {
	if len(os.Args) == 1 {
		fmt.Printf(usage)
		os.Exit(0)
	}

	cpuProfileF, err := os.Create(cpuProfile)
	if err != nil {
		log.Fatal("could not create CPU profile: ", err)
	}
	defer cpuProfileF.Close()
	if err := pprof.StartCPUProfile(cpuProfileF); err != nil {
		log.Fatal("could not start CPU profile: ", err)
	}
	defer pprof.StopCPUProfile()

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
		lexiconLang := languages.ZH_TW //language.MustParse("zh-tw").String()
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
	case "tokenize":
		if len(os.Args) < 3 {
			fmt.Println(usage)
			os.Exit(1)
		}

		lexiconName := "Traditional Chinese Comprehensive"
		lexiconLang := languages.ZH_TW //language.MustParse("zh-tw").String()
		lexicon := l.NewZhTwLexicon(lexiconName, lexiconLang)

		err := lexicon.LoadRepository(repo)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		id, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fetchedContent, err := repo.GetFetchedContent(id)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		tokenizer := zhtw.NewTokenizer(&t.Options{
			MaxDepth: 3,
		})

		tokens, err := tokenizer.Tokenize(fetchedContent.Body, lexicon)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		err = repo.RegisterTokens(id, tokens)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		for _, token := range tokens {
			fmt.Println(token.Word)
		}

	case "tokenize_by_tag":
		if len(os.Args) < 3 {
			fmt.Println(usage)
			os.Exit(1)
		}

		lexiconName := "Traditional Chinese Comprehensive"
		lexiconLang := languages.ZH_TW //language.MustParse("zh-tw").String()
		lexicon := l.NewZhTwLexicon(lexiconName, lexiconLang)

		err := lexicon.LoadRepository(repo)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		tag := os.Args[2]
		fetchedContents, err := repo.GetFetchedContentByTag(tag)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		tokenizer := zhtw.NewTokenizer(&t.Options{
			MaxDepth: 3,
		})

		for _, fetchedContent := range fetchedContents {
			tokens, err := tokenizer.Tokenize(fetchedContent.Body, lexicon)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			err = repo.RegisterTokens(fetchedContent.Id, tokens)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}

	default:
		fmt.Printf(usage)
		os.Exit(1)
	}

	memProfileF, err := os.Create(memProfile)
	if err != nil {
		log.Fatal("could not create memory profile: ", err)
	}
	defer memProfileF.Close()
	runtime.GC() // get up-to-date statistics
	if err := pprof.WriteHeapProfile(memProfileF); err != nil {
		log.Fatal("could not write memory profile: ", err)
	}

}
