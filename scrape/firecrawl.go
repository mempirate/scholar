package scrape

import (
	"net/url"
	"time"

	"github.com/mempirate/scholar/document"

	"github.com/mendableai/firecrawl-go"
	"github.com/pkg/errors"
)

const FIRECRAWL_API = "https://api.firecrawl.dev"

// FirecrawlScraper is a scraper that uses the Firecrawl API to scrape web pages.
type FirecrawlScraper struct {
	app *firecrawl.FirecrawlApp

	params *firecrawl.ScrapeParams
}

func NewFirecrawlScraper(key string) (*FirecrawlScraper, error) {
	app, err := firecrawl.NewFirecrawlApp(key, FIRECRAWL_API)
	if err != nil {
		return nil, err
	}

	scrapePDF := true
	timeout := 90_000

	defaultParams := &firecrawl.ScrapeParams{
		Formats:  []string{"markdown", "links"},
		ParsePDF: &scrapePDF,
		Timeout:  &timeout,
	}

	return &FirecrawlScraper{
		app:    app,
		params: defaultParams,
	}, nil
}

// Scrape scrapes the given URL and returns a Document.
//
//	type FirecrawlDocumentMetadata struct {
//		Title             *string   `json:"title,omitempty"`
//		Description       *string   `json:"description,omitempty"`
//		Language          *string   `json:"language,omitempty"`
//		Keywords          *string   `json:"keywords,omitempty"`
//		Robots            *string   `json:"robots,omitempty"`
//		OGTitle           *string   `json:"ogTitle,omitempty"`
//		OGDescription     *string   `json:"ogDescription,omitempty"`
//		OGURL             *string   `json:"ogUrl,omitempty"`
//		OGImage           *string   `json:"ogImage,omitempty"`
//		OGAudio           *string   `json:"ogAudio,omitempty"`
//		OGDeterminer      *string   `json:"ogDeterminer,omitempty"`
//		OGLocale          *string   `json:"ogLocale,omitempty"`
//		OGLocaleAlternate []*string `json:"ogLocaleAlternate,omitempty"`
//		OGSiteName        *string   `json:"ogSiteName,omitempty"`
//		OGVideo           *string   `json:"ogVideo,omitempty"`
//		DCTermsCreated    *string   `json:"dctermsCreated,omitempty"`
//		DCDateCreated     *string   `json:"dcDateCreated,omitempty"`
//		DCDate            *string   `json:"dcDate,omitempty"`
//		DCTermsType       *string   `json:"dctermsType,omitempty"`
//		DCType            *string   `json:"dcType,omitempty"`
//		DCTermsAudience   *string   `json:"dctermsAudience,omitempty"`
//		DCTermsSubject    *string   `json:"dctermsSubject,omitempty"`
//		DCSubject         *string   `json:"dcSubject,omitempty"`
//		DCDescription     *string   `json:"dcDescription,omitempty"`
//		DCTermsKeywords   *string   `json:"dctermsKeywords,omitempty"`
//		ModifiedTime      *string   `json:"modifiedTime,omitempty"`
//		PublishedTime     *string   `json:"publishedTime,omitempty"`
//		ArticleTag        *string   `json:"articleTag,omitempty"`
//		ArticleSection    *string   `json:"articleSection,omitempty"`
//		SourceURL         *string   `json:"sourceURL,omitempty"`
//		StatusCode        *int      `json:"statusCode,omitempty"`
//		Error             *string   `json:"error,omitempty"`
//	}
//
// <https://www.firecrawl.dev/blog/mastering-firecrawl-scrape-endpoint>
func (s *FirecrawlScraper) Scrape(url *url.URL) (*document.Document, error) {
	fcDoc, err := s.app.ScrapeURL(url.String(), s.params)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to scrape URL %s", url.String())
	}

	md := fcDoc.Metadata

	// Attempt to set the title (OGTitle first, then Title)
	// If this fails, the title will be empty. This is usually the case
	// with PDFs. The title can be extracted from the PDF content using `document.FindTitle()`.
	var title string
	if md.OGTitle != nil {
		title = *md.OGTitle
	} else if md.Title != nil {
		title = *md.Title
	}

	description := new(string)
	if md.Description != nil {
		description = md.Description
	} else if md.OGDescription != nil {
		description = md.OGDescription
	}

	var source string
	if md.SourceURL != nil {
		source = *md.SourceURL
	}

	doc := &document.Document{
		Content: []byte(fcDoc.Markdown),
		Metadata: document.Metadata{
			Title:         title,
			Description:   description,
			Keywords:      []string{}, // TODO: with extract
			Authors:       []string{}, // TODO: use extract API
			Source:        source,
			SiteName:      md.OGSiteName,
			PublishedTime: md.PublishedTime,
			ModifiedTime:  md.ModifiedTime,
			ProcessedTime: time.Now().Format(time.RFC3339),
			Links:         fcDoc.Links,
		},
	}

	// Attempt to find the title in the document content
	doc.FindTitle()

	var ty document.Type
	if doc.Metadata.Title == "" {
		// TODO: Simple rule, might not always work
		ty = document.TypePDF
	} else {
		ty = document.TypeArticle
	}

	doc.Metadata.Type = ty

	return doc, nil
}
