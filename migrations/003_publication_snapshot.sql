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
