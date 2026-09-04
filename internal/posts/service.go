package posts

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Ivenfpeng/diary_blog/internal/content"
)

const maxContentBytes = 2 * 1024 * 1024

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Service struct {
	repo     Repository
	renderer *content.Renderer
	clock    func() time.Time
}

func NewService(repo Repository, renderer *content.Renderer, clock func() time.Time) *Service {
	if renderer == nil {
		renderer = content.NewRenderer()
	}
	if clock == nil {
		clock = time.Now
	}
	return &Service{repo: repo, renderer: renderer, clock: clock}
}

func (s *Service) Preview(markdown string) (content.RenderedContent, error) {
	rendered, err := s.renderer.Render(markdown)
	if errors.Is(err, content.ErrMarkdownTooLarge) {
		return content.RenderedContent{}, validationError("content exceeds 2 MiB")
	}
	return rendered, err
}

func (s *Service) CreateDraft(ctx context.Context, input content.PostInput) (content.Post, error) {
	if err := validateInput(input, false); err != nil {
		return content.Post{}, err
	}
	return s.repo.CreateDraft(ctx, input, s.clock())
}

func (s *Service) Get(ctx context.Context, id int64) (content.Post, error) {
	if err := validateID(id, "post id"); err != nil {
		return content.Post{}, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *Service) SaveDraft(ctx context.Context, id int64, input content.PostInput, expectedRevision int64) (content.Post, error) {
	if err := validateID(id, "post id"); err != nil {
		return content.Post{}, err
	}
	if err := validateRevision(expectedRevision); err != nil {
		return content.Post{}, err
	}
	if err := validateInput(input, false); err != nil {
		return content.Post{}, err
	}
	return s.repo.SaveDraft(ctx, id, input, expectedRevision, s.clock())
}

func (s *Service) ListAdmin(ctx context.Context, filter AdminFilter) ([]content.Post, int, error) {
	if err := validatePage(filter.Page, filter.PageSize); err != nil {
		return nil, 0, err
	}
	return s.repo.ListAdmin(ctx, filter)
}

func (s *Service) ListRevisions(ctx context.Context, id int64) ([]content.PostRevision, error) {
	if err := validateID(id, "post id"); err != nil {
		return nil, err
	}
	return s.repo.ListRevisions(ctx, id)
}

func (s *Service) RestoreRevision(ctx context.Context, id, revisionID, expectedRevision int64) (content.Post, error) {
	if err := validateID(id, "post id"); err != nil {
		return content.Post{}, err
	}
	if err := validateID(revisionID, "revision id"); err != nil {
		return content.Post{}, err
	}
	if err := validateRevision(expectedRevision); err != nil {
		return content.Post{}, err
	}
	return s.repo.RestoreRevision(ctx, id, revisionID, expectedRevision, s.clock())
}

func (s *Service) Publish(ctx context.Context, id, expectedRevision int64) (content.Post, error) {
	if err := validateID(id, "post id"); err != nil {
		return content.Post{}, err
	}
	if err := validateRevision(expectedRevision); err != nil {
		return content.Post{}, err
	}
	post, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return content.Post{}, err
	}
	if err := validateInput(content.PostInput{Slug: post.Slug, Title: post.Title, ContentMD: post.ContentMD}, true); err != nil {
		return content.Post{}, err
	}
	rendered, err := s.Preview(post.ContentMD)
	if err != nil {
		return content.Post{}, err
	}
	return s.repo.Publish(ctx, id, rendered, expectedRevision, s.clock())
}

func (s *Service) Archive(ctx context.Context, id, expectedRevision int64) (content.Post, error) {
	if err := validateID(id, "post id"); err != nil {
		return content.Post{}, err
	}
	if err := validateRevision(expectedRevision); err != nil {
		return content.Post{}, err
	}
	return s.repo.Archive(ctx, id, expectedRevision, s.clock())
}

func validateInput(input content.PostInput, requirePublishFields bool) error {
	if len(input.ContentMD) > maxContentBytes || !utf8.ValidString(input.ContentMD) {
		return validationError("content is invalid or exceeds 2 MiB")
	}
	if len(input.Title) > 300 || strings.ContainsAny(input.Title, "\r\n") {
		return validationError("title is invalid")
	}
	if input.Slug != "" && !slugPattern.MatchString(input.Slug) {
		return validationError("slug is invalid")
	}
	if requirePublishFields && strings.TrimSpace(input.Title) == "" {
		return validationError("title is required for publishing")
	}
	if requirePublishFields && input.Slug == "" {
		return validationError("slug is required for publishing")
	}
	return nil
}

func validateID(value int64, name string) error {
	if value <= 0 {
		return validationError(name + " must be positive")
	}
	return nil
}

func validateRevision(value int64) error {
	if value <= 0 {
		return validationError("expected revision must be positive")
	}
	return nil
}

func validatePage(page, pageSize int) error {
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return validationError("page must be positive and page size must be between 1 and 100")
	}
	return nil
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}
