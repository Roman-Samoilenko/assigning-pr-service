CREATE TABLE teams (
    team_name VARCHAR(255) PRIMARY KEY
);

CREATE TABLE users (
    user_id VARCHAR(255) PRIMARY KEY,
    username VARCHAR(255) NOT NULL,
    team_name VARCHAR(255) NOT NULL REFERENCES teams(team_name),
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX idx_users_team ON users(team_name);

CREATE TABLE pull_requests (
    pull_request_id VARCHAR(255) PRIMARY KEY,
    pull_request_name VARCHAR(255) NOT NULL,
    author_id VARCHAR(255) NOT NULL REFERENCES users(user_id),
    status VARCHAR(20) NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    merged_at TIMESTAMPTZ
);

CREATE TABLE pr_reviewers (
    pull_request_id VARCHAR(255) REFERENCES pull_requests(pull_request_id),
    user_id VARCHAR(255) REFERENCES users(user_id),
    PRIMARY KEY (pull_request_id, user_id)
);