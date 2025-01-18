package content

import (
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/mempirate/scholar/document"
	"github.com/mempirate/scholar/scrape"
)

func TestRegexID(t *testing.T) {
	fc, err := scrape.NewFirecrawlScraper(os.Getenv("FIRECRAWL_API_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	h := NewContentHandler(fc)

	urls := []string{
		"https://x.com/tarunchitra/status/1874532036297490554",
		"https://twitter.com/tarunchitra/status/1874532036297490554",
		"https://www.x.com/tarunchitra/status/1874532036297490554",
	}

	for _, url := range urls {
		id, err := h.extractTweetID(url)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if id != "1874532036297490554" {
			t.Errorf("unexpected id: %s", id)
		}
	}
}

func TestHandleWebPage(t *testing.T) {
	fc, err := scrape.NewFirecrawlScraper(os.Getenv("FIRECRAWL_API_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	h := NewContentHandler(fc)

	// link := "https://collective.flashbots.net/t/the-role-of-relays-in-reorgs/4247"
	link := "https://vitalik.eth.limo/general/2025/01/05/dacc2.html"
	// t := "https://ethresear.ch/t/fork-choice-enforced-inclusion-lists-focil-a-simple-committee-based-inclusion-list-proposal/19870"

	uri, err := url.Parse(link)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	content, err := h.HandleURL(uri)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if content.Metadata.Type != document.TypeArticle {
		t.Errorf("unexpected type: %s", content.Metadata.Type)
	}

	fmt.Println(content.Metadata.Title)

	os.WriteFile(content.Metadata.Title, content.Content, 0644)
}

func TestGithubMarkdown(t *testing.T) {
	fc, err := scrape.NewFirecrawlScraper(os.Getenv("FIRECRAWL_API_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	h := NewContentHandler(fc)

	urls := []string{
		"https://github.com/opentimestamps/opentimestamps-server/blob/master/doc/merkle-mountain-range.md",
		"https://github.com/opentimestamps/opentimestamps-server/blob/master/README.md",
	}

	for i, link := range urls {
		uri, _ := url.Parse(link)

		doc, err := h.handleGithubMarkdown(uri)
		if err != nil {
			t.Fatal(err)
		}

		_, content, err := doc.ToMarkdown()
		if err != nil {
			t.Fatal(err)
		}

		os.WriteFile(fmt.Sprint(i), content, 0644)

		if doc.Metadata.Type != document.TypeArticle {
			t.Errorf("unexpected type: %s", doc.Metadata.Type)
		}

		name, content, err := doc.ToMarkdown()
		if err != nil {
			t.Fatal(err)
		}

		os.WriteFile(name, content, 0644)
	}
}
