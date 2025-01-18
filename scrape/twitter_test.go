package scrape

import (
	"os"
	"testing"
)

func TestGetTweet(t *testing.T) {
	tweet, err := GetTweet("1874431128096096666")
	if err != nil {
		t.Fatal(err)
	}

	name, content, err := tweet.ToMarkdown()
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile(name, content, 0644)
}
