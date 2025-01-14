package slack

import (
	"regexp"
	"testing"
)

func TestRegex(t *testing.T) {
	tests := []string{
		"https://example.com",
		"https://layerzero.network/publications/QMDB_13Jan2025_v1.0.pdf",
		"http://test.com/path_with_underscores",
		"https://domain.com/file-name_123.pdf",
	}

	regex := regexp.MustCompile(URL_REGEX)
	for _, url := range tests {
		if !regex.MatchString(url) {
			t.Errorf("URL_REGEX failed to match: %s", url)
		}
	}
}
