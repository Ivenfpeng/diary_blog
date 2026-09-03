package content

import (
	"reflect"
	"strings"
	"testing"
)

func TestRendererCreatesDeterministicHeadingIDs(t *testing.T) {
	renderer := NewRenderer()

	result, err := renderer.Render("# SQLite 数据库\n\n## 查询计划\n\n# SQLite 数据库")
	if err != nil {
		t.Fatal(err)
	}

	want := []Heading{
		{Level: 1, ID: "sqlite-数据库", Text: "SQLite 数据库"},
		{Level: 2, ID: "查询计划", Text: "查询计划"},
		{Level: 1, ID: "sqlite-数据库-2", Text: "SQLite 数据库"},
	}
	if !reflect.DeepEqual(result.Headings, want) {
		t.Fatalf("headings = %#v, want %#v", result.Headings, want)
	}
	for _, heading := range want {
		if !strings.Contains(result.HTML, `id="`+heading.ID+`"`) {
			t.Errorf("HTML missing heading id %q: %s", heading.ID, result.HTML)
		}
	}
}

func TestRendererHighlightsCodeAndRemovesUnsafeHTML(t *testing.T) {
	renderer := NewRenderer()
	markdown := "# Title\n\n<script>alert(1)</script>\n\n[j](javascript:alert(1))\n\n```go\nrouter := chi.Router{}\n```"

	result, err := renderer.Render(markdown)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(result.HTML, "<script") || strings.Contains(result.HTML, "javascript:") {
		t.Fatalf("unsafe HTML: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, `class="chroma"`) {
		t.Fatalf("highlighted code missing Chroma class: %s", result.HTML)
	}
	if !strings.Contains(result.PlainText, "chi.Router") {
		t.Fatalf("plain text missing code identifier: %q", result.PlainText)
	}
}

func TestRendererCalculatesReadingTimeFromPlainText(t *testing.T) {
	renderer := NewRenderer()
	markdown := strings.Repeat("字", 501)

	result, err := renderer.Render(markdown)
	if err != nil {
		t.Fatal(err)
	}

	if result.PlainText != markdown {
		t.Fatalf("plain text length = %d, want %d", len([]rune(result.PlainText)), len([]rune(markdown)))
	}
	if result.ReadingMinutes != 2 {
		t.Fatalf("reading minutes = %d, want 2", result.ReadingMinutes)
	}
}

func TestRendererRejectsMarkdownOverTwoMiB(t *testing.T) {
	renderer := NewRenderer()

	_, err := renderer.Render(strings.Repeat("a", 2*1024*1024+1))
	if err == nil {
		t.Fatal("expected size limit error")
	}
}
