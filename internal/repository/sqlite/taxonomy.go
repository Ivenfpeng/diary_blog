package sqlite

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	ErrValidation = errors.New("management validation failed")
	ErrNotFound   = errors.New("management resource not found")
	ErrReferenced = errors.New("management resource is referenced")
)

var taxonomySlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Taxonomy struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type TaxonomyInput struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (r *PostRepository) ListCategories(ctx context.Context) ([]Taxonomy, error) {
	return r.listTaxonomy(ctx, "categories")
}
func (r *PostRepository) ListTags(ctx context.Context) ([]Taxonomy, error) {
	return r.listTaxonomy(ctx, "tags")
}
func (r *PostRepository) CreateCategory(ctx context.Context, input TaxonomyInput, now time.Time) (Taxonomy, error) {
	return r.createTaxonomy(ctx, "categories", "tags", input, now)
}
func (r *PostRepository) CreateTag(ctx context.Context, input TaxonomyInput, now time.Time) (Taxonomy, error) {
	return r.createTaxonomy(ctx, "tags", "categories", input, now)
}
func (r *PostRepository) UpdateCategory(ctx context.Context, id int64, input TaxonomyInput, now time.Time) (Taxonomy, error) {
	return r.updateTaxonomy(ctx, "categories", "tags", id, input, now)
}
func (r *PostRepository) UpdateTag(ctx context.Context, id int64, input TaxonomyInput, now time.Time) (Taxonomy, error) {
	return r.updateTaxonomy(ctx, "tags", "categories", id, input, now)
}

func (r *PostRepository) DeleteCategory(ctx context.Context, id int64) error {
	var count int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM posts WHERE category_id = ?", id).Scan(&count); err != nil {
		return fmt.Errorf("check category references: %w", err)
	}
	if count > 0 {
		return ErrReferenced
	}
	return r.deleteTaxonomy(ctx, "categories", id)
}
func (r *PostRepository) DeleteTag(ctx context.Context, id int64) error {
	return r.deleteTaxonomy(ctx, "tags", id)
}

func (r *PostRepository) listTaxonomy(ctx context.Context, table string) ([]Taxonomy, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, slug, created_at, updated_at FROM "+table+" ORDER BY name, id")
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", table, err)
	}
	defer rows.Close()
	values := []Taxonomy{}
	for rows.Next() {
		var item Taxonomy
		var created, updated string
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &created, &updated); err != nil {
			return nil, err
		}
		item.CreatedAt, err = parseTime(created)
		if err == nil {
			item.UpdatedAt, err = parseTime(updated)
		}
		if err != nil {
			return nil, err
		}
		values = append(values, item)
	}
	return values, rows.Err()
}
func cleanTaxonomy(input TaxonomyInput) (TaxonomyInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = strings.TrimSpace(strings.ToLower(input.Slug))
	if input.Name == "" || !taxonomySlugPattern.MatchString(input.Slug) || len(input.Name) > 120 || len(input.Slug) > 120 {
		return TaxonomyInput{}, ErrValidation
	}
	return input, nil
}
func (r *PostRepository) slugInUse(ctx context.Context, table string, id int64, slug string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE slug = ? AND id != ?", slug, id).Scan(&count)
	return count > 0, err
}
func (r *PostRepository) createTaxonomy(ctx context.Context, table, other string, input TaxonomyInput, now time.Time) (Taxonomy, error) {
	var err error
	if input, err = cleanTaxonomy(input); err != nil {
		return Taxonomy{}, err
	}
	used, err := r.slugInUse(ctx, other, 0, input.Slug)
	if err != nil {
		return Taxonomy{}, err
	}
	if used {
		return Taxonomy{}, ErrValidation
	}
	result, err := r.db.ExecContext(ctx, "INSERT INTO "+table+" (name, slug, created_at, updated_at) VALUES (?, ?, ?, ?)", input.Name, input.Slug, formatTime(now), formatTime(now))
	if err != nil {
		return Taxonomy{}, fmt.Errorf("create %s: %w", table, err)
	}
	id, _ := result.LastInsertId()
	return Taxonomy{ID: id, Name: input.Name, Slug: input.Slug, CreatedAt: now, UpdatedAt: now}, nil
}
func (r *PostRepository) updateTaxonomy(ctx context.Context, table, other string, id int64, input TaxonomyInput, now time.Time) (Taxonomy, error) {
	var err error
	if id < 1 {
		return Taxonomy{}, ErrNotFound
	}
	if input, err = cleanTaxonomy(input); err != nil {
		return Taxonomy{}, err
	}
	used, err := r.slugInUse(ctx, other, id, input.Slug)
	if err != nil {
		return Taxonomy{}, err
	}
	if used {
		return Taxonomy{}, ErrValidation
	}
	result, err := r.db.ExecContext(ctx, "UPDATE "+table+" SET name = ?, slug = ?, updated_at = ? WHERE id = ?", input.Name, input.Slug, formatTime(now), id)
	if err != nil {
		return Taxonomy{}, fmt.Errorf("update %s: %w", table, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return Taxonomy{}, ErrNotFound
	}
	return Taxonomy{ID: id, Name: input.Name, Slug: input.Slug, UpdatedAt: now}, nil
}
func (r *PostRepository) deleteTaxonomy(ctx context.Context, table string, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM "+table+" WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete %s: %w", table, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
