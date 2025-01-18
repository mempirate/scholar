package document

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

type Type = string

const (
	TypePDF     Type = "pdf"
	TypeTweet   Type = "tweet"
	TypeArticle Type = "article"
)

// TODO: turn into YAML front matter
type Metadata struct {
	Title string `yaml:"title"`
	// Description: either Description or OGDescription
	Description *string  `yaml:"description"`
	Keywords    []string `yaml:"keywords,omitempty"`
	Authors     []string `yaml:"authors,omitempty"`
	Source      string   `yaml:"source"`
	Type        Type     `yaml:"type"`
	// OGSiteName
	SiteName      *string  `yaml:"siteName,omitempty"`
	PublishedTime *string  `yaml:"publishedTime,omitempty"`
	ModifiedTime  *string  `yaml:"modifiedTime,omitempty"`
	ProcessedTime string   `yaml:"processedTime"`
	Links         []string `yaml:"links,omitempty"`
}

type Document struct {
	// The markdown content of the scraped document.
	Content string
	// Metadata about the document.
	Metadata Metadata
}

func (d *Document) HasTitle() bool {
	return d.Metadata.Title != ""
}

func (d *Document) FindTitle() string {
	// If the title is already set, return it.
	if d.Metadata.Title != "" {
		return d.Metadata.Title
	}

	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)

	content := []byte(d.Content)
	reader := text.NewReader(content)
	doc := md.Parser().Parse(reader)

	var title string
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if heading, ok := n.(*ast.Heading); ok && entering && heading.Level == 1 {
			var titleBuilder strings.Builder
			// Walk through child nodes of the heading
			for child := heading.FirstChild(); child != nil; child = child.NextSibling() {
				if text, ok := child.(*ast.Text); ok {
					titleBuilder.Write(text.Segment.Value(content))
				}
			}
			title = titleBuilder.String()
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil // Changed from WalkStop to allow continuing if not h1
	})

	return title
}

// ToMarkdown converts the Document to a markdown string, with metadata as YAML front matter.
// It returns the filename and the markdown content, and an optional error.
func (d *Document) ToMarkdown() (string, string, error) {
	// Make sure title is set
	d.FindTitle()

	var builder strings.Builder
	frontMatter, err := yaml.Marshal(d.Metadata)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to marshal metadata to YAML")
	}

	builder.WriteString("---\n")
	builder.Write(frontMatter)
	builder.WriteString("---\n")
	builder.WriteString(d.Content)

	fileName := d.Metadata.Title + ".md"

	return fileName, builder.String(), nil
}
