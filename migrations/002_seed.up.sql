INSERT INTO teams (team_name) VALUES 
('team1'), ('team2'), ('team3');

INSERT INTO users (user_id, username, team_name, is_active) VALUES
('user1', 'Alice', 'team1', true),
('user2', 'Bob', 'team1', true),
('user3', 'Charlie', 'team1', true),
('user4', 'Diana', 'team1', true),
('user5', 'Eve', 'team2', true),
('user6', 'Frank', 'team2', true),
('user7', 'Grace', 'team2', true),
('user8', 'Henry', 'team3', true),
('user9', 'Ivy', 'team3', true),
('user10', 'Jack', 'team3', false);