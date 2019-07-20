package repository

import (
	"database/sql"
	//"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"

	pq "github.com/lib/pq"
	"github.com/qwwqe/colly/storage"
	"github.com/qwwqe/tcsuite/content"
	"github.com/qwwqe/tcsuite/entities/corpus"
	//"github.com/qwwqe/tcsuite/lexicon"
)

type Repository interface {
	GetFetchedContent(id int) (*content.FetchedContent, error)
	GetFetchedContentByTag(tag string) ([]*content.FetchedContent, error)
	GetUntokenizedContent() ([]*content.FetchedContent, error)
	SaveContent(c *content.FetchedContent)

	RegisterTokens(contentId int, tokens []*corpus.Word) error

	AddLexeme(name string, language string, lexeme string, frequency int) error
	AddLexemes(name string, language string, lexemes []string, frequencies []int) error
	GetLexemes(name string, language string) (lexemes []string, frequences []int, err error)

	CollyStorage
}

type CollyStorage storage.Storage

type RepositoryOptions struct {
	RestoreRequestHistory bool
	EnableCookies         bool
}

type repository struct {
	db      *sql.DB
	Options RepositoryOptions
	wordIds map[string]int
	//Storage colly.Storage
}

var repo *repository
var once sync.Once

var dbuser = "rosie"
var dbname = "tcsuite"

func GetRepository(options RepositoryOptions) Repository {
	once.Do(func() {
		repo = &repository{
			wordIds: map[string]int{},
		}
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
	// CONTENT
	db.Exec("CREATE TABLE IF NOT EXISTS original_content (id SERIAL PRIMARY KEY, title VARCHAR NOT NULL, date TIMESTAMP, author VARCHAR, abstract VARCHAR, body TEXT NOT NULL, uri VARCHAR UNIQUE NOT NULL, language INTEGER REFERENCES languages(id), tokenized BOOLEAN DEFAULT FALSE)")
	//db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS language_idx ON original_content(language)")
	db.Exec("CREATE TABLE IF NOT EXISTS sources (name VARCHAR UNIQUE NOT NULL, uri VARCHAR)")
	db.Exec("CREATE TABLE IF NOT EXISTS content_tags (name VARCHAR UNIQUE NOT NULL)")
	db.Exec("CREATE TABLE IF NOT EXISTS content_to_sources (contentId INTEGER REFERENCES original_content(id), source VARCHAR REFERENCES sources(name), unique(contentId, source))")
	db.Exec("CREATE TABLE IF NOT EXISTS content_to_tags (contentId INTEGER REFERENCES original_content(id), tag VARCHAR REFERENCES content_tags(name), unique(contentId, tag))")
	db.Exec("CREATE TABLE IF NOT EXISTS languages (id SERIAL PRIMARY KEY, name VARCHAR UNIQUE NOT NULL)")

	// WORDS
	db.Exec("CREATE TABLE IF NOT EXISTS words (id SERIAL PRIMARY KEY, word VARCHAR NOT NULL, lexical BOOLEAN DEFAULT TRUE, language INTEGER REFERENCES languages(id), constraint unique_word_lang_pair unique (word, language))")
	db.Exec("CREATE TABLE IF NOT EXISTS tokenized_content (id SERIAL PRIMARY KEY, position INTEGER NOT NULL, word INTEGER REFERENCES words(id), content INTEGER REFERENCES original_content(id))")
	db.Exec("CREATE INDEX IF NOT EXISTS token_content_idx ON tokenized_content(content)")

	db.Exec("CREATE MATERIALZIED VIEW IF NOT EXISTS token_strings " +
		"SELECT tokenized_content.content, tokenized_content.id AS token_id, tokenized_content.position, words.id AS word_id, words.word, words.lexical " +
		"FROM tokenized_content LEFT JOIN words ON tokenized_content.word = words.id")
	db.Exec("CREATE INDEX IF NOT EXISTS ON token_strings (content)")
	db.Exec("CREATE INDEX IF NOT EXISTS ON token_strings (position)")
	db.Exec("CREATE INDEX IF NOT EXISTS ON token_strings (word_id)")

	// LEXICA
	db.Exec("CREATE TABLE IF NOT EXISTS lexica (id SERIAL PRIMARY KEY, name VARCHAR UNIQUE NOT NULL, language INTEGER REFERENCES languages(id))")
	db.Exec("CREATE TABLE IF NOT EXISTS lexicon_words (id SERIAL PRIMARY KEY, word VARCHAR NOT NULL, frequency INTEGER NOT NULL DEFAULT 0, lexicon INTEGER REFERENCES lexica(id), unique(word, lexicon))")

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

func (r *repository) GetFetchedContent(id int) (*content.FetchedContent, error) {
	var c content.FetchedContent
	err := r.db.QueryRow("SELECT id, title, date, author, abstract, body FROM original_content WHERE id = $1", id).Scan(
		&c.Id, &c.Title, &c.Date, &c.Author, &c.Abstract, &c.Body)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func (r *repository) GetFetchedContentByTag(tag string) ([]*content.FetchedContent, error) {
	contents := []*content.FetchedContent{}
	rows, err := r.db.Query("SELECT id, title, date, author, abstract, body FROM original_content WHERE id in (SELECT contentid FROM content_to_tags WHERE tag = $1)", tag)
	if err != nil {
		return []*content.FetchedContent{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var c content.FetchedContent
		if err := rows.Scan(&c.Id, &c.Title, &c.Date, &c.Author, &c.Abstract, &c.Body); err != nil {
			return []*content.FetchedContent{}, err
		}
		contents = append(contents, &c)
	}

	if err = rows.Err(); err != nil {
		return []*content.FetchedContent{}, err
	}

	return contents, nil
}

func (r *repository) GetUntokenizedContent() ([]*content.FetchedContent, error) {
	contents := make([]*content.FetchedContent, 0, 10000)
	//contents := []*content.FetchedContent{}
	rows, err := r.db.Query("SELECT id, title, date, author, abstract, body FROM original_content WHERE tokenized = false")
	if err != nil {
		return []*content.FetchedContent{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var c content.FetchedContent
		if err := rows.Scan(&c.Id, &c.Title, &c.Date, &c.Author, &c.Abstract, &c.Body); err != nil {
			return []*content.FetchedContent{}, err
		}
		contents = append(contents, &c)
	}

	if err = rows.Err(); err != nil {
		return []*content.FetchedContent{}, err
	}

	return contents, nil
}

func (r *repository) RegisterTokens(contentId int, tokens []*corpus.Word) error {
	var languageId int
	var tokenized bool
	err := r.db.QueryRow("SELECT language, tokenized FROM original_content WHERE id = $1", contentId).Scan(&languageId, &tokenized)
	if err != nil {
		return err
	}

	// Halt if content is already tokenized
	if tokenized {
		return nil
	}

	// Retrieve/add word ids corresponding to the tokens
	// TODO: address this bottleneck
	/*
		wordIds := []int{}
		for _, token := range tokens {
			wordId, err := r.addOrRetrieveWordId(token.Word, token.Lexical, languageId)
			if err != nil {
				return err
			}

			wordIds = append(wordIds, wordId)
		}
	*/

	wordToId, err := r.addOrRetrieveWordIds(tokens, languageId)
	if err != nil {
		return err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	//stmt, err := tx.Prepare("INSERT INTO tokenized_content (position, word, content) VALUES ($1, $2, $3)")
	stmt, err := tx.Prepare(pq.CopyIn("tokenized_content", "position", "word", "content"))
	if err != nil {
		return err
	}

	// Compile tokenized corpus
	for i, token := range tokens {
		//_, err = stmt.Exec(i, wordIds[i], contentId)
		//fmt.Printf("%s %d %d\n", token.Word, wordToId[token.Word], contentId)
		_, err = stmt.Exec(i, wordToId[token.Word], contentId)
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				return rollbackErr
			}
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

	// Update content tokenized status
	_, err = tx.Exec("UPDATE original_content SET tokenized = TRUE WHERE id = $1", contentId)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return rollbackErr
		}
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
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

func (r *repository) addOrRetrieveWordId(word string, lexical bool, languageId int) (int, error) {
	var wordId int
	err := r.db.QueryRow("SELECT id FROM words WHERE word = $1 and language = $2", word, languageId).Scan(&wordId)
	if err != nil {
		if err == sql.ErrNoRows {
			err = r.db.QueryRow("INSERT INTO words (word, lexical, language) VALUES ($1, $2, $3) RETURNING id", word, lexical, languageId).Scan(&wordId)
			if err != nil {
				return -1, err
			}
		} else {
			return -1, err
		}
	}

	return wordId, nil
}

func (r *repository) testDoodler() error {
	foo := []interface{}{"condo", true, 3, "digbsy", false, 3}
	rows, err := r.db.Query("INSERT INTO words (word, lexical, language) VALUES ($1, $2, $3), ($4, $5, $6) ON CONFLICT ON CONSTRAINT unique_word_lang_pair DO UPDATE SET language = words.language RETURNING word, id", foo...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var word string
		var id int
		if err := rows.Scan(&word, &id); err != nil {
			return err
		}
		fmt.Printf("%s %d\n", word, id)
	}
	err = rows.Err()
	if err != nil {
		return err
	}

	return nil

}

func (r *repository) addOrRetrieveWordIds(words []*corpus.Word, languageId int) (map[string]int, error) {
	// TODO: whitespace isn't being inserted properly
	wordSeen := map[string]bool{}

	// this is so retarded
	valueStrings := make([]string, 0, len(words))
	wordArgs := make([]interface{}, 0, len(words))
	offset := 0
	for i, word := range words {
		_, seen := wordSeen[word.Word]
		if _, ok := r.wordIds[word.Word]; ok || seen {
			offset++
			continue
		}

		wordSeen[word.Word] = true

		// DANGER DANGER AMIRITE?
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, %t, %d)", i+1-offset, word.Lexical, languageId))
		wordArgs = append(wordArgs, word.Word)
	}

	if len(valueStrings) == 0 {
		return r.wordIds, nil
	}

	stmtString := fmt.Sprintf("INSERT INTO words (word, lexical, language) VALUES %s ON CONFLICT ON CONSTRAINT unique_word_lang_pair DO UPDATE SET language = words.language RETURNING word, id",
		strings.Join(valueStrings, ","))
	rows, err := r.db.Query(stmtString, wordArgs...)
	if err != nil {
		return map[string]int{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var word string
		var id int
		if err := rows.Scan(&word, &id); err != nil {
			return map[string]int{}, err
		}
		//wordMap[word] = id
		r.wordIds[word] = id
	}
	err = rows.Err()
	if err != nil {
		return map[string]int{}, err
	}

	return r.wordIds, nil

	/*
		entryRows := make([]interface{}, 0, len(words))
		for i, _ := range words {
			entryRows := append(entryRows,
		}
		langIds := make([]int, len(words)) // this is so retarded
		for i, _ := range langIds {
			langIds[i] = languageId
		}
		/// insert into words (word, lexical, language) values ('攻擊', true, 3) ON CONFLICT ON CONSTRAINT unique_word_lang_pair DO UPDATE SET language = 3 RETURNING id;
		stmt, err := r.db.Prepare("INSERT INTO WORDS (word, lexical, language) values ($1, $2, $3)
	*/
}
