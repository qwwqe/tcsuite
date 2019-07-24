package womany

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	//"github.com/gocolly/colly"
	"github.com/qwwqe/colly"
	"github.com/qwwqe/tcsuite/content"
	"github.com/qwwqe/tcsuite/entities/languages"
	"github.com/qwwqe/tcsuite/fetcher"
)

type Fetcher struct {
	FetcherOptions *fetcher.FetcherOptions
}

type articleList struct {
	TotalCount  int               `json:"total_count"`
	TotalPages  int               `json:"total_pages"`
	CurrentPage int               `json:"current_page"`
	Articles    []articleListItem `json:"articles"`
}

type articleListItem struct {
	Id       int                 `json:"id"`
	Title    string              `json:"title"`
	Date     string              `json:"published_at"`
	Abstract string              `json:"description"`
	Tags     []map[string]string `json:"tags"`
	Author   articleListAuthor   `json:"author"`
}

type articleListAuthor struct {
	Name string
}

var allowedDomains = []string{
	"api.womany.net",
	"womany.net",
	"www.womany.net",
}

var disallowedUrls = []*regexp.Regexp{
	/*
		regexp.MustCompile("/images/"),
		regexp.MustCompile("womany.net/shop[^/]*$"),
		regexp.MustCompile("womany.net/shop/"),
		regexp.MustCompile("/print$"),
	*/
}

var disallowedCacheUrls = []*regexp.Regexp{
	regexp.MustCompile("api.womany.net/articles/list"),
	/*
		regexp.MustCompile("womany.net/$"),
		regexp.MustCompile("/interests/"),
		regexp.MustCompile("top_writers"),
	*/
}

var universalTags = []string{
	"女人迷",
	"Womany",
}

//var defaultDeparturePoint = "https:/womany.net/"
//var defaultDeparturePoint = "https://womany.net/"
//var articleRegex = regexp.MustCompile("/read/article/")

var canonName = "女人迷"
var cacheDir = fetcher.CacheDir + "womany_cache"
var language = languages.ZH_TW

var articleBaseUrl = "https://womany.net/read/article/"
var apiUrl = "https://api.womany.net/articles/list" // ?page=1
var defaultDeparturePoint = "https://api.womany.net/articles/list?page=1"

var successful = 0

var savedArticles = map[int]articleListItem{}

func fetchLogf(format string, a ...interface{}) (n int, err error) {
	return fmt.Printf("[WOMANY FETCHER] "+format, a...)
}

func (f *Fetcher) SetFetcherOptions(fetcherOptions *fetcher.FetcherOptions) {
	f.FetcherOptions = fetcherOptions
}

func (f *Fetcher) GetFetcherOptions() *fetcher.FetcherOptions {
	return f.FetcherOptions
}

func (f *Fetcher) Fetch(fetchOptions fetcher.FetchOptions) error {
	successful = 0

	repo := f.GetFetcherOptions().Repository

	c := colly.NewCollector(
		//colly.AllowedDomains(allowedDomains...),
		//colly.DisallowedURLFilters(disallowedUrls...),
		colly.CacheDir(cacheDir),
		colly.DisallowedCacheURLFilters(disallowedCacheUrls...),
		colly.IgnoreRobotsTxt(),
		colly.MaxDepth(fetchOptions.MaxDepth),
		colly.Async(fetchOptions.Async),
	)

	if fetchOptions.Parallelism > 1 {
		c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: fetchOptions.Parallelism})
	}

	if err := c.SetStorage(repo); err != nil {
		fetchLogf("Error setting storage: %v\n", err)
		return err
	}

	articleCollector := c.Clone()

	c.OnRequest(func(r *colly.Request) {
		fetchLogf("GRABBING JSON: %s\n", r.URL.String())
	})

	c.OnResponse(func(r *colly.Response) {
		fetchLogf("RESPONSE (JSON): %s\n", r.Request.URL.String())

		var l articleList
		err := json.Unmarshal(r.Body, &l)
		if err != nil {
			fetchLogf("Error unmarshaling JSON: %v\n", err)
			return
		}

		//fmt.Println(string(r.Body))
		fmt.Printf("Total Count: %d, Total Pages: %d, Current Page: %d\n, # Articles: %d\n", l.TotalCount, l.TotalPages, l.CurrentPage, len(l.Articles))

		// collect articles in current json listing
		for _, article := range l.Articles {
			if _, ok := savedArticles[article.Id]; !ok {
				// ensure satisfaction of date filters
				pubDate, _ := time.Parse(time.RFC3339, article.Date)
				if !fetchOptions.BeforeTime.IsZero() && !pubDate.Before(fetchOptions.BeforeTime) {
					continue
				}
				if !fetchOptions.AfterTime.IsZero() && !pubDate.After(fetchOptions.AfterTime) {
					continue
				}

				// grab article
				savedArticles[article.Id] = article
				u := fmt.Sprintf("%s%d", articleBaseUrl, article.Id)

				articleCollector.Visit(r.Request.AbsoluteURL(u))
				//articleCollector.Visit(u)
			}
		}

		// get next json listing
		if l.CurrentPage < l.TotalPages {
			u, err := url.Parse(apiUrl)
			if err != nil {
				fetchLogf("Error parsing api url: %v\n", err)
				return
			}

			q := u.Query()
			q.Add("page", fmt.Sprintf("%d", l.CurrentPage+1))
			u.RawQuery = q.Encode()

			//r.Request.Visit(u.String())
			c.Visit(r.Request.AbsoluteURL(u.String()))
		}

	})

	articleCollector.OnResponse(func(r *colly.Response) {
		fetchLogf("RESPONSE (ARTICLE): %s\n", r.Request.URL.String())
		fc := &content.FetchedContent{}

		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Body))
		if err != nil {
			fetchLogf("ERROR PROCESSING RESPONSE BODY: %v\n", err)
			return
		}

		fc.Uri = r.Request.URL.String()

		// extract article id from url
		trailingId := regexp.MustCompile(`/(\d+)$`)
		groups := trailingId.FindStringSubmatch(fc.Uri)
		if len(groups) < 2 {
			fetchLogf("ERROR MATCHING ID IN URL: %v\n", err)
			return
		}
		id, err := strconv.Atoi(groups[1])
		if err != nil {
			fetchLogf("ERROR PARSING ID FROM URL: %v\n", err)
			return
		}

		// get body
		bodyText := getArticleBody(doc)

		if bodyText == "" {
			fetchLogf("FAILED (BODY): %s\n", fc.Uri)
			return
		} else {
			fc.Body = bodyText
		}

		articleListing := savedArticles[id]
		fc.Title = articleListing.Title

		dateFormat := "2006-01-02 15:04:05"
		date, _ := time.Parse(time.RFC3339, articleListing.Date)
		fc.Date = date.Format(dateFormat)

		fc.Author = articleListing.Author.Name
		fc.Abstract = articleListing.Abstract

		// process tags
		fc.Tags = make([]string, 0, len(articleListing.Tags)+len(universalTags))
		tagMap := map[string]bool{}
		for _, tag := range universalTags {
			if !tagMap[tag] {
				tagMap[tag] = true
				fc.Tags = append(fc.Tags, tag)
			}
		}
		for _, articleTagMap := range articleListing.Tags {
			for _, tag := range articleTagMap {
				if !tagMap[tag] {
					tagMap[tag] = true
					fc.Tags = append(fc.Tags, tag)
				}
			}
		}

		fc.CanonName = canonName
		fc.Language = language

		repo.SaveContent(fc)

		fetchLogf("SUCCESS: %s\n", fc.Uri)
		successful++
	})

	c.OnError(func(r *colly.Response, err error) {
		fetchLogf("ERROR: %v\n", err)
	})

	if fetchOptions.DeparturePoint != "" {
		c.Visit(fetchOptions.DeparturePoint)
	} else {
		c.Visit(defaultDeparturePoint)
	}

	if fetchOptions.Async {
		c.Wait()
	}

	fetchLogf("TOTAL SUCCESSFUL: %d\n", successful)

	return nil
}

// Return Time representation of article's publication date.
// Format:
// <meta property="article:published_time" content="2019-07-20T23:17:00+08:00">
func getArticleDate(doc *goquery.Document) time.Time {
	date := time.Time{}

	dateSelection := doc.Find(`meta[property="article:published_time"]`)
	dateSelection.Each(func(_ int, s *goquery.Selection) {
		rawDate, exists := s.Attr("content")
		if exists {
			date, _ = time.Parse(time.RFC3339, rawDate)
		}
	})

	return date
}

// Return article abstract as a string.
// Format:
// <meta property="og:description" content="...">
func getArticleAbstract(doc *goquery.Document) string {
	var abstract string

	abstractSelection := doc.Find(`meta[property="og:description"]`)
	abstractSelection.Each(func(_ int, s *goquery.Selection) {
		metaAbstract, exists := s.Attr("content")
		if exists {
			abstract = metaAbstract
		}
	})

	return abstract
}

// Return article title as a string.
// Format:
// <meta property="og:title" content="...｜..." />
func getArticleTitle(doc *goquery.Document) string {
	var title string
	//trailerRegex := regexp.MustCompile("｜女人迷 Womany$")
	trailerRegex := regexp.MustCompile("｜[^｜]+$")

	titleSelection := doc.Find(`meta[property="og:title"]`)
	titleSelection.Each(func(_ int, s *goquery.Selection) {
		titleText, exists := s.Attr("content")
		if exists {
			title = trailerRegex.ReplaceAllString(titleText, "")
		}
	})

	return title
}

// Get tags from article.
// Format:
// <meta name="keywords" content="愛情,結婚,單身,失戀,分手,自己,電影,難過,單身日記,不是愛情,夏天,是">
func getArticleTags(doc *goquery.Document) []string {
	tags := []string{}

	keywordsSelection := doc.Find(`meta[name="keywords"]`)
	keywordsSelection.Each(func(_ int, s *goquery.Selection) {
		tagString, exists := s.Attr("content")
		if exists {
			for _, tag := range strings.Split(tagString, ",") {
				trimTag := strings.TrimSpace(tag)
				if trimTag != "" {
					tags = append(tags, trimTag)
				}
			}
		}
	})

	return tags
}

// Get author.
// `.article-author h3`
func getAuthor(doc *goquery.Document) string {
	var author string

	authorSelection := doc.Find(`h3[itemprop="name"]`)
	authorSelection.Each(func(_ int, s *goquery.Selection) {
		author = s.Text()
	})

	return author
}

// Get article body and return it as a string.
func getArticleBody(doc *goquery.Document) string {
	bodySelector := doc.Find(`section[itemprop="articleBody"]`).First()
	if bodySelector.Length() == 0 {
		return ""
	}

	bodySelector.Find(`p[class="with_img"]`).Remove()
	paraSelectors := bodySelector.Find(`p, h1, h2, h3, h4, h5, h6`)
	paras := []string{}
	paraSelectors.Each(func(_ int, s *goquery.Selection) {
		trimmedText := strings.TrimSpace(s.Text())
		if trimmedText != "" {
			paras = append(paras, trimmedText)
		}
	})

	return strings.Join(paras, "\n\n")
}

func processArticle(r *colly.Response, doc *goquery.Document) (*content.FetchedContent, error) {
	fetchLogf("PROCESS: %s\n", r.Request.URL.String())
	fc := &content.FetchedContent{}

	var err error
	if doc == nil {
		doc, err = goquery.NewDocumentFromReader(bytes.NewReader(r.Body))
		if err != nil {
			fetchLogf("ERROR PROCESSING RESPONSE BODY: %v\n", err)
			return nil, errors.New("DOCPARSE")
		}
	}

	fc.Uri = r.Request.URL.String()

	// TITLE
	title := getArticleTitle(doc)
	if title == "" {
		fetchLogf("FAILED (TITLE): %s\n", fc.Uri)
		return nil, errors.New("TITLE")
	} else {
		fc.Title = title
	}

	// PUBLICATION DATE
	dateFormat := "2006-01-02 15:04:05"
	date := getArticleDate(doc)

	if date.IsZero() {
		fc.Date = time.Now().Format(dateFormat)
	} else {
		fc.Date = date.Format(dateFormat)
	}

	// AUTHOR
	author := getAuthor(doc)
	if author == "" {
		fc.Author = "Womany"
	} else {
		fc.Author = author
	}

	// ABSTRACT
	abstract := getArticleAbstract(doc)
	if abstract == "" {
		fc.Abstract = fc.Title
	} else {
		fc.Abstract = abstract
	}

	// TAGS
	tags := getArticleTags(doc)

	// Add default media tags
	for _, tag := range universalTags {
		tags = append(tags, tag)
	}

	// Filter unique tags
	tagMap := map[string]bool{}
	for _, tag := range tags {
		if !tagMap[tag] {
			tagMap[tag] = true
			fc.Tags = append(fc.Tags, tag)
		}
	}

	// CANON NAME
	fc.CanonName = canonName

	// BODY
	bodyText := getArticleBody(doc)

	if bodyText == "" {
		fetchLogf("FAILED (BODY): %s\n", fc.Uri)
		return nil, errors.New("BODY")
	} else {
		fc.Body = bodyText
	}

	// LANGUAGE
	fc.Language = language

	fetchLogf("SUCCESS: %s\n", fc.Uri)
	successful++
	return fc, nil
}
