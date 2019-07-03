package repository

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"sync"

	_ "github.com/lib/pq"
	"github.com/qwwqe/tcsuite/content"
)

type Repository interface {
	SaveContent(c *content.FetchedContent)
	//GetLastVisited() string

	Init() error
	Visited(requestID uint64) error
	IsVisited(requestID uint64) (bool, error)
	Cookies(u *url.URL) string
	SetCookies(u *url.URL, cookies string)
}

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
	db.Exec("CREATE TABLE IF NOT EXISTS original_content (id SERIAL PRIMARY KEY, title VARCHAR NOT NULL, date TIMESTAMP, author VARCHAR, abstract VARCHAR, body TEXT NOT NULL, uri VARCHAR UNIQUE NOT NULL)")
	db.Exec("CREATE TABLE IF NOT EXISTS sources (name VARCHAR UNIQUE NOT NULL, uri VARCHAR)")
	db.Exec("CREATE TABLE IF NOT EXISTS content_tags (name VARCHAR UNIQUE NOT NULL)")
	db.Exec("CREATE TABLE IF NOT EXISTS content_to_sources (contentId INTEGER REFERENCES original_content(id), source VARCHAR REFERENCES sources(name), unique(contentId, source))")
	db.Exec("CREATE TABLE IF NOT EXISTS content_to_tags (contentId INTEGER REFERENCES original_content(id), tag VARCHAR REFERENCES content_tags(name), unique(contentId, tag))")

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
}

func (r *repository) SaveContent(c *content.FetchedContent) {
	// TODO: deal with conflicts (...ON CONFLICT DO NOTHING)
	// TODO: use a transaction
	var lastContentId int
	err := r.db.QueryRow("INSERT INTO original_content (title, date, author, abstract, body, uri) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING RETURNING id",
		c.Title, c.Date, c.Author, c.Abstract, c.Body, c.Uri).Scan(&lastContentId)
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
