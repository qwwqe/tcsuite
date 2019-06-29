package fetcher

/***
Liberty Times html page outline follows.
Title: always the only text within the only <h1></h1> tags.
Date: <meta name="pubdate" content="___">
Author: <meta name="author" content="___">
Abstract: <meta name="description" content="___">
Body:
Tags: <meta name="keywords" content="___,...,___">
CanonName: 自由時報
Uri:


***/

type LibertyFetcher struct{}

func (f LibertyFetcher) Fetch(n int, state interface{}) ([]FetchedContent, error) {
	return []FetchedContent{}, nil
}

func (f LibertyFetcher) FetchByUri(Uri string) (FetchedContent, error) {
	return FetchedContent{}, nil
}

func (f LibertyFetcher) Name() string {
	return "自由時報"
}
