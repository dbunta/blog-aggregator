-- name: CreateFeedFollow :one
WITH inserted_feed_follows AS (
  INSERT INTO feed_follows (id, created_at, updated_at, user_id, feed_id)
  VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
  )
  RETURNING *
) SELECT ff.*, u.name as user_name, f.name as feed_name
FROM inserted_feed_follows ff
JOIN users u on u.id = ff.user_id
JOIN feeds f on f.id = ff.feed_id;

-- name: GetFeedFollows :many
SELECT ff.*, u.name as user_name, f.name as feed_name
FROM feed_follows ff
JOIN users u on u.id = ff.user_id
JOIN feeds f on f.id = ff.feed_id
WHERE ff.user_id = $1;

-- name: DeleteFeedFollows :exec
DELETE FROM feed_follows 
WHERE id IN (
SELECT ff.id
FROM feed_follows ff
JOIN users u on ff.user_id = u.id
JOIN feeds f on f.id = ff.feed_id
WHERE u.name = $1
AND f.url = $2
);

