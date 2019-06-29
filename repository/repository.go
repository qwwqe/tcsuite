package repository

import (
	"database/sql"
	"log"
	"sync"

	_ "github.com/lib/pq"
	"github.com/qwwqe/tcsuite/fetcher"
)

type Repository interface {
	SaveContent(c *fetcher.FetchedContent)
}

type repository struct {
	db *sql.DB
}

var repo *repository
var once sync.Once

var dbuser = "rosie"
var dbname = "tcsuite"

func GetRepository() *repository {
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

		initDatabase(repo.db)
	})

	return repo
}

func initDatabase(db *sql.DB) {
	db.Exec("CREATE TABLE IF NOT EXISTS original_content (id SERIAL PRIMARY KEY, title VARCHAR NOT NULL, date TIMESTAMP, author VARCHAR, abstract VARCHAR, body TEXT NOT NULL, uri VARCHAR UNIQUE NOT NULL)")
	db.Exec("CREATE TABLE IF NOT EXISTS sources (name VARCHAR UNIQUE NOT NULL, uri VARCHAR)")
	db.Exec("CREATE TABLE IF NOT EXISTS content_tags (name VARCHAR UNIQUE NOT NULL)")
	db.Exec("CREATE TABLE IF NOT EXISTS content_to_sources (contentId INTEGER REFERENCES original_content(id), source VARCHAR REFERENCES sources(name)), unique(contentId, source)")
	db.Exec("CREATE TABLE IF NOT EXISTS content_to_tags (contentId INTEGER REFERENCES original_content(id), tag VARCHAR REFERENCES content_tags(name)), unique(contentId, tag)")
}

func (r *repository) SaveContent(c *fetcher.FetchedContent) {
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
