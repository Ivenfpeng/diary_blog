CREATE TABLE published_posts (
  post_id INTEGER PRIMARY KEY,
  slug TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  summary TEXT NOT NULL DEFAULT '',
  content_md TEXT NOT NULL,
  content_html TEXT NOT NULL,
  content_plain TEXT NOT NULL,
  published_at TEXT NOT NULL,
  FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE
);

CREATE TABLE published_post_categories (
  post_id INTEGER PRIMARY KEY,
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  FOREIGN KEY (post_id) REFERENCES published_posts(post_id) ON DELETE CASCADE
);

CREATE TABLE published_post_tags (
  post_id INTEGER NOT NULL,
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  PRIMARY KEY (post_id, slug),
  FOREIGN KEY (post_id) REFERENCES published_posts(post_id) ON DELETE CASCADE
);

CREATE INDEX idx_published_posts_published_at ON published_posts(published_at DESC, post_id DESC);
CREATE INDEX idx_published_post_categories_slug ON published_post_categories(slug);
CREATE INDEX idx_published_post_tags_slug ON published_post_tags(slug);

INSERT INTO published_posts (post_id, slug, title, summary, content_md, content_html, content_plain, published_at)
SELECT id, slug, title, summary, content_md, content_html, content_plain, COALESCE(published_at, updated_at)
FROM posts
WHERE status = 'published';

INSERT INTO published_post_categories (post_id, slug, name)
SELECT p.id, c.slug, c.name
FROM posts p
JOIN categories c ON c.id = p.category_id
WHERE p.status = 'published';

INSERT INTO published_post_tags (post_id, slug, name)
SELECT pt.post_id, t.slug, t.name
FROM post_tags pt
JOIN posts p ON p.id = pt.post_id
JOIN tags t ON t.id = pt.tag_id
WHERE p.status = 'published';

DELETE FROM posts_fts;

INSERT INTO posts_fts (post_id, title, summary, content_plain)
SELECT post_id, title, summary, content_plain
FROM published_posts;
