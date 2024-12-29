package slack

import (
	"regexp"
	"testing"
)

func TestRegex(t *testing.T) {
	// Test the URL regex
	regex := regexp.MustCompile(URL_REGEX)
	if !regex.MatchString("https://example.com") {
		t.Error("URL_REGEX failed to match a URL")
	}
}
