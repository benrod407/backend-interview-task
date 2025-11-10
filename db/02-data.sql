-- Ingest the db tables with initial data

-- Insert users
INSERT INTO user (id, name)
VALUES
('1', 'Lily'),
('2', 'Matt'),
('3', 'Kevin'),     -- likes everyone
('4', 'Alice'),
('5', "Anna"),      -- liked by everyone
('6', "Sebastian");

-- Insert decisions
INSERT INTO decision (actor_user_id, recipient_user_id, liked_recipient)
VALUES
('1', '2', TRUE),
('2', '1', TRUE),
('2', '4', FALSE),
('3', '1', TRUE),
('3', '2', TRUE),
('3', '4', TRUE),
('4', '1', TRUE),
('1', '5', TRUE),
('2', '5', TRUE),
('3', '5', TRUE),
('4', '5', TRUE),
('6', '5', TRUE);

-- Insert like counts
INSERT INTO like_stats (user_id, like_count)
VALUES
('1', 3),  -- liked by users 2, 3, 4
('2', 2),  -- liked by users 1, 3
('3', 0),  -- not liked by anyone
('4', 1),  -- liked by user 3
('5', 5),  -- liked by everyone
('6', 0);  -- not liked by anyone