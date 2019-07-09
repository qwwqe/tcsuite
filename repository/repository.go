package repository

import (
	"database/sql"
	//"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"sync"

	pq "github.com/lib/pq"
	"github.com/qwwqe/colly/storage"
	"github.com/qwwqe/tcsuite/content"
	//"github.com/qwwqe/tcsuite/lexicon"
)

type Repository interface {
	SaveContent(c *content.FetchedContent)
	CollyStorage

	AddLexeme(name string, language string, lexeme string, frequency int) error
	AddLexemes(name string, language string, lexemes []string, frequencies []int) error
	GetLexemes(name string, language string) (lexemes []string, frequences []int, err error)
}

type CollyStorage storage.Storage

type RepositoryOptions struct {
	RestoreRequestHistory bool
	EnableCookies         bool
}

type repository struct {
	db      *sql.DB
	Options RepositoryOptions
	//Storage colly.Storage
}

var repo *repository
var once sync.Once

var dbuser = "rosie"
var dbname = "tcsuite"

func GetRepository(options RepositoryOptions) Repository {
	once.Do(func() {
		repo = &repository{}
		var err error
		repo.db, err = sql.Open("postgres", "user=rosie dbname=tcsuite sslmode=disable")
		if err != nil {
			log.Fatal(err)
		}

		if err = repo.db.Ping(); err != nil {
			log.Fatal(err)
		}

		repo.db.SetMaxOpenConns(50)
		repo.db.SetMaxIdleConns(0)

		initDatabase(repo.db, options.RestoreRequestHistory)

		repo.Options.RestoreRequestHistory = options.RestoreRequestHistory
	})

	return repo
}

func initDatabase(db *sql.DB, restoreRequestHistory bool) {
	// SCRAPED CONTENT
	db.Exec("CREATE TABLE IF NOT EXISTS original_content (id SERIAL PRIMARY KEY, title VARCHAR NOT NULL, date TIMESTAMP, author VARCHAR, abstract VARCHAR, body TEXT NOT NULL, uri VARCHAR UNIQUE NOT NULL, language INTEGER REFERENCES languages(id))")
	//db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS language_idx ON original_content(language)")
	db.Exec("CREATE TABLE IF NOT EXISTS sources (name VARCHAR UNIQUE NOT NULL, uri VARCHAR)")
	db.Exec("CREATE TABLE IF NOT EXISTS content_tags (name VARCHAR UNIQUE NOT NULL)")
	db.Exec("CREATE TABLE IF NOT EXISTS content_to_sources (contentId INTEGER REFERENCES original_content(id), source VARCHAR REFERENCES sources(name), unique(contentId, source))")
	db.Exec("CREATE TABLE IF NOT EXISTS content_to_tags (contentId INTEGER REFERENCES original_content(id), tag VARCHAR REFERENCES content_tags(name), unique(contentId, tag))")
	db.Exec("CREATE TABLE IF NOT EXISTS languages (id SERIAL PRIMARY KEY, name VARCHAR UNIQUE NOT NULL)")

	// COLLY BOOKKEEPING
	if !restoreRequestHistory {
		db.Exec("DROP TABLE IF EXISTS request_history")
	}
	db.Exec("CREATE TABLE IF NOT EXISTS request_history (requestId VARCHAR)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS requestId_idx ON request_history(requestId)")

	if !restoreRequestHistory {
		db.Exec("DROP TABLE IF EXISTS cookie_history")
	}
	db.Exec("CREATE TABLE IF NOT EXISTS cookie_history (host VARCHAR, cookies VARCHAR)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS host_idx ON cookie_history(host)")

	// LEXICA
	db.Exec("CREATE TABLE IF NOT EXISTS lexica (id SERIAL PRIMARY KEY, name VARCHAR UNIQUE NOT NULL, language INTEGER REFERENCES languages(id))")
	db.Exec("CREATE TABLE IF NOT EXISTS lexicon_words (id SERIAL PRIMARY KEY, word VARCHAR NOT NULL, frequency INTEGER NOT NULL DEFAULT 0, lexicon INTEGER REFERENCES lexica(id), unique(word, lexicon))")
}

func (r *repository) SaveContent(c *content.FetchedContent) {
	// TODO: deal with conflicts (...ON CONFLICT DO NOTHING)
	// TODO: use a transaction

	// Add language and retrieve id
	if c.Language == "" {
		log.Fatal("No language present on FetchedContent")
	}
	languageId, err := r.addOrRetrieveLanguageId(c.Language)
	if err != nil {
		log.Fatal(err)
	}

	// Insert content
	var lastContentId int
	err = r.db.QueryRow("INSERT INTO original_content (title, date, author, abstract, body, uri, language) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING RETURNING id",
		c.Title, c.Date, c.Author, c.Abstract, c.Body, c.Uri, languageId).Scan(&lastContentId)
	if err != nil {
		if err == sql.ErrNoRows { // content saved already
			return
		}
		log.Fatal(err)
	}

	_, err = r.db.Exec("INSERT INTO sources (name) VALUES ($1) ON CONFLICT DO NOTHING", c.CanonName)
	if err != nil {
		log.Fatal(err)
	}

	for _, tag := range c.Tags {
		_, err = r.db.Exec("INSERT INTO content_tags (name) VALUES ($1) ON CONFLICT DO NOTHING", tag)
		if err != nil {
			log.Fatal(err)
		}

		_, err = r.db.Exec("INSERT INTO content_to_tags (contentId, tag) VALUES ($1, $2) ON CONFLICT DO NOTHING", lastContentId, tag)
		if err != nil {
			log.Fatal(err)
		}
	}

	_, err = r.db.Exec("INSERT INTO content_to_sources (contentId, source) VALUES ($1, $2) ON CONFLICT DO NOTHING", lastContentId, c.CanonName)
	if err != nil {
		log.Fatal(err)
	}

}

// Below is this repository's implementation of colly's Storage interface

func (r *repository) Init() error {
	return nil
}

func (r *repository) Visited(requestId uint64) error {
	// Go's sql package doesn't support insertion of uint64s...
	requestIdString := strconv.FormatUint(requestId, 10)
	_, err := r.db.Exec("INSERT INTO request_history (requestId) VALUES ($1) ON CONFLICT DO NOTHING", requestIdString)
	return err
}

func (r *repository) IsVisited(requestId uint64) (bool, error) {
	requestIdString := strconv.FormatUint(requestId, 10)
	var destRequest string

	err := r.db.QueryRow("SELECT requestId FROM request_history WHERE requestId = $1", requestIdString).Scan(&destRequest)
	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		fmt.Printf("Repository error: %v\n", err)
		return false, err
	}

	return true, nil
}

func (r *repository) Cookies(u *url.URL) string {
	if !r.Options.EnableCookies {
		return ""
	}

	var cookies string
	err := r.db.QueryRow("SELECT cookies FROM cookie_history WHERE host = $1", u.Hostname()).Scan(&cookies)
	if err != nil {
		fmt.Printf("Repository error: %v\n", err)
		return ""
	}

	return cookies
}

func (r *repository) SetCookies(u *url.URL, cookies string) {
	if !r.Options.EnableCookies {
		return
	}

	_, err := r.db.Exec("INSERT INTO cookie_history (host, cookies) VALUES ($1, $2) ON CONFLICT DO NOTHING", u.Hostname(), cookies)
	if err != nil {
		fmt.Printf("Repository error: %v\n", err)
	}
}

// LEXICON

// AddLexeme adds an individual lexeme the the lexeme repository.
// Duplicate lexemes will be ignored.
func (r *repository) AddLexeme(name string, language string, lexeme string, frequency int) error {
	languageId, err := r.addOrRetrieveLanguageId(language)
	if err != nil {
		return err
	}

	lexiconId, err := r.addOrRetrieveLexiconId(name, languageId)
	if err != nil {
		return err
	}

	_, err = r.db.Exec("INSERT INTO lexicon_words (word, frequency, lexicon) ($1, $2, $3) ON CONFLICT DO NOTHING", lexeme, frequency, lexiconId)
	if err != nil {
		return err
	}

	return nil
}

// AddLexemes adds lexemes by bulk to the lexeme repository.
// Due to limitations of pq.CopyIn() and unlike the AddLexeme() method, this will fail on duplicate entries.
func (r *repository) AddLexemes(name string, language string, lexemes []string, frequencies []int) error {
	languageId, err := r.addOrRetrieveLanguageId(language)
	if err != nil {
		return err
	}

	lexiconId, err := r.addOrRetrieveLexiconId(name, languageId)
	if err != nil {
		return err
	}

	// stmt, err := r.db.Prepare("INSERT INTO lexicon_words (word, lexicon) VALUES ($1, $2) ON CONFLICT DO NOTHING")
	// if err != nil {
	// 	return err
	// }

	// // TODO: BLANK LEXEMES????
	// for i, lexeme := range lexemes {
	// 	if i >= len(frequencies) {
	// 		break
	// 	}
	// 	_, err = stmt.Exec(lexeme, lexiconId)
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	txn, err := r.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := txn.Prepare(pq.CopyIn("lexicon_words", "word", "frequency", "lexicon"))
	if err != nil {
		return err
	}

	for i, lexeme := range lexemes {
		if i >= len(frequencies) {
			break
		}
		_, err = stmt.Exec(lexeme, frequencies[i], lexiconId)
		if err != nil {
			fmt.Printf("AddLexeme error on: %s, %d (%d)\n", lexeme, frequencies[i], lexiconId)
			return err
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	err = stmt.Close()
	if err != nil {
		return err
	}

	err = txn.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (r *repository) GetLexemes(lexiconName string, language string) ([]string, []int, error) {
	lexemes := make([]string, 0)
	frequencies := make([]int, 0)

	languageId, err := r.retrieveLanguageId(language)
	if err != nil {
		return []string{}, []int{}, err
	}

	lexiconId, err := r.retrieveLexiconId(lexiconName, languageId)
	if err != nil {
		return []string{}, []int{}, err
	}

	rows, err := r.db.Query("SELECT word, frequency FROM lexicon_words WHERE lexicon = $1", lexiconId)
	if err != nil {
		return []string{}, []int{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var lexeme string
		var frequency int
		if err := rows.Scan(&lexeme, &frequency); err != nil {
			return []string{}, []int{}, err
		}

		lexemes = append(lexemes, lexeme)
		frequencies = append(frequencies, frequency)
	}
	err = rows.Err()
	if err != nil {
		return []string{}, []int{}, err
	}

	return lexemes, frequencies, nil
}

// HELPERS

func (r *repository) retrieveLanguageId(name string) (int, error) {
	var languageId int
	err := r.db.QueryRow("SELECT id FROM LANGUAGES WHERE name = $1", name).Scan(&languageId)
	if err != nil {
		return -1, err
	}

	return languageId, nil
}

func (r *repository) addOrRetrieveLanguageId(name string) (int, error) {
	var languageId int
	err := r.db.QueryRow("SELECT id FROM languages WHERE name = $1", name).Scan(&languageId)
	if err == sql.ErrNoRows {
		fmt.Printf("FAILED TO GRAB LANGUAGE FROM TABLE (MUST BE FIRST OCCURRENCE?). Language: %s\n", name)
		err = r.db.QueryRow("INSERT INTO languages (name) VALUES ($1) ON CONFLICT DO NOTHING RETURNING id", name).Scan(&languageId)
		if err != nil {
			return -1, err
		}
	} else if err != nil {
		return -1, err
	}

	return languageId, nil
}

func (r *repository) retrieveLexiconId(name string, languageId int) (int, error) {
	var lexiconId int
	err := r.db.QueryRow("SELECT id FROM lexica WHERE name = $1 and language = $2", name, languageId).Scan(&lexiconId)
	if err != nil {
		return -1, err
	}

	return lexiconId, nil
}

func (r *repository) addOrRetrieveLexiconId(name string, languageId int) (int, error) {
	var lexiconId int
	err := r.db.QueryRow("SELECT id FROM lexica WHERE name = $1 and language = $2", name, languageId).Scan(&lexiconId)
	if err != nil {
		if err == sql.ErrNoRows {
			err = r.db.QueryRow("INSERT INTO lexica (name, language) VALUES ($1, $2) RETURNING id", name, languageId).Scan(&lexiconId)
			if err != nil {
				return -1, err
			}
		} else {
			return -1, err
		}
	}

	return lexiconId, err
}
