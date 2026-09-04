package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Ivenfpeng/diary_blog/internal/content"
	"github.com/Ivenfpeng/diary_blog/internal/posts"
)

type PostRepository struct {
	db *sql.DB
}

func NewPostRepository(db *sql.DB) *PostRepository {
	return &PostRepository{db: db}
}

func (r *PostRepository) CreateDraft(ctx context.Context, input content.PostInput, now time.Time) (content.Post, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return content.Post{}, fmt.Errorf("begin create draft: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO posts (
			slug, title, summary, content_md, status, category_id, cover_media_id,
			revision, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		input.Slug, input.Title, input.Summary, input.ContentMD, content.StatusDraft,
		input.CategoryID, input.CoverMediaID, formatTime(now), formatTime(now),
	)
	if err != nil {
		return content.Post{}, fmt.Errorf("insert draft: %w", err)
	}
	postID, err := result.LastInsertId()
	if err != nil {
		return content.Post{}, fmt.Errorf("read draft id: %w", err)
	}
	if err := replaceTags(ctx, tx, postID, input.TagIDs); err != nil {
		return content.Post{}, err
	}
	post, err := getPost(ctx, tx, postID)
	if err != nil {
		return content.Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return content.Post{}, fmt.Errorf("commit create draft: %w", err)
	}
	return post, nil
}

func (r *PostRepository) GetByID(ctx context.Context, id int64) (content.Post, error) {
	return getPost(ctx, r.db, id)
}

func (r *PostRepository) SaveDraft(ctx context.Context, id int64, input content.PostInput, expectedRevision int64, now time.Time) (content.Post, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return content.Post{}, fmt.Errorf("begin save draft: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := getPost(ctx, tx, id)
	if err != nil {
		return content.Post{}, err
	}
	if current.Revision != expectedRevision {
		return content.Post{}, posts.ErrConflict
	}
	if err := insertRevision(ctx, tx, current, now); err != nil {
		return content.Post{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE posts
		SET slug = ?, title = ?, summary = ?, content_md = ?, category_id = ?,
			cover_media_id = ?, revision = revision + 1, updated_at = ?
		WHERE id = ? AND revision = ?`,
		input.Slug, input.Title, input.Summary, input.ContentMD, input.CategoryID,
		input.CoverMediaID, formatTime(now), id, expectedRevision,
	)
	if err != nil {
		return content.Post{}, fmt.Errorf("update draft: %w", err)
	}
	if err := requireUpdated(result); err != nil {
		return content.Post{}, err
	}
	if err := replaceTags(ctx, tx, id, input.TagIDs); err != nil {
		return content.Post{}, err
	}
	if err := trimRevisions(ctx, tx, id); err != nil {
		return content.Post{}, err
	}
	updated, err := getPost(ctx, tx, id)
	if err != nil {
		return content.Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return content.Post{}, fmt.Errorf("commit save draft: %w", err)
	}
	return updated, nil
}

func (r *PostRepository) ListRevisions(ctx context.Context, postID int64) ([]content.PostRevision, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, post_id, revision, title, slug, summary, content_md,
			category_id, tag_ids_json, created_at
		FROM post_revisions WHERE post_id = ? ORDER BY revision DESC`, postID)
	if err != nil {
		return nil, fmt.Errorf("list post revisions: %w", err)
	}
	defer rows.Close()

	revisions := make([]content.PostRevision, 0)
	for rows.Next() {
		var revision content.PostRevision
		var categoryID sql.NullInt64
		var tagIDsJSON, createdAt string
		if err := rows.Scan(
			&revision.ID, &revision.PostID, &revision.Revision, &revision.Title,
			&revision.Slug, &revision.Summary, &revision.ContentMD, &categoryID,
			&tagIDsJSON, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan post revision: %w", err)
		}
		revision.CategoryID = optionalInt64(categoryID)
		if err := json.Unmarshal([]byte(tagIDsJSON), &revision.TagIDs); err != nil {
			return nil, fmt.Errorf("decode revision tags: %w", err)
		}
		revision.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, revision)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate post revisions: %w", err)
	}
	return revisions, nil
}

func (r *PostRepository) RestoreRevision(ctx context.Context, postID, revisionID, expectedRevision int64, now time.Time) (content.Post, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return content.Post{}, fmt.Errorf("begin restore revision: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := getPost(ctx, tx, postID)
	if err != nil {
		return content.Post{}, err
	}
	if current.Revision != expectedRevision {
		return content.Post{}, posts.ErrConflict
	}
	target, err := getRevision(ctx, tx, postID, revisionID)
	if err != nil {
		return content.Post{}, err
	}
	if err := insertRevision(ctx, tx, current, now); err != nil {
		return content.Post{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE posts
		SET slug = ?, title = ?, summary = ?, content_md = ?, category_id = ?,
			revision = revision + 1, updated_at = ?
		WHERE id = ? AND revision = ?`,
		target.Slug, target.Title, target.Summary, target.ContentMD, target.CategoryID,
		formatTime(now), postID, expectedRevision,
	)
	if err != nil {
		return content.Post{}, fmt.Errorf("restore post revision: %w", err)
	}
	if err := requireUpdated(result); err != nil {
		return content.Post{}, err
	}
	if err := replaceTags(ctx, tx, postID, target.TagIDs); err != nil {
		return content.Post{}, err
	}
	if err := trimRevisions(ctx, tx, postID); err != nil {
		return content.Post{}, err
	}
	restored, err := getPost(ctx, tx, postID)
	if err != nil {
		return content.Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return content.Post{}, fmt.Errorf("commit restore revision: %w", err)
	}
	return restored, nil
}

func (r *PostRepository) Publish(ctx context.Context, id int64, rendered content.RenderedContent, expectedRevision int64, now time.Time) (content.Post, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return content.Post{}, fmt.Errorf("begin publish: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	current, err := getPost(ctx, tx, id)
	if err != nil {
		return content.Post{}, err
	}
	if current.Revision != expectedRevision {
		return content.Post{}, posts.ErrConflict
	}
	if err := insertRevision(ctx, tx, current, now); err != nil {
		return content.Post{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE posts SET content_html = ?, content_plain = ?, status = ?,
			published_at = COALESCE(published_at, ?), revision = revision + 1, updated_at = ?
		WHERE id = ? AND revision = ?`, rendered.HTML, rendered.PlainText,
		content.StatusPublished, formatTime(now), formatTime(now), id, expectedRevision)
	if err != nil {
		return content.Post{}, fmt.Errorf("publish post: %w", err)
	}
	if err := requireUpdated(result); err != nil {
		return content.Post{}, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM posts_fts WHERE post_id = ?", id); err != nil {
		return content.Post{}, fmt.Errorf("clear post search index: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO posts_fts (post_id, title, summary, content_plain)
		VALUES (?, ?, ?, ?)`, id, current.Title, current.Summary, rendered.PlainText); err != nil {
		return content.Post{}, fmt.Errorf("index published post: %w", err)
	}
	if err := trimRevisions(ctx, tx, id); err != nil {
		return content.Post{}, err
	}
	published, err := getPost(ctx, tx, id)
	if err != nil {
		return content.Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return content.Post{}, fmt.Errorf("commit publish: %w", err)
	}
	return published, nil
}

func (r *PostRepository) Archive(ctx context.Context, id, expectedRevision int64, now time.Time) (content.Post, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return content.Post{}, fmt.Errorf("begin archive: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	current, err := getPost(ctx, tx, id)
	if err != nil {
		return content.Post{}, err
	}
	if current.Revision != expectedRevision {
		return content.Post{}, posts.ErrConflict
	}
	if err := insertRevision(ctx, tx, current, now); err != nil {
		return content.Post{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE posts SET status = ?, revision = revision + 1, updated_at = ?
		WHERE id = ? AND revision = ?`, content.StatusArchived, formatTime(now), id, expectedRevision)
	if err != nil {
		return content.Post{}, fmt.Errorf("archive post: %w", err)
	}
	if err := requireUpdated(result); err != nil {
		return content.Post{}, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM posts_fts WHERE post_id = ?", id); err != nil {
		return content.Post{}, fmt.Errorf("remove archived post from search: %w", err)
	}
	if err := trimRevisions(ctx, tx, id); err != nil {
		return content.Post{}, err
	}
	archived, err := getPost(ctx, tx, id)
	if err != nil {
		return content.Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return content.Post{}, fmt.Errorf("commit archive: %w", err)
	}
	return archived, nil
}

func (r *PostRepository) GetPublishedBySlug(ctx context.Context, slug string) (content.Post, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, "SELECT id FROM posts WHERE slug = ? AND status = ?", slug, content.StatusPublished).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return content.Post{}, posts.ErrNotFound
	}
	if err != nil {
		return content.Post{}, fmt.Errorf("find published post: %w", err)
	}
	return getPost(ctx, r.db, id)
}

func (r *PostRepository) ListPublished(ctx context.Context, filter posts.PublishedFilter) ([]content.Post, int, error) {
	where := []string{"p.status = ?"}
	args := []any{content.StatusPublished}
	if filter.CategorySlug != "" {
		where = append(where, "EXISTS (SELECT 1 FROM categories c WHERE c.id = p.category_id AND c.slug = ?)")
		args = append(args, filter.CategorySlug)
	}
	if filter.TagSlug != "" {
		where = append(where, "EXISTS (SELECT 1 FROM post_tags pt JOIN tags t ON t.id = pt.tag_id WHERE pt.post_id = p.id AND t.slug = ?)")
		args = append(args, filter.TagSlug)
	}
	if filter.Year != 0 {
		where = append(where, "CAST(strftime('%Y', p.published_at) AS INTEGER) = ?")
		args = append(args, filter.Year)
	}
	return r.listPostIDs(ctx, strings.Join(where, " AND "), args, filter.Page, filter.PageSize, "p.published_at DESC, p.id DESC")
}

func (r *PostRepository) SearchPublished(ctx context.Context, query string, page, pageSize int) ([]content.Post, int, error) {
	match, err := searchExpression(query)
	if err != nil {
		return nil, 0, err
	}
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return nil, 0, fmt.Errorf("%w: invalid pagination", posts.ErrValidation)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM posts_fts f JOIN posts p ON p.id = f.post_id
		WHERE posts_fts MATCH ? AND p.status = ?`, match, content.StatusPublished).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count search results: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id FROM posts_fts f JOIN posts p ON p.id = f.post_id
		WHERE posts_fts MATCH ? AND p.status = ?
		ORDER BY bm25(posts_fts), p.id DESC LIMIT ? OFFSET ?`,
		match, content.StatusPublished, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("search published posts: %w", err)
	}
	ids, err := collectIDs(rows)
	if err != nil {
		return nil, 0, err
	}
	result, err := r.loadPosts(ctx, ids)
	return result, total, err
}

func (r *PostRepository) RebuildSearch(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin rebuild search: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "DELETE FROM posts_fts"); err != nil {
		return fmt.Errorf("clear search index: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO posts_fts (post_id, title, summary, content_plain)
		SELECT id, title, summary, content_plain FROM posts WHERE status = ?`, content.StatusPublished); err != nil {
		return fmt.Errorf("rebuild search index: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rebuild search: %w", err)
	}
	return nil
}

func (r *PostRepository) ListAdmin(ctx context.Context, filter posts.AdminFilter) ([]content.Post, int, error) {
	where := []string{"1 = 1"}
	args := make([]any, 0, 3)
	if filter.Status != "" {
		where = append(where, "p.status = ?")
		args = append(args, filter.Status)
	}
	if query := strings.TrimSpace(filter.Query); query != "" {
		where = append(where, "(p.title LIKE ? ESCAPE '\\' OR p.slug LIKE ? ESCAPE '\\')")
		query = "%" + escapeLike(query) + "%"
		args = append(args, query, query)
	}
	return r.listPostIDs(ctx, strings.Join(where, " AND "), args, filter.Page, filter.PageSize, "p.updated_at DESC, p.id DESC")
}

func (r *PostRepository) listPostIDs(ctx context.Context, where string, args []any, page, pageSize int, order string) ([]content.Post, int, error) {
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return nil, 0, fmt.Errorf("%w: invalid pagination", posts.ErrValidation)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM posts p WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count posts: %w", err)
	}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, "SELECT p.id FROM posts p WHERE "+where+" ORDER BY "+order+" LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list posts: %w", err)
	}
	ids, err := collectIDs(rows)
	if err != nil {
		return nil, 0, err
	}
	result, err := r.loadPosts(ctx, ids)
	return result, total, err
}

func collectIDs(rows *sql.Rows) ([]int64, error) {
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan post id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate post ids: %w", err)
	}
	return ids, nil
}

func (r *PostRepository) loadPosts(ctx context.Context, ids []int64) ([]content.Post, error) {
	result := make([]content.Post, 0, len(ids))
	for _, id := range ids {
		post, err := getPost(ctx, r.db, id)
		if err != nil {
			return nil, err
		}
		result = append(result, post)
	}
	return result, nil
}

func searchExpression(query string) (string, error) {
	if utf8.RuneCountInString(query) > 100 {
		return "", fmt.Errorf("%w: search query exceeds 100 characters", posts.ErrValidation)
	}
	tokens := strings.Fields(query)
	if len(tokens) == 0 {
		return "", fmt.Errorf("%w: search query is empty", posts.ErrValidation)
	}
	for i, token := range tokens {
		tokens[i] = `"` + strings.ReplaceAll(token, `"`, `""`) + `"`
	}
	return strings.Join(tokens, " "), nil
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getPost(ctx context.Context, q queryer, id int64) (content.Post, error) {
	var post content.Post
	var categoryID, coverMediaID sql.NullInt64
	var publishedAt sql.NullString
	var createdAt, updatedAt string
	err := q.QueryRowContext(ctx, `
		SELECT id, slug, title, summary, content_md, content_html, content_plain,
			status, category_id, cover_media_id, revision, published_at, created_at, updated_at
		FROM posts WHERE id = ?`, id).Scan(
		&post.ID, &post.Slug, &post.Title, &post.Summary, &post.ContentMD,
		&post.ContentHTML, &post.ContentPlain, &post.Status, &categoryID,
		&coverMediaID, &post.Revision, &publishedAt, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return content.Post{}, posts.ErrNotFound
	}
	if err != nil {
		return content.Post{}, fmt.Errorf("get post: %w", err)
	}
	post.CategoryID = optionalInt64(categoryID)
	post.CoverMediaID = optionalInt64(coverMediaID)
	if publishedAt.Valid {
		value, err := parseTime(publishedAt.String)
		if err != nil {
			return content.Post{}, err
		}
		post.PublishedAt = &value
	}
	post.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return content.Post{}, err
	}
	post.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return content.Post{}, err
	}
	post.TagIDs, err = getTagIDs(ctx, q, id)
	if err != nil {
		return content.Post{}, err
	}
	return post, nil
}

func getTagIDs(ctx context.Context, q queryer, postID int64) ([]int64, error) {
	rows, err := q.QueryContext(ctx, "SELECT tag_id FROM post_tags WHERE post_id = ? ORDER BY tag_id", postID)
	if err != nil {
		return nil, fmt.Errorf("list post tags: %w", err)
	}
	defer rows.Close()
	tagIDs := make([]int64, 0)
	for rows.Next() {
		var tagID int64
		if err := rows.Scan(&tagID); err != nil {
			return nil, fmt.Errorf("scan post tag: %w", err)
		}
		tagIDs = append(tagIDs, tagID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate post tags: %w", err)
	}
	return tagIDs, nil
}

func insertRevision(ctx context.Context, tx *sql.Tx, post content.Post, now time.Time) error {
	tagIDsJSON, err := json.Marshal(post.TagIDs)
	if err != nil {
		return fmt.Errorf("encode revision tags: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO post_revisions (
			post_id, revision, title, slug, summary, content_md, category_id,
			tag_ids_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		post.ID, post.Revision, post.Title, post.Slug, post.Summary, post.ContentMD,
		post.CategoryID, string(tagIDsJSON), formatTime(now),
	)
	if err != nil {
		return fmt.Errorf("insert post revision: %w", err)
	}
	return nil
}

func getRevision(ctx context.Context, tx *sql.Tx, postID, revisionID int64) (content.PostRevision, error) {
	var revision content.PostRevision
	var categoryID sql.NullInt64
	var tagIDsJSON, createdAt string
	err := tx.QueryRowContext(ctx, `
		SELECT id, post_id, revision, title, slug, summary, content_md,
			category_id, tag_ids_json, created_at
		FROM post_revisions WHERE id = ? AND post_id = ?`, revisionID, postID).Scan(
		&revision.ID, &revision.PostID, &revision.Revision, &revision.Title,
		&revision.Slug, &revision.Summary, &revision.ContentMD, &categoryID,
		&tagIDsJSON, &createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return content.PostRevision{}, posts.ErrNotFound
	}
	if err != nil {
		return content.PostRevision{}, fmt.Errorf("get post revision: %w", err)
	}
	revision.CategoryID = optionalInt64(categoryID)
	if err := json.Unmarshal([]byte(tagIDsJSON), &revision.TagIDs); err != nil {
		return content.PostRevision{}, fmt.Errorf("decode revision tags: %w", err)
	}
	revision.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return content.PostRevision{}, err
	}
	return revision, nil
}

func replaceTags(ctx context.Context, tx *sql.Tx, postID int64, tagIDs []int64) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM post_tags WHERE post_id = ?", postID); err != nil {
		return fmt.Errorf("clear post tags: %w", err)
	}
	for _, tagID := range tagIDs {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO post_tags (post_id, tag_id) VALUES (?, ?)", postID, tagID,
		); err != nil {
			return fmt.Errorf("insert post tag: %w", err)
		}
	}
	return nil
}

func trimRevisions(ctx context.Context, tx *sql.Tx, postID int64) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM post_revisions
		WHERE post_id = ? AND id NOT IN (
			SELECT id FROM post_revisions WHERE post_id = ?
			ORDER BY revision DESC LIMIT 30
		)`, postID, postID)
	if err != nil {
		return fmt.Errorf("trim post revisions: %w", err)
	}
	return nil
}

func requireUpdated(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated rows: %w", err)
	}
	if rows != 1 {
		return posts.ErrConflict
	}
	return nil
}

func optionalInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	return &value.Int64
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse database time %q: %w", value, err)
	}
	return parsed, nil
}
