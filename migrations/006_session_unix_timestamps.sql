DROP INDEX idx_sessions_expires_at;
ALTER TABLE sessions RENAME TO sessions_rfc3339;

CREATE TABLE sessions (
  token_hash BLOB PRIMARY KEY,
  csrf_hash BLOB NOT NULL,
  admin_id INTEGER NOT NULL,
  expires_at INTEGER NOT NULL CHECK(typeof(expires_at) = 'integer'),
  last_seen_at INTEGER NOT NULL CHECK(typeof(last_seen_at) = 'integer'),
  created_at INTEGER NOT NULL CHECK(typeof(created_at) = 'integer'),
  FOREIGN KEY (admin_id) REFERENCES admins(id) ON DELETE CASCADE
);

WITH source AS (
  SELECT
    token_hash,
    csrf_hash,
    admin_id,
    expires_at,
    last_seen_at,
    created_at,
    CASE WHEN substr(expires_at, 20, 1) = '.' THEN
      substr(substr(expires_at, 21), 1,
        CASE
          WHEN instr(substr(expires_at, 21), 'Z') > 0 THEN instr(substr(expires_at, 21), 'Z') - 1
          WHEN instr(substr(expires_at, 21), '+') > 0 THEN instr(substr(expires_at, 21), '+') - 1
          WHEN instr(substr(expires_at, 21), '-') > 0 THEN instr(substr(expires_at, 21), '-') - 1
          ELSE 0
        END)
      ELSE '' END AS expires_fraction,
    CASE WHEN substr(last_seen_at, 20, 1) = '.' THEN
      substr(substr(last_seen_at, 21), 1,
        CASE
          WHEN instr(substr(last_seen_at, 21), 'Z') > 0 THEN instr(substr(last_seen_at, 21), 'Z') - 1
          WHEN instr(substr(last_seen_at, 21), '+') > 0 THEN instr(substr(last_seen_at, 21), '+') - 1
          WHEN instr(substr(last_seen_at, 21), '-') > 0 THEN instr(substr(last_seen_at, 21), '-') - 1
          ELSE 0
        END)
      ELSE '' END AS last_seen_fraction,
    CASE WHEN substr(created_at, 20, 1) = '.' THEN
      substr(substr(created_at, 21), 1,
        CASE
          WHEN instr(substr(created_at, 21), 'Z') > 0 THEN instr(substr(created_at, 21), 'Z') - 1
          WHEN instr(substr(created_at, 21), '+') > 0 THEN instr(substr(created_at, 21), '+') - 1
          WHEN instr(substr(created_at, 21), '-') > 0 THEN instr(substr(created_at, 21), '-') - 1
          ELSE 0
        END)
      ELSE '' END AS created_fraction
  FROM sessions_rfc3339
)
INSERT INTO sessions (token_hash, csrf_hash, admin_id, expires_at, last_seen_at, created_at)
SELECT
  token_hash,
  csrf_hash,
  admin_id,
  CAST(strftime('%s', expires_at) AS INTEGER) * 1000000000 +
    CASE WHEN expires_fraction = '' THEN 0 ELSE CAST(substr(expires_fraction || '000000000', 1, 9) AS INTEGER) END,
  CAST(strftime('%s', last_seen_at) AS INTEGER) * 1000000000 +
    CASE WHEN last_seen_fraction = '' THEN 0 ELSE CAST(substr(last_seen_fraction || '000000000', 1, 9) AS INTEGER) END,
  CAST(strftime('%s', created_at) AS INTEGER) * 1000000000 +
    CASE WHEN created_fraction = '' THEN 0 ELSE CAST(substr(created_fraction || '000000000', 1, 9) AS INTEGER) END
FROM source;

DROP TABLE sessions_rfc3339;
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
