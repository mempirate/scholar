package scrape

import "net/url"

// Scraper is an interface for scraping web pages, that returns markdown content and metadata.
type Scraper interface {
	Scrape(url *url.URL, depth int)
}
