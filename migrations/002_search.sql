CREATE VIRTUAL TABLE posts_fts USING fts5(
  post_id UNINDEXED,
  title,
  summary,
  content_plain,
  tokenize='trigram'
);
