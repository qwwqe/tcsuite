package fetcher

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	//"github.com/gocolly/colly"
	"github.com/qwwqe/colly"
	"github.com/qwwqe/tcsuite/content"
)

type WhoGovernsTwFetcher struct {
	FetcherOptions *FetcherOptions
}

var wgtDomain = "whogovernstw.org"

var wgtUniversalTags = []string{
	"菜市場政治學",
	"政治分析",
	"政治評論",
	"評論",
}

var wgtDefaultDeparturePoint = "https://whogovernstw.org/"
var wgtCanonName = "菜市場政治學"

var wgtCacheDir = "./cache/whogovernstw_cache"

var wgtSuccessful = 0

func wgtFetchLogf(format string, a ...interface{}) (n int, err error) {
	return fmt.Printf("[WHOGOVERNSTW FETCHER] "+format, a...)
}

func (f *WhoGovernsTwFetcher) SetFetcherOptions(fetcherOptions *FetcherOptions) {
	f.FetcherOptions = fetcherOptions
}

func (f *WhoGovernsTwFetcher) GetFetcherOptions() *FetcherOptions {
	return f.FetcherOptions
}

func (f *WhoGovernsTwFetcher) Fetch(fetchOptions FetchOptions) error {
	wgtSuccessful = 0

	repo := f.GetFetcherOptions().Repository

	c := colly.NewCollector(
		colly.AllowedDomains(wgtDomain),
		colly.CacheDir(wgtCacheDir),
		colly.IgnoreRobotsTxt(),
		colly.MaxDepth(fetchOptions.MaxDepth),
		colly.Async(fetchOptions.Async),
	)

	if fetchOptions.Parallelism > 1 {
		c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: fetchOptions.Parallelism})
	}

	if err := c.SetStorage(repo); err != nil {
		wgtFetchLogf("Error setting storage: %v\n", err)
		return err
	}

	c.OnRequest(func(r *colly.Request) {
		wgtFetchLogf("VISITING: %s\n", r.URL.String())
	})

	c.OnResponse(func(r *colly.Response) {
		url := r.Request.URL.String()
		wgtFetchLogf("RESPONSE: %s\n", url)

		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Body))
		if err != nil {
			wgtFetchLogf("ERROR PROCESSING RESPONSE BODY: %v\n", err)
			return
		}

		// Filter response by url
		if !wgtIsArticle(doc) {
			return
		}

		// Filter response by date
		articleDate := wgtGetArticleDate(doc)

		if !fetchOptions.BeforeTime.IsZero() && !articleDate.Before(fetchOptions.BeforeTime) {
			return
		}

		if !fetchOptions.AfterTime.IsZero() && !articleDate.After(fetchOptions.AfterTime) {
			return
		}

		fc, err := wgtProcessArticle(r, doc)
		if err == nil {
			repo.SaveContent(fc)
		}
	})

	c.OnHTML("html", func(e *colly.HTMLElement) {
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(e.Response.Body))
		if err != nil {
			wgtFetchLogf("ERROR PROCESSING RESPONSE BODY: %v\n", err)
			return
		}

		// colly's Visit() method does not support unspecified ('//' - http or https) notation,
		// so add the schema before calling Visit()
		hrefSelection := doc.Find(`a[href]`)
		hrefSelection.Each(func(_ int, s *goquery.Selection) {
			link, _ := s.Attr("href")
			origUrl, err := url.Parse(link)
			if err != nil {
				wgtFetchLogf("ERROR: %v (%s)\n", err, link)
				return
			}
			origUrl.Scheme = "https"
			e.Request.Visit(origUrl.String())
		})
	})

	c.OnError(func(r *colly.Response, err error) {
		wgtFetchLogf("ERROR: %v\n", err)
	})

	if fetchOptions.DeparturePoint != "" {
		c.Visit(fetchOptions.DeparturePoint)
	} else {
		c.Visit(wgtDefaultDeparturePoint)
	}

	if fetchOptions.Async {
		c.Wait()
	}

	wgtFetchLogf("TOTAL SUCCESSFUL: %d\n", wgtSuccessful)

	return nil
}

func wgtIsArticle(d *goquery.Document) bool {
	articleSelection := d.Find(`article.post`)
	return articleSelection.Length() > 0
}

func wgtProcessArticle(r *colly.Response, doc *goquery.Document) (*content.FetchedContent, error) {
	wgtFetchLogf("PROCESS: %s\n", r.Request.URL.String())
	fc := &content.FetchedContent{}

	var err error
	if doc == nil {
		doc, err = goquery.NewDocumentFromReader(bytes.NewReader(r.Body))
		if err != nil {
			wgtFetchLogf("ERROR PROCESSING RESPONSE BODY: %v\n", err)
			return nil, errors.New("DOCPARSE")
		}
	}

	fc.Uri = r.Request.URL.String()

	// TITLE
	title := wgtGetArticleTitle(doc)
	if title == "" {
		wgtFetchLogf("FAILED (TITLE): %s\n", fc.Uri)
		return nil, errors.New("TITLE")
	} else {
		fc.Title = title
	}

	// PUBLICATION DATE
	dateFormat := "2006-01-02 15:04:05"
	date := wgtGetArticleDate(doc)

	if date.IsZero() {
		fc.Date = time.Now().Format(dateFormat)
	} else {
		fc.Date = date.Format(dateFormat)
	}

	// AUTHOR
	author := wgtGetAuthor(doc)
	if author == "" {
		fc.Author = "菜市場政治學"
	} else {
		fc.Author = author
	}

	// ABSTRACT
	abstract := wgtGetArticleAbstract(doc)
	if abstract == "" {
		fc.Abstract = fc.Title
	} else {
		fc.Abstract = abstract
	}

	// TAGS
	tags := wgtGetArticleTags(doc)

	// Add default media tags
	for _, tag := range wgtUniversalTags {
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
	fc.CanonName = wgtCanonName

	// BODY
	bodyText := wgtGetArticleBody(doc)

	if bodyText == "" {
		wgtFetchLogf("FAILED (BODY): %s\n", fc.Uri)
		return nil, errors.New("BODY")
	} else {
		fc.Body = bodyText
	}

	wgtFetchLogf("SUCCESS: %s\n", fc.Uri)
	wgtSuccessful++
	return fc, nil
}

// Return Time representation of article's publication date.
// Format:
// <meta property="article:published_time" content="2019-06-22T05:22:32+00:00" />
func wgtGetArticleDate(doc *goquery.Document) time.Time {
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
// This is usually found in the 'content' attribute of a metatag named 'og:description'.
func wgtGetArticleAbstract(doc *goquery.Document) string {
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
// <meta property="og:title" content="..." />
func wgtGetArticleTitle(doc *goquery.Document) string {
	var title string

	titleSelection := doc.Find(`meta[property="og:title"]`)
	titleSelection.Each(func(_ int, s *goquery.Selection) {
		titleText, exists := s.Attr("content")
		if exists {
			title = titleText
		}
	})

	return title
}

// Get tags from article.
// <meta name='shareaholic:keywords' content='中國夢, 中華, 大外宣, 新中國, 法治, 葉明叡, post' />
func wgtGetArticleTags(doc *goquery.Document) []string {
	tags := []string{}

	keywordsSelection := doc.Find(`meta[name="shareaholic:keywords"]`)
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
// <meta name='shareaholic:article_author_name' content='陳 方隅' />
func wgtGetAuthor(doc *goquery.Document) string {
	var author string

	authorSelection := doc.Find(`meta[name="shareaholic:article_author_name"]`)
	authorSelection.Each(func(_ int, s *goquery.Selection) {
		authorString, exists := s.Attr("content")
		if exists {
			author = authorString
		}
	})

	return author
}

// Get article body and return it as a string.
func wgtGetArticleBody(doc *goquery.Document) string {
	bodySelector := doc.Find(`article .entry-content`).First()
	if bodySelector.Length() == 0 {
		return ""
	}

	bodySelector.Find(`span`).Remove()
	bodySelector.Find(`.img-cap, .footnotes, .extra-hatom-entry-title, .shareaholic-canvas, .tags`).Remove()
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
