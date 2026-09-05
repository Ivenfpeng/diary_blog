CREATE TRIGGER categories_slug_must_not_match_tag_insert
BEFORE INSERT ON categories
WHEN EXISTS (SELECT 1 FROM tags WHERE slug = NEW.slug)
BEGIN
  SELECT RAISE(ABORT, 'taxonomy slug already exists');
END;

CREATE TRIGGER categories_slug_must_not_match_tag_update
BEFORE UPDATE OF slug ON categories
WHEN EXISTS (SELECT 1 FROM tags WHERE slug = NEW.slug)
BEGIN
  SELECT RAISE(ABORT, 'taxonomy slug already exists');
END;

CREATE TRIGGER tags_slug_must_not_match_category_insert
BEFORE INSERT ON tags
WHEN EXISTS (SELECT 1 FROM categories WHERE slug = NEW.slug)
BEGIN
  SELECT RAISE(ABORT, 'taxonomy slug already exists');
END;

CREATE TRIGGER tags_slug_must_not_match_category_update
BEFORE UPDATE OF slug ON tags
WHEN EXISTS (SELECT 1 FROM categories WHERE slug = NEW.slug)
BEGIN
  SELECT RAISE(ABORT, 'taxonomy slug already exists');
END;
