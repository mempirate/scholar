package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/pkg/errors"
	"golang.org/x/net/html"

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
		body, ct, err := util.DownloadContent(uri)
		if err != nil {
			return nil, err
		}

		switch ct {
		case "application/pdf":
			return h.handlePDF(uri, body)
		case "text/html":
			return h.handleArticle(uri, body)
		default:
			return nil, errors.Wrapf(err, "unsupported content type: %s", ct)
		}
	}
}

func (h *ContentHandler) handlePDF(url *url.URL, body []byte) (*Content, error) {
	// By default, fileName is the last part of the URL path.
	fileName := url.Path[strings.LastIndex(url.Path, "/")+1:]

	// If there is no path, set filename to the host
	if fileName == "" {
		host := strings.TrimPrefix(url.Host, "www.")
		fileName = strings.ReplaceAll(host, ".", "-")
	}

	if !strings.HasSuffix(fileName, ".pdf") {
		fileName += ".pdf"
	}

	const pdfMagicNumber = "%PDF-"

	if !bytes.HasPrefix(body, []byte(pdfMagicNumber)) {
		return nil, errors.New("invalid magic on PDF")
	}

	return &Content{
		Type:    TypePDF,
		Name:    fileName,
		URL:     url,
		Content: body,
	}, nil
}

// handleArticle converts the HTML webpage to a Markdown file, and returns that
// as the content.
func (h *ContentHandler) handleArticle(url *url.URL, body []byte) (*Content, error) {
	r := bytes.NewReader(body)
	doc, err := html.Parse(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse HTML")
	}

	fileName, _ := extractTitle(doc)

	if fileName == "" {
		path := strings.Split(url.Path, "/")
		reverseArray(path)

		for _, segment := range path {
			if segment != "" && !isNumber(segment) {
				fileName = segment
				break
			}
		}
	}

	if fileName == "" {
		host := strings.TrimPrefix(url.Host, "www.")
		fileName = strings.ReplaceAll(host, ".", "-")
	}

	fileName += ".md"

	r = bytes.NewReader(body)
	mdBody, err := md.ConvertReader(r, converter.WithDomain(url.Host))
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert HTML to Markdown")
	}

	return &Content{
		Type:    TypeArticle,
		Name:    fileName,
		URL:     url,
		Content: mdBody,
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

func isTitleElement(n *html.Node) bool {
	return n.Type == html.ElementNode && n.Data == "title"
}

func extractTitle(n *html.Node) (string, bool) {
	if isTitleElement(n) {
		return n.FirstChild.Data, true
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result, ok := extractTitle(c)
		if ok {
			return sanitizeFileName(result), ok
		}
	}

	return "", false
}

func sanitizeFileName(name string) string {
	re := regexp.MustCompile(`[\/\\:\*\?"<>\|\p{C}]`)

	name = re.ReplaceAllString(name, "-")
	return strings.Trim(name, " .")
}

func reverseArray(arr []string) {
	left := 0
	right := len(arr) - 1
	for left < right {
		// Swap elements at left and right indices
		arr[left], arr[right] = arr[right], arr[left]
		left++
		right--
	}
}

func isNumber(str string) bool {
	if _, err := strconv.Atoi(str); err != nil {
		return true
	}

	if _, err := strconv.ParseFloat(str, 64); err != nil {
		return true
	}

	return false
}
