-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6
)
RETURNING *;

-- name: GetFeeds :many
SELECT f.name, f.url, u.name as user_name FROM feeds f
JOIN users u on u.id = f.user_id;

-- name: GetFeed :one
SELECT * FROM feeds WHERE url = $1;

-- name: MarkFeedFetched :exec
UPDATE feeds SET
last_fetched_at = $1,
updated_at = $1
WHERE id = $2;

-- name: GetNextFeedToFetch :one
SELECT * 
FROM feeds  
ORDER BY last_fetched_at NULLS FIRST
FETCH FIRST 1 ROWS ONLY;