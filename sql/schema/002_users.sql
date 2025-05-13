-- +goose Up
CREATE TABLE feeds (
  id UUID PRIMARY KEY,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  name VARCHAR UNIQUE NOT NULL,
  url VARCHAR UNIQUE NOT NULL,
  user_id UUID UNIQUE NOT NULL,
  CONSTRAINT fk_users 
  FOREIGN KEY (user_id) 
  REFERENCES users(id) 
  ON DELETE CASCADE
);

-- +goose Down
DROP TABLE feeds;
