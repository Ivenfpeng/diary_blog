package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
