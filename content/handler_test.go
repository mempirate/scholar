package content

import (
	"fmt"
	"net/url"
	"os"
	"testing"
)

func TestRegexID(t *testing.T) {
	h := NewContentHandler()

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
	h := NewContentHandler()

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

	if content.Type != TypeArticle {
		t.Errorf("unexpected type: %s", content.Type)
	}

	fmt.Println(content.Name)

	os.WriteFile(content.Name, content.Content, 0644)
}
