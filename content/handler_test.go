package content

import "testing"

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
