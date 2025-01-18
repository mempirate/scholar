package document

import (
	"bytes"
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
	ID    string `yaml:"id,omitempty"`
	// Description: either Description or OGDescription
	Description *string  `yaml:"description,omitempty"`
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
	Content []byte
	// Metadata about the document.
	Metadata Metadata
}

func (d *Document) HasTitle() bool {
	return d.Metadata.Title != ""
}

func (d *Document) FileName() string {
	return d.Metadata.Title + ".md"
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
	for level := 1; level <= 6; level++ {
		found := false
		ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if heading, ok := n.(*ast.Heading); ok && entering && heading.Level == level {
				var titleBuilder strings.Builder
				for child := heading.FirstChild(); child != nil; child = child.NextSibling() {
					if text, ok := child.(*ast.Text); ok {
						titleBuilder.Write(text.Segment.Value(content))
					}
				}
				title = titleBuilder.String()
				found = true
				return ast.WalkStop, nil
			}
			return ast.WalkContinue, nil
		})
		if found {
			break
		}
	}

	d.Metadata.Title = title
	return title
}

// ToMarkdown converts the Document to a markdown string, with metadata as YAML front matter.
// It returns the filename and the markdown content, and an optional error.
func (d *Document) ToMarkdown() (string, []byte, error) {
	// Make sure title is set
	d.FindTitle()

	var builder bytes.Buffer
	frontMatter, err := yaml.Marshal(d.Metadata)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to marshal metadata to YAML")
	}

	builder.WriteString("---\n")
	builder.Write(frontMatter)
	builder.WriteString("---\n")
	builder.Write(d.Content)

	fileName := d.Metadata.Title + ".md"

	return fileName, builder.Bytes(), nil
}
