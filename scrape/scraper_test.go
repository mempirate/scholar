package scrape

import (
	"net/url"
	"os"
	"testing"
)

func TestScrapeArticle(t *testing.T) {
	fc, err := NewFirecrawlScraper(os.Getenv("FIRECRAWL_API_KEY"))
	if err != nil {
		t.Fatal(err)
	}

	uri, _ := url.Parse("https://github.com/opentimestamps/opentimestamps-server/blob/master/README.md")

	doc, err := fc.Scrape(uri)
	if err != nil {
		t.Fatal(err)
	}

	_, content, err := doc.ToMarkdown()
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile("README", []byte(content), 0644)
}

func TestScrapePDF(t *testing.T) {
	fc, err := NewFirecrawlScraper(os.Getenv("FIRECRAWL_API_KEY"))
	if err != nil {
		t.Fatal(err)
	}

	uri, _ := url.Parse("https://layerzero.network/publications/QMDB_13Jan2025_v1.0.pdf")

	doc, err := fc.Scrape(uri)
	if err != nil {
		t.Fatal(err)
	}

	name, content, err := doc.ToMarkdown()
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile(name, []byte(content), 0644)
}
