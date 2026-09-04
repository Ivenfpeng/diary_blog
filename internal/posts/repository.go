package posts

import (
	"context"
	"errors"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/content"
)

var ErrNotFound = errors.New("post not found")
var ErrConflict = errors.New("post update conflict")

type Repository interface {
	CreateDraft(context.Context, content.PostInput, time.Time) (content.Post, error)
	GetByID(context.Context, int64) (content.Post, error)
	SaveDraft(context.Context, int64, content.PostInput, int64, time.Time) (content.Post, error)
	ListRevisions(context.Context, int64) ([]content.PostRevision, error)
	RestoreRevision(context.Context, int64, int64, int64, time.Time) (content.Post, error)
}
