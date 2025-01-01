package content

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"

	"github.com/mempirate/scholar/twitter"
	"github.com/mempirate/scholar/util"
)

type Type = string

const (
	TypePDF     Type = "pdf"
	TypeTweet   Type = "tweet"
	TypeArticle Type = "article"
)

type Content struct {
	Type    Type
	Name    string
	URL     *url.URL
	Content []byte
}

type ContentHandler struct {
	twitterRegex *regexp.Regexp
}

func NewContentHandler() *ContentHandler {
	twitterRegex := regexp.MustCompile(`(?i)https?://(www\.)?(twitter\.com|x\.com)/\w+/status/(\d+)`)
	return &ContentHandler{
		twitterRegex: twitterRegex,
	}
}

// HandleURL downloads the content from the given URL and returns it.
func (h *ContentHandler) HandleURL(uri *url.URL) (*Content, error) {
	if h.twitterRegex.MatchString(uri.String()) {
		// Tweet
		id, err := h.extractTweetID(uri.String())
		if err != nil {
			return nil, errors.New("failed to extract tweet ID")
		}

		tweet, err := twitter.GetTweet(id)
		if err != nil {
			return nil, err
		}

		content, err := json.Marshal(tweet)
		if err != nil {
			return nil, err
		}

		return &Content{
			Type:    TypeTweet,
			Name:    fmt.Sprintf("tweet-%s.json", id),
			URL:     uri,
			Content: content,
		}, nil

	} else {
		// PDF
		name, body, err := util.DownloadPDF(uri)
		if err != nil {
			return nil, err
		}

		return &Content{
			Type:    TypePDF,
			Name:    name,
			URL:     uri,
			Content: body,
		}, nil
	}
}

// ExtractTweetID extracts the tweet ID from a given Twitter or X URL.
func (h *ContentHandler) extractTweetID(url string) (string, error) {
	matches := h.twitterRegex.FindStringSubmatch(url)
	if len(matches) < 4 {
		return "", fmt.Errorf("no tweet ID found in URL: %s", url)
	}

	return matches[3], nil
}
