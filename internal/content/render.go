package content

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

const (
	maxMarkdownBytes = 2 * 1024 * 1024
	runesPerMinute   = 500
)

var ErrMarkdownTooLarge = errors.New("markdown exceeds 2 MiB limit")

type Renderer struct {
	markdown  goldmark.Markdown
	sanitizer *bluemonday.Policy
}

func NewRenderer() *Renderer {
	markdown := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
	)

	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("id").Matching(regexp.MustCompile(`^[\p{L}\p{N}_-]+$`)).OnElements(
		"h1", "h2", "h3", "h4", "h5", "h6",
	)
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^[A-Za-z0-9_ -]+$`)).OnElements(
		"div", "pre", "code", "span",
	)

	return &Renderer{markdown: markdown, sanitizer: policy}
}

func (r *Renderer) Render(markdown string) (RenderedContent, error) {
	if len(markdown) > maxMarkdownBytes {
		return RenderedContent{}, ErrMarkdownTooLarge
	}

	source := []byte(markdown)
	document := r.markdown.Parser().Parse(text.NewReader(source))
	headings, err := assignHeadingIDs(document, source)
	if err != nil {
		return RenderedContent{}, fmt.Errorf("collect headings: %w", err)
	}

	plainText, err := extractPlainText(document, source)
	if err != nil {
		return RenderedContent{}, fmt.Errorf("extract plain text: %w", err)
	}

	var rendered bytes.Buffer
	if err := r.markdown.Renderer().Render(&rendered, source, document); err != nil {
		return RenderedContent{}, fmt.Errorf("render markdown: %w", err)
	}

	minutes := (utf8.RuneCountInString(plainText) + runesPerMinute - 1) / runesPerMinute
	if minutes < 1 {
		minutes = 1
	}

	return RenderedContent{
		HTML:           r.sanitizer.Sanitize(rendered.String()),
		PlainText:      plainText,
		Headings:       headings,
		ReadingMinutes: minutes,
	}, nil
}

func assignHeadingIDs(document ast.Node, source []byte) ([]Heading, error) {
	counts := make(map[string]int)
	headings := make([]Heading, 0)

	err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := node.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}

		text := strings.TrimSpace(string(heading.Text(source)))
		baseID := headingID(text)
		counts[baseID]++
		id := baseID
		if counts[baseID] > 1 {
			id = fmt.Sprintf("%s-%d", baseID, counts[baseID])
		}
		heading.SetAttributeString("id", []byte(id))
		headings = append(headings, Heading{Level: heading.Level, ID: id, Text: text})
		return ast.WalkContinue, nil
	})
	return headings, err
}

func headingID(value string) string {
	var result strings.Builder
	separatorPending := false

	for _, char := range strings.ToLower(value) {
		switch {
		case unicode.IsLetter(char) || unicode.IsDigit(char):
			if separatorPending && result.Len() > 0 {
				result.WriteByte('-')
			}
			result.WriteRune(char)
			separatorPending = false
		default:
			separatorPending = result.Len() > 0
		}
	}
	if result.Len() == 0 {
		return "section"
	}
	return result.String()
}

func extractPlainText(document ast.Node, source []byte) (string, error) {
	var plain strings.Builder

	err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			switch node.Kind() {
			case ast.KindHeading, ast.KindParagraph, ast.KindListItem, ast.KindBlockquote:
				plain.WriteByte(' ')
			}
			return ast.WalkContinue, nil
		}

		switch current := node.(type) {
		case *ast.Text:
			plain.Write(current.Segment.Value(source))
			if current.SoftLineBreak() || current.HardLineBreak() {
				plain.WriteByte(' ')
			}
		case *ast.String:
			plain.Write(current.Value)
		case *ast.CodeBlock:
			plain.Write(current.Lines().Value(source))
			plain.WriteByte(' ')
			return ast.WalkSkipChildren, nil
		case *ast.FencedCodeBlock:
			plain.Write(current.Lines().Value(source))
			plain.WriteByte(' ')
			return ast.WalkSkipChildren, nil
		case *ast.RawHTML:
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return "", err
	}
	return strings.Join(strings.Fields(plain.String()), " "), nil
}
