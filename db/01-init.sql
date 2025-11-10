-- Initialize the db schema, creating our tables if they do not exist

-- Create user table
CREATE TABLE IF NOT EXISTS user (
  id CHAR(36) PRIMARY KEY, -- add DEFAULT (UUID()) to autogenerate in prod
  name VARCHAR(100) NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create decision table
CREATE TABLE IF NOT EXISTS decision (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  actor_user_id CHAR(36) NOT NULL,
  recipient_user_id CHAR(36) NOT NULL,
  liked_recipient BOOLEAN NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  
  -- force unique pair of (actor, recipient) decisions
  UNIQUE KEY unique_actor_recipient (actor_user_id, recipient_user_id),

  -- foreign key references
  FOREIGN KEY (actor_user_id) REFERENCES user(id),
  FOREIGN KEY (recipient_user_id) REFERENCES user(id)
);

-- Create like_stats table
CREATE TABLE IF NOT EXISTS like_stats (
  user_id CHAR(36) PRIMARY KEY,
  like_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  -- foreign key references
  FOREIGN KEY (user_id) REFERENCES user(id)
);

-- index for ListLikedYou query optimization
CREATE INDEX idx_decision_recipient_like_id 
  ON decision (recipient_user_id, liked_recipient, id);

-- index for ListNewLikedYou sub-query optimization
CREATE INDEX idx_decision_actor_recipient_like 
  ON decision (actor_user_id, recipient_user_id, liked_recipient);
