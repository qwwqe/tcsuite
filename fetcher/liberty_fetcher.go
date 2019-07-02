package fetcher

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	//"github.com/gocolly/colly"
	"github.com/qwwqe/colly"
	"github.com/qwwqe/tcsuite/content"
)

type LibertyFetcher struct {
	FetcherOptions *FetcherOptions
}

var domains = map[string]string{
	"www.ltn.com.tw":     "新聞",
	"news.ltn.com.tw":    "新聞",
	"ent.ltn.com.tw":     "娛樂",
	"istyle.ltn.com.tw":  "時尚",
	"ec.ltn.com.tw":      "財經",
	"auto.ltn.com.tw":    "汽車",
	"sports.ltn.com.tw":  "運動",
	"3c.ltn.com.tw":      "3C",
	"talk.ltn.com.tw":    "評論",
	"playing.ltn.com.tw": "旅遊",
	"food.ltn.com.tw":    "食譜",
	"health.ltn.com.tw":  "健康",
	"estate.ltn.com.tw":  "地產",
}

var disallowedUrls = []*regexp.Regexp{
	regexp.MustCompile("/assets/"),
	regexp.MustCompile("/print$"),
	regexp.MustCompile("/m/"),
}

var disallowedCacheUrls = []*regexp.Regexp{
	regexp.MustCompile("ltn.com.tw/?$"),
	regexp.MustCompile("/list/"),
}

var universalTags = []string{
	"新聞",
	"報紙",
	"自由時報",
}

var defaultDeparturePoint = "https://www.ltn.com.tw/"
var canonName = "自由時報"
var cacheDir = "./cache/liberty_cache"

var successful = 0

func fetchLogf(format string, a ...interface{}) (n int, err error) {
	return fmt.Printf("[LIBERTY FETCHER] "+format, a...)
}

func (f *LibertyFetcher) SetFetcherOptions(fetcherOptions *FetcherOptions) {
	f.FetcherOptions = fetcherOptions
}

func (f *LibertyFetcher) GetFetcherOptions() *FetcherOptions {
	return f.FetcherOptions
}

func (f *LibertyFetcher) Fetch(fetchOptions FetchOptions) error {
	successful = 0

	repo := f.GetFetcherOptions().Repository

	allowedDomains := make([]string, 0, len(domains))
	for domain := range domains {
		allowedDomains = append(allowedDomains, domain)
	}

	c := colly.NewCollector(
		colly.AllowedDomains(allowedDomains...),
		colly.DisallowedURLFilters(disallowedUrls...),
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

	c.OnRequest(func(r *colly.Request) {
		fetchLogf("VISITING: %s\n", r.URL.String())
	})

	c.OnResponse(func(r *colly.Response) {
		url := r.Request.URL.String()
		fetchLogf("RESPONSE: %s\n", url)

		// Filter response by url
		isArticle, _ := regexp.MatchString(`/\d+$`, url)
		if !isArticle {
			return
		}

		// Filter response by date
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Body))
		if err != nil {
			fetchLogf("ERROR PROCESSING RESPONSE BODY: %v\n", err)
			return
		}
		articleDate := getArticleDate(doc)

		if !fetchOptions.BeforeTime.IsZero() && !articleDate.Before(fetchOptions.BeforeTime) {
			return
		}

		if !fetchOptions.AfterTime.IsZero() && !articleDate.After(fetchOptions.AfterTime) {
			return
		}

		fc, err := processArticle(r, doc)
		if err == nil {
			repo.SaveContent(fc)
		}
	})

	c.OnHTML("html", func(e *colly.HTMLElement) {
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(e.Response.Body))
		if err != nil {
			fetchLogf("ERROR PROCESSING RESPONSE BODY: %v\n", err)
			return
		}

		isArticle, _ := regexp.MatchString(`/\d+$`, e.Request.URL.String())
		if isArticle {
			articleDate := getArticleDate(doc)
			if !articleDate.IsZero() {
				if !fetchOptions.BeforeTime.IsZero() && !articleDate.Before(fetchOptions.BeforeTime) {
					fetchLogf("FAILED (TOO NEW)\n")
					return
				}

				if !fetchOptions.AfterTime.IsZero() && !articleDate.After(fetchOptions.AfterTime) {
					fetchLogf("FAILED (TOO OLD)\n")
					return
				}
			}
		}

		// colly's Visit() method does not support unspecified ('//' - http or https) notation,
		// so add the schema before calling Visit()
		hrefSelection := doc.Find(`a[href]`)
		hrefSelection.Each(func(_ int, s *goquery.Selection) {
			link, _ := s.Attr("href")
			origUrl, err := url.Parse(link)
			if err != nil {
				fetchLogf("ERROR: %v (%s)\n", err, link)
				return
			}
			origUrl.Scheme = "https"
			//c.Visit(e.Request.AbsoluteURL(origUrl.String()))
			e.Request.Visit(origUrl.String())
		})
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
// The publication date of articles on the Liberty Times website is
// usually found in the 'content' attribute of the metatag named 'pubdate'.
func getArticleDate(doc *goquery.Document) time.Time {
	date := time.Time{}

	dateSelection := doc.Find(`meta[name="pubdate"]`)
	dateSelection.Each(func(_ int, s *goquery.Selection) {
		rawDate, exists := s.Attr("content")
		if exists {
			date, _ = time.Parse(time.RFC3339, rawDate)
		}
	})

	return date
}

// Return article abstract as a string.
// This is usually found in the 'content' attribute of a metatag named 'description'.
func getArticleAbstract(doc *goquery.Document) string {
	var abstract string

	abstractSelection := doc.Find(`meta[name="description"]`)
	abstractSelection.Each(func(_ int, s *goquery.Selection) {
		metaAbstract, exists := s.Attr("content")
		if exists {
			abstract = metaAbstract
		}
	})

	return abstract
}

// Return article title as a string.
// The titles of Liberty Times articles tend to be formatted as
// <TITLE> - <ARBITRARY COMBINATION OF CATEGORY AND PRESS NAME>,
// where the latter combination can also be a ' - ' separated list. Here
// we assume that ' - ' won't appear in the actual article titles (it doesn't seem to)
// and simply drop everything after the first appearance.
func getArticleTitle(doc *goquery.Document) string {
	var title string
	//titleDeletionRegex, _ := regexp.Compile(" - [^-]+ - [^-]+$")
	titleDeletionRegex, _ := regexp.Compile(" - .+$")

	titleSelection := doc.Find(`title`)
	titleSelection.Each(func(_ int, s *goquery.Selection) {
		rawTitle := s.Text()
		title = strings.TrimSpace(titleDeletionRegex.ReplaceAllLiteralString(rawTitle, ""))
	})

	return title
}

// Get tags from article.
// These are usually found as a comma-separated list in the
// 'content' attribute of a metatag named 'keywords'.
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

// Get article body and return it as a string.
// This is sometimes found in a <div> with attribute
// itemprop="articleBody". Other times it is found in a <div>
// with class "text".
// We should be safe to delete all <span> elements in this <div>,
// the <p> with class "appE1121", and frankly all non-<p> elements.
func getArticleBody(doc *goquery.Document) string {
	bodySelector := doc.Find(`div[itemprop="articleBody"]`).First()
	if bodySelector.Length() == 0 { // try <div class="text">
		bodySelector = doc.Find(`div[class="text"]`).First()
	}

	if bodySelector.Length() == 0 {
		return ""
	}

	bodySelector.Find(`span`).Remove()
	bodySelector.Find(`.appE1121`).Remove()
	bodySelector.Find(`div[data-desc="圖片"]`).Remove()
	bodySelector.Find(`div[data-desc="小圖"]`).Remove()
	paraSelectors := bodySelector.Find(`p`)
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
	// If no <title> tag is present, skip the article (it probably isn't an article).
	title := getArticleTitle(doc)
	if title == "" {
		fetchLogf("FAILED (TITLE): %s\n", fc.Uri)
		return nil, errors.New("TITLE")
	} else {
		fc.Title = title
	}

	// PUBLICATION DATE
	// Liberty times uses RFC3339 formatting to represent the timestamp,
	// so below we convert this to YYYY-MM-DD HH:MM:SS format before
	// saving it in the FetchedContent object.
	dateFormat := "2006-01-02 15:04:05"
	date := getArticleDate(doc)

	if date.IsZero() {
		fc.Date = time.Now().Format(dateFormat)
	} else {
		fc.Date = date.Format(dateFormat)
	}

	// AUTHOR
	// This is typically absent from the articles themselves.
	fc.Author = "自由時報"

	// ABSTRACT
	abstract := getArticleAbstract(doc)
	if abstract == "" {
		fc.Abstract = fc.Title
	} else {
		fc.Abstract = abstract
	}

	// TAGS
	tags := getArticleTags(doc)

	// Add the tag associated to this hostname
	articleUrl, err := url.Parse(fc.Uri)
	if err == nil {
		domain := articleUrl.Hostname()
		if tag, ok := domains[domain]; ok {
			tags = append(tags, tag)
		}
	}

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

	fetchLogf("SUCCESS: %s\n", fc.Uri)
	successful++
	return fc, nil
}
