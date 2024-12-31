package content

import (
	"net/url"

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
}

func NewContentHandler() *ContentHandler {
	return &ContentHandler{}
}

// HandleURL downloads the content from the given URL and returns it.
func (h *ContentHandler) HandleURL(uri *url.URL) (*Content, error) {
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
