package content

import "time"

type PostStatus string

const (
	StatusDraft     PostStatus = "draft"
	StatusPublished PostStatus = "published"
	StatusArchived  PostStatus = "archived"
)

type Post struct {
	ID           int64
	Slug         string
	Title        string
	Summary      string
	ContentMD    string
	ContentHTML  string
	ContentPlain string
	Status       PostStatus
	CategoryID   *int64
	CoverMediaID *int64
	TagIDs       []int64
	Revision     int64
	PublishedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PostInput struct {
	Slug         string
	Title        string
	Summary      string
	ContentMD    string
	CategoryID   *int64
	CoverMediaID *int64
	TagIDs       []int64
}

type PostRevision struct {
	ID         int64
	PostID     int64
	Revision   int64
	Title      string
	Slug       string
	Summary    string
	ContentMD  string
	CategoryID *int64
	TagIDs     []int64
	CreatedAt  time.Time
}

type Heading struct {
	Level int
	ID    string
	Text  string
}

type RenderedContent struct {
	HTML           string
	PlainText      string
	Headings       []Heading
	ReadingMinutes int
}
