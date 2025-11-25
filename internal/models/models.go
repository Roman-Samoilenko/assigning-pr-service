package models

type Team struct {
	TeamName string       `json:"team_name"`
	Members  []TeamMember `json:"members"`
}

type TeamMember struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

type User struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	TeamName string `json:"team_name"`
	IsActive bool   `json:"is_active"`
}

type PR struct {
	ID                string   `json:"pull_request_id"`
	Name              string   `json:"pull_request_name"`
	AuthorID          string   `json:"author_id"`
	Status            string   `json:"status"`
	AssignedReviewers []string `json:"assigned_reviewers"`
	CreatedAt         *string  `json:"createdAt,omitempty"`
	MergedAt          *string  `json:"mergedAt,omitempty"`
}

type PRShort struct {
	ID       string `json:"pull_request_id"`
	Name     string `json:"pull_request_name"`
	AuthorID string `json:"author_id"`
	Status   string `json:"status"`
}

type Stats struct {
	TotalTeams        int               `json:"total_teams"`
	TotalUsers        int               `json:"total_users"`
	TotalPRs          int               `json:"total_prs"`
	OpenPRs           int               `json:"open_prs"`
	MergedPRs         int               `json:"merged_prs"`
	AssignmentsByUser []UserAssignments `json:"assignments_by_user"`
	ReviewersByPR     []PRReviewerCount `json:"reviewers_by_pr"`
}

type UserAssignments struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	Assignments int    `json:"total_assignments"`
}

type PRReviewerCount struct {
	PRID          string `json:"pull_request_id"`
	PRName        string `json:"pull_request_name"`
	ReviewerCount int    `json:"reviewer_count"`
}
