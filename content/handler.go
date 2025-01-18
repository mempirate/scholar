package content

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/mempirate/scholar/document"
	"github.com/mempirate/scholar/scrape"
	"github.com/mempirate/scholar/util"
)

type ContentHandler struct {
	twitterRegex *regexp.Regexp
	githubRegex  *regexp.Regexp
	scraper      *scrape.FirecrawlScraper
}

func NewContentHandler(scraper *scrape.FirecrawlScraper) *ContentHandler {
	twitterRegex := regexp.MustCompile(`(?i)https?://(www\.)?(twitter\.com|x\.com)/\w+/status/(\d+)`)
	githubRegex := regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+)/blob/([^/]+)/(.+\.md)$`)
	return &ContentHandler{
		twitterRegex: twitterRegex,
		githubRegex:  githubRegex,
		scraper:      scraper,
	}
}

// HandleURL downloads the content from the given URL and returns it.
func (h *ContentHandler) HandleURL(uri *url.URL) (*document.Document, error) {
	if h.twitterRegex.MatchString(uri.String()) {
		// Tweet
		id, err := h.extractTweetID(uri.String())
		if err != nil {
			return nil, errors.New("failed to extract tweet ID")
		}

		return scrape.GetTweet(id)
	} else if h.githubRegex.MatchString(uri.String()) {
		return h.handleGithubMarkdown(uri)
	} else {
		return h.scraper.Scrape(uri)
	}
}

func getRawGithubURL(url *url.URL) (*url.URL, error) {
	// Replace the domain and remove the "/blob/" segment
	rawURL := strings.Replace(url.String(), "github.com", "raw.githubusercontent.com", 1)
	rawURL = strings.Replace(rawURL, "/blob/", "/", 1)

	return url.Parse(rawURL)
}

func (h *ContentHandler) handleGithubMarkdown(url *url.URL) (*document.Document, error) {
	fileName := url.Path[strings.LastIndex(url.Path, "/")+1:]
	matches := h.githubRegex.FindStringSubmatch(url.String())
	repo := matches[2]
	rawUrl, err := getRawGithubURL(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get raw URL for %s", url)
	}

	title := strings.TrimSuffix(fileName, ".md")

	body, ct, err := util.DownloadContent(rawUrl)
	if err != nil {
		return nil, err
	}

	if ct != "text/plain" {
		return nil, errors.New("invalid content type for GitHub markdown")
	}

	siteName := "GitHub"

	return &document.Document{
		Content: body,
		Metadata: document.Metadata{
			Title:         title,
			Authors:       []string{repo},
			Source:        url.String(),
			Type:          document.TypeArticle,
			SiteName:      &siteName,
			ProcessedTime: time.Now().Format(time.RFC3339),
		},
	}, nil
}

// ExtractTweetID extracts the tweet ID from a given Twitter or X URL.
func (h *ContentHandler) extractTweetID(url string) (string, error) {
	matches := h.twitterRegex.FindStringSubmatch(url)
	if len(matches) < 4 {
		return "", fmt.Errorf("no tweet ID found in URL: %s", url)
	}

	return matches[3], nil
}
