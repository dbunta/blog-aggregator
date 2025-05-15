-- +goose Up
CREATE TABLE posts (
  id UUID PRIMARY KEY,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  title VARCHAR NOT NULL,
  url VARCHAR UNIQUE NOT NULL,
  description VARCHAR NOT NULL,
  published_at TIMESTAMP NOT NULL,
  feed_id UUID NOT NULL,
  CONSTRAINT fk_feeds FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE
);
-- +goose Down
DROP TABLE posts;
