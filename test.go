package main

import (
	//"encoding/json"
	//	"log"
	//"os"
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
	"github.com/qwwqe/tcsuite/fetcher"
	"github.com/qwwqe/tcsuite/repository"
)

func main() {
	repo := repository.GetRepository()

	allowedDomains := []string{
		"ltn.com.tw", "www.ltn.com.tw",
		"news.ltn.com.tw",
		"ent.ltn.com.tw",
		"istyle.ltn.com.tw",
		"ec.ltn.com.tw",
		"auto.ltn.com.tw",
		"sports.ltn.com.tw",
		"3c.ltn.com.tw",
		"talk.ltn.com.tw",
		"playing.ltn.com.tw",
		"food.ltn.com.tw",
		"health.ltn.com.tw",
		"estate.ltn.com.tw",
	}

	disallowedUrls := []*regexp.Regexp{
		regexp.MustCompile("/assets/"),
		regexp.MustCompile("/print$"),
	}

	c := colly.NewCollector(
		colly.AllowedDomains(allowedDomains...),
		colly.DisallowedURLFilters(disallowedUrls...),
		colly.CacheDir("./liberty_cache"),
		colly.IgnoreRobotsTxt(),
	)

	c.OnRequest(func(r *colly.Request) {
		fmt.Printf("VISITING: %s\n", r.URL.String())
	})

	c.OnResponse(func(r *colly.Response) {
		//fmt.Println(r.Body)
		url := r.Request.URL.String()
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Body))
		if err != nil {
			fmt.Printf("CRAPO! %v\n", err)
			return
		}
		selectorString := `meta[property="og:type"][content="article"]`
		meta := doc.Find(selectorString)
		fmt.Printf("PASSED: %v (%s)\n", meta.Length() > 0, url)
	})

	// TODO: consider making content extraction more efficient (probably not a bottleneck, though...)
	//c.OnHTML(`.text[itemprop="articleBody"]`, func(e *colly.HTMLElement) {
	c.OnHTML(`meta[property="og:type"][content="article"]`, func(e *colly.HTMLElement) {
		fmt.Printf("PROCESS: %s\n", e.Request.URL.String())
		fc := &fetcher.FetchedContent{}
		top := e.DOM.ParentsUntil("~")

		fc.Uri = e.Request.URL.String()

		// Get title. The titles of Liberty Times articles tend to
		// be of the format <TITLE> - <CATEGORY> - <PRESS NAME>,
		// thus the regex below serves to remove the trailing
		// category and press name, if they are present.
		// If no <title> tag is present, skip the article (it probably
		// isn't an article).
		titleDeletionRegex, _ := regexp.Compile(" - [^-]+ - [^-]+$")

		titleSelection := top.Find(`title`)
		titleSelection.Each(func(_ int, s *goquery.Selection) {
			rawTitle := s.Text()
			title := titleDeletionRegex.ReplaceAllLiteralString(rawTitle, "")
			fc.Title = title
		})

		if fc.Title == "" {
			fmt.Println("FAILED (TITLE): %s\n", fc.Uri)
			return
		}

		// Get publication date. The publication date of articles on
		// the Liberty Times website is found in the 'content' attribute
		// of the metatag named 'pubdate'.
		// This metatag uses RFC3339 formatting to represent the timestamp,
		// so below we convert this to YYYY-MM-DD HH:MM:SS format before
		// saving it in the FetchedContent object.
		dateFormat := "2006-01-02 15:04:05"

		dateSelection := top.Find(`meta[name="pubdate"]`)
		dateSelection.Each(func(_ int, s *goquery.Selection) {
			rawDate, exists := s.Attr("content")
			if exists {
				goDate, _ := time.Parse(time.RFC3339, rawDate)
				fc.Date = goDate.Format(dateFormat)
			}
		})

		if fc.Date == "" {
			fc.Date = time.Now().Format(dateFormat)
		}

		// Get author. This is typically absent.
		fc.Author = "自由時報"

		// Get abstract. This is usually found in the 'content' attribute
		// of a metatag named 'description'.
		abstractSelection := top.Find(`meta[name="description"]`)
		abstractSelection.Each(func(_ int, s *goquery.Selection) {
			abstract, exists := s.Attr("content")
			if exists {
				fc.Abstract = abstract
			}
		})

		if fc.Abstract == "" {
			fc.Abstract = fc.Title
		}

		// Get tags. These are usually found as a comma-separated list in the
		// 'content' attribute of a metatag named 'keywords'.
		fc.Tags = []string{}
		keywordsSelection := top.Find(`meta[name="keywords"]`)
		keywordsSelection.Each(func(_ int, s *goquery.Selection) {
			tagString, exists := s.Attr("content")
			if exists {
				for _, tag := range strings.Split(tagString, ",") {
					trimTag := strings.TrimSpace(tag)
					if trimTag != "" {
						fc.Tags = append(fc.Tags, trimTag)
					}
				}
			}
		})

		// Canon name.
		fc.CanonName = "自由時報"

		// Get body. This is sometimes found in a <div> with attribute
		// itemprop="articleBody". Other times it is found in a <div>
		// with class "text".

		// We should be safe to delete all <span> elements in this <div>,
		// as well as the <p> with class "appE1121". Maybe experiement
		// with deletion of all non-<p> elements.
		bodySelector := top.Find(`div[itemprop="articleBody"]`).First()
		if bodySelector.Length() == 0 { // try <div class="text">
			bodySelector = top.Find(`div[class="text"]`).First()
		}

		if bodySelector.Length() == 0 {
			fmt.Printf("FAILED (BODY NOT FOUND): %s\n", fc.Uri)
			return
		}

		bodySelector.Find(`span`).Remove()
		bodySelector.Find(`.appE1121`).Remove()
		paras := bodySelector.Find(`p`)
		newLineRegex, _ := regexp.Compile("[\n\r]+")
		text := newLineRegex.ReplaceAllLiteralString(strings.TrimSpace(paras.Text()), "\n\n")

		if text == "" {
			fmt.Printf("FAILED (BODY): %s\n", fc.Uri)
			return
		}
		fc.Body = text

		fmt.Printf("SUCCESS: %s\n", fc.Uri)
		repo.SaveContent(fc)
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		origUrl, err := url.Parse(link)
		if err != nil {
			fmt.Printf("ERROR: %v (%s)\n")
			return
		}
		origUrl.Scheme = "https"
		c.Visit(e.Request.AbsoluteURL(origUrl.String()))
	})

	c.OnError(func(r *colly.Response, err error) {
		fmt.Printf("ERROR: %v\n", err)
	})

	// detailCollector.OnHTML("html", func(e *colly.HTMLElement) {
	// 	fc := &fetcher.FetchedContent{}

	// })

	/*
		fc := &fetcher.FetchedContent{
			Title:     "Dog Down on 32nd Street",
			Date:      "2018-11-04 23:43:23",
			Author:    "Roger Daltry",
			Abstract:  "A 9 year-old Maltese Terrier cross was downed today on 32nd street.",
			Body:      "A 9 year-old Maltese Terrier cross was downed today on 32nd street. His parents were informed of the incident.",
			Tags:      []string{"Dogs", "Death", "Tragey", "StreetBeat"},
			CanonName: "Apple Daily News",
			Uri:       "12345",
		}
		repo.SaveContent(fc)
		print(repo)*/

	//c.Visit("https://www.ltn.com.tw/")
	c.Visit("https://news.ltn.com.tw/news/local/paper/890986")

	// b := colly.NewCollector(
	// 	colly.AllowedDomains(allowedDomains...),
	// )

	// b.OnHTML("a[href]", func(e *colly.HTMLElement) {
	// 	link := e.Attr("href")
	// 	fmt.Printf("Link found: %q -> %s\n", e.Text, link)
	// 	c.Visit(e.Request.AbsoluteURL(link))
	// })

	// b.OnRequest(func(r *colly.Request) {
	// 	fmt.Println("Visiting", r.URL.String())
	// })

	// b.Visit("https://www.ltn.com.tw/")
}

/*
	Title     string
	Date      string
	Author    string
	Abstract  string
	Body      string
	Tags      []string
	CanonName string
	Uri       string

*/
