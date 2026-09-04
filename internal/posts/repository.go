package posts

import (
	"context"
	"errors"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/content"
)

var ErrNotFound = errors.New("post not found")
var ErrConflict = errors.New("post update conflict")
var ErrValidation = errors.New("post validation failed")

type PublishedFilter struct {
	CategorySlug string
	TagSlug      string
	Year         int
	Page         int
	PageSize     int
}

type AdminFilter struct {
	Status   content.PostStatus
	Query    string
	Page     int
	PageSize int
}

type Repository interface {
	CreateDraft(context.Context, content.PostInput, time.Time) (content.Post, error)
	GetByID(context.Context, int64) (content.Post, error)
	SaveDraft(context.Context, int64, content.PostInput, int64, time.Time) (content.Post, error)
	ListRevisions(context.Context, int64) ([]content.PostRevision, error)
	RestoreRevision(context.Context, int64, int64, int64, time.Time) (content.Post, error)
	Publish(context.Context, int64, content.RenderedContent, int64, time.Time) (content.Post, error)
	Archive(context.Context, int64, int64, time.Time) (content.Post, error)
	GetPublishedBySlug(context.Context, string) (content.Post, error)
	ListPublished(context.Context, PublishedFilter) ([]content.Post, int, error)
	SearchPublished(context.Context, string, int, int) ([]content.Post, int, error)
	RebuildSearch(context.Context) error
	ListAdmin(context.Context, AdminFilter) ([]content.Post, int, error)
}
