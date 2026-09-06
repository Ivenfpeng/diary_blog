# Publishing a durable release gate

The publishing workflow deserves a full-browser test because the public page
is the durable contract readers actually use.

## Why this workflow exists

The authoring flow preserves a Markdown source, a rendered preview, and a
searchable public article.

### A nested heading

Readers can use the table of contents to move through longer notes without
losing their place.

```go
func publish(note string) error {
	return nil
}
```

The release gate also checks that long code remains scrollable inside its own
container instead of making the full page wider than the viewport.
