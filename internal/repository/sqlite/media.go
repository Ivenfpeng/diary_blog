package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Media struct {
	ID        int64     `json:"id"`
	Path      string    `json:"path"`
	MIMEType  string    `json:"mime_type"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Size      int64     `json:"size"`
	AltText   string    `json:"alt_text"`
	CreatedAt time.Time `json:"created_at"`
}
type MediaInput struct {
	Path     string
	MIMEType string
	Width    int
	Height   int
	Size     int64
	AltText  string
}

func (r *PostRepository) CreateMedia(ctx context.Context, input MediaInput, now time.Time) (Media, error) {
	input.Path = strings.Trim(strings.TrimSpace(input.Path), "/")
	input.AltText = strings.TrimSpace(input.AltText)
	if input.Path == "" || input.Width < 1 || input.Height < 1 || input.Size < 1 || input.Size > 10*1024*1024 || !allowedMediaType(input.MIMEType) {
		return Media{}, ErrValidation
	}
	result, err := r.db.ExecContext(ctx, "INSERT INTO media (path, mime_type, width, height, size, alt_text, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)", input.Path, input.MIMEType, input.Width, input.Height, input.Size, input.AltText, formatTime(now))
	if err != nil {
		return Media{}, fmt.Errorf("create media: %w", err)
	}
	id, _ := result.LastInsertId()
	return Media{ID: id, Path: input.Path, MIMEType: input.MIMEType, Width: input.Width, Height: input.Height, Size: input.Size, AltText: input.AltText, CreatedAt: now}, nil
}
func (r *PostRepository) ListMedia(ctx context.Context) ([]Media, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, path, mime_type, width, height, size, alt_text, created_at FROM media ORDER BY id DESC")
	if err != nil {
		return nil, fmt.Errorf("list media: %w", err)
	}
	defer rows.Close()
	result := []Media{}
	for rows.Next() {
		var value Media
		var created string
		if err := rows.Scan(&value.ID, &value.Path, &value.MIMEType, &value.Width, &value.Height, &value.Size, &value.AltText, &created); err != nil {
			return nil, err
		}
		value.CreatedAt, err = parseTime(created)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
func (r *PostRepository) UpdateMediaAltText(ctx context.Context, id int64, altText string) (Media, error) {
	altText = strings.TrimSpace(altText)
	result, err := r.db.ExecContext(ctx, "UPDATE media SET alt_text = ? WHERE id = ?", altText, id)
	if err != nil {
		return Media{}, err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return Media{}, ErrNotFound
	}
	rows, err := r.ListMedia(ctx)
	if err != nil {
		return Media{}, err
	}
	for _, item := range rows {
		if item.ID == id {
			return item, nil
		}
	}
	return Media{}, ErrNotFound
}
func allowedMediaType(mime string) bool {
	switch mime {
	case "image/jpeg", "image/png", "image/webp", "image/gif", "image/avif":
		return true
	}
	return false
}
