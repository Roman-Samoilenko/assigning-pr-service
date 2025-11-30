package repo

import (
	"context"
	"errors"
	"time"

	"prreviewer/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) TeamExists(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM teams WHERE team_name=$1)", name).Scan(&exists)
	return exists, err
}

func (r *Repository) CreateTeam(ctx context.Context, team models.Team) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, "INSERT INTO teams(team_name) VALUES($1)", team.TeamName)
	if err != nil {
		return err
	}

	for _, m := range team.Members {
		_, err = tx.Exec(ctx, `
			INSERT INTO users(user_id, username, team_name, is_active) 
			VALUES($1, $2, $3, $4)
			ON CONFLICT(user_id) DO UPDATE 
			SET username=$2, team_name=$3, is_active=$4`,
			m.UserID, m.Username, team.TeamName, m.IsActive)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) GetTeam(ctx context.Context, name string) (*models.Team, error) {
	exists, err := r.TeamExists(ctx, name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}

	rows, err := r.db.Query(ctx,
		"SELECT user_id, username, is_active FROM users WHERE team_name=$1 ORDER BY user_id",
		name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []models.TeamMember{}
	for rows.Next() {
		var m models.TeamMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.IsActive); err != nil {
			return nil, err
		}
		members = append(members, m)
	}

	return &models.Team{TeamName: name, Members: members}, nil
}

func (r *Repository) GetUser(ctx context.Context, uid string) (*models.User, error) {
	var u models.User
	err := r.db.QueryRow(ctx,
		"SELECT user_id, username, team_name, is_active FROM users WHERE user_id=$1",
		uid).Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

func (r *Repository) UpdateUserActiveStatus(ctx context.Context, uid string, active bool) error {
	tag, err := r.db.Exec(ctx, "UPDATE users SET is_active=$1 WHERE user_id=$2", active, uid)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) GetActiveTeamMembers(ctx context.Context, teamName string, excludeIDs []string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		"SELECT user_id FROM users WHERE team_name=$1 AND is_active=true ORDER BY user_id",
		teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	excludeMap := make(map[string]bool)
	for _, id := range excludeIDs {
		excludeMap[id] = true
	}

	result := []string{}
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		if !excludeMap[uid] {
			result = append(result, uid)
		}
	}

	return result, nil
}

func (r *Repository) PRExists(ctx context.Context, prID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id=$1)",
		prID).Scan(&exists)
	return exists, err
}

func (r *Repository) CreatePR(ctx context.Context, pr models.PR) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx,
		"INSERT INTO pull_requests(pull_request_id, pull_request_name, author_id, status) VALUES($1, $2, $3, 'OPEN')",
		pr.ID, pr.Name, pr.AuthorID)
	if err != nil {
		return err
	}

	for _, reviewerID := range pr.AssignedReviewers {
		_, err = tx.Exec(ctx,
			"INSERT INTO pr_reviewers(pull_request_id, user_id) VALUES($1, $2)",
			pr.ID, reviewerID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) GetPR(ctx context.Context, prID string) (*models.PR, error) {
	var pr models.PR
	var createdAt, mergedAt *time.Time

	err := r.db.QueryRow(ctx, `
		SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at 
		FROM pull_requests WHERE pull_request_id=$1`,
		prID).Scan(&pr.ID, &pr.Name, &pr.AuthorID, &pr.Status, &createdAt, &mergedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if createdAt != nil {
		s := createdAt.Format(time.RFC3339)
		pr.CreatedAt = &s
	}
	if mergedAt != nil {
		s := mergedAt.Format(time.RFC3339)
		pr.MergedAt = &s
	}

	rows, err := r.db.Query(ctx,
		"SELECT user_id FROM pr_reviewers WHERE pull_request_id=$1 ORDER BY user_id",
		prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pr.AssignedReviewers = []string{}
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		pr.AssignedReviewers = append(pr.AssignedReviewers, uid)
	}

	return &pr, nil
}

func (r *Repository) MergePR(ctx context.Context, prID string) error {
	tag, err := r.db.Exec(ctx,
		"UPDATE pull_requests SET status='MERGED', merged_at=NOW() WHERE pull_request_id=$1 AND status='OPEN'",
		prID)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		exists, _ := r.PRExists(ctx, prID)
		if !exists {
			return ErrNotFound
		}
	}

	return nil
}

func (r *Repository) ReplaceReviewer(ctx context.Context, prID, oldReviewerID, newReviewerID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx,
		"DELETE FROM pr_reviewers WHERE pull_request_id=$1 AND user_id=$2",
		prID, oldReviewerID)
	if err != nil {
		return err
	}

	if newReviewerID != "" {
		_, err = tx.Exec(ctx,
			"INSERT INTO pr_reviewers(pull_request_id, user_id) VALUES($1, $2)",
			prID, newReviewerID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) GetUserReviews(ctx context.Context, uid string) ([]models.PRShort, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.pull_request_id, p.pull_request_name, p.author_id, p.status 
		FROM pull_requests p 
		JOIN pr_reviewers r ON p.pull_request_id = r.pull_request_id 
		WHERE r.user_id = $1
		ORDER BY p.created_at DESC`,
		uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prs := []models.PRShort{}
	for rows.Next() {
		var pr models.PRShort
		if err := rows.Scan(&pr.ID, &pr.Name, &pr.AuthorID, &pr.Status); err != nil {
			return nil, err
		}
		prs = append(prs, pr)
	}

	return prs, nil
}

func (r *Repository) DeactivateTeamMembers(ctx context.Context, teamName string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		"UPDATE users SET is_active=false WHERE team_name=$1 AND is_active=true RETURNING user_id",
		teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deactivated := []string{}
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		deactivated = append(deactivated, uid)
	}

	return deactivated, nil
}

func (r *Repository) GetOpenPRsByReviewers(ctx context.Context, reviewerIDs []string) ([]string, error) {
	if len(reviewerIDs) == 0 {
		return []string{}, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT r.pull_request_id 
		FROM pr_reviewers r
		JOIN pull_requests p ON r.pull_request_id = p.pull_request_id
		WHERE p.status = 'OPEN' AND r.user_id = ANY($1)`,
		reviewerIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prIDs := []string{}
	for rows.Next() {
		var prID string
		if err := rows.Scan(&prID); err != nil {
			return nil, err
		}
		prIDs = append(prIDs, prID)
	}

	return prIDs, nil
}

type DeactivationResult struct {
	DeactivatedUsers []string
	Reassignments    []map[string]string
}

func (r *Repository) DeactivateTeamAndReassignPRs(
	ctx context.Context,
	teamName string,
	rng interface{ Intn(int) int },
) (*DeactivationResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	deactivated, err := r.deactivateTeamUsers(ctx, tx, teamName)
	if err != nil {
		return nil, err
	}

	if len(deactivated) == 0 {
		_ = tx.Commit(ctx)
		return &DeactivationResult{DeactivatedUsers: []string{}, Reassignments: []map[string]string{}}, nil
	}

	affectedPRs, err := r.getAffectedPRs(ctx, tx, deactivated)
	if err != nil {
		return nil, err
	}

	activeCandidates, err := r.getActiveUsersByTeam(ctx, tx)
	if err != nil {
		return nil, err
	}

	userTeams, err := r.getUserTeams(ctx, tx, deactivated)
	if err != nil {
		return nil, err
	}

	reassignments, err := r.reassignReviewers(ctx, tx, affectedPRs, userTeams, activeCandidates, rng)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &DeactivationResult{
		DeactivatedUsers: deactivated,
		Reassignments:    reassignments,
	}, nil
}

func (r *Repository) GetStats(ctx context.Context) (*models.Stats, error) {
	stats := &models.Stats{}

	queries := []struct {
		sql    string
		target *int
	}{
		{"SELECT COUNT(*) FROM teams", &stats.TotalTeams},
		{"SELECT COUNT(*) FROM users", &stats.TotalUsers},
		{"SELECT COUNT(*) FROM pull_requests", &stats.TotalPRs},
		{"SELECT COUNT(*) FROM pull_requests WHERE status='OPEN'", &stats.OpenPRs},
		{"SELECT COUNT(*) FROM pull_requests WHERE status='MERGED'", &stats.MergedPRs},
	}

	for _, q := range queries {
		if err := r.db.QueryRow(ctx, q.sql).Scan(q.target); err != nil {
			return nil, err
		}
	}

	rows, err := r.db.Query(ctx, `
		SELECT u.user_id, u.username, COUNT(r.pull_request_id) 
		FROM users u 
		LEFT JOIN pr_reviewers r ON u.user_id = r.user_id
		GROUP BY u.user_id 
		ORDER BY COUNT(r.pull_request_id) DESC, u.user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats.AssignmentsByUser = []models.UserAssignments{}
	for rows.Next() {
		var ua models.UserAssignments
		if err := rows.Scan(&ua.UserID, &ua.Username, &ua.Assignments); err != nil {
			return nil, err
		}
		stats.AssignmentsByUser = append(stats.AssignmentsByUser, ua)
	}

	rows2, err := r.db.Query(ctx, `
		SELECT p.pull_request_id, p.pull_request_name, COUNT(r.user_id) 
		FROM pull_requests p 
		LEFT JOIN pr_reviewers r ON p.pull_request_id = r.pull_request_id
		GROUP BY p.pull_request_id 
		ORDER BY COUNT(r.user_id) DESC, p.pull_request_id`)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()

	stats.ReviewersByPR = []models.PRReviewerCount{}
	for rows2.Next() {
		var prc models.PRReviewerCount
		if err := rows2.Scan(&prc.PRID, &prc.PRName, &prc.ReviewerCount); err != nil {
			return nil, err
		}
		stats.ReviewersByPR = append(stats.ReviewersByPR, prc)
	}

	return stats, nil
}

// Вспомогательные функции.
func (r *Repository) deactivateTeamUsers(ctx context.Context, tx pgx.Tx, teamName string) ([]string, error) {
	rows, err := tx.Query(ctx,
		"UPDATE users SET is_active=false WHERE team_name=$1 AND is_active=true RETURNING user_id",
		teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deactivated := []string{}
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		deactivated = append(deactivated, uid)
	}
	return deactivated, nil
}

func (r *Repository) getAffectedPRs(ctx context.Context, tx pgx.Tx, deactivated []string) (map[string]*prData, error) {
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT p.pull_request_id, p.author_id, r.user_id as reviewer
		FROM pull_requests p
		JOIN pr_reviewers r ON p.pull_request_id = r.pull_request_id
		WHERE p.status = 'OPEN' AND r.user_id = ANY($1)
		ORDER BY p.pull_request_id`,
		deactivated)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	affectedPRs := make(map[string]*prData)
	for rows.Next() {
		var prID, authorID, reviewer string
		if err := rows.Scan(&prID, &authorID, &reviewer); err != nil {
			return nil, err
		}

		if affectedPRs[prID] == nil {
			affectedPRs[prID] = &prData{prID: prID, authorID: authorID}
		}
		affectedPRs[prID].reviewers = append(affectedPRs[prID].reviewers, reviewer)
	}
	return affectedPRs, nil
}

func (r *Repository) getActiveUsersByTeam(ctx context.Context, tx pgx.Tx) (map[string][]string, error) {
	rows, err := tx.Query(ctx,
		"SELECT user_id, team_name FROM users WHERE is_active=true ORDER BY user_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activeCandidates := make(map[string][]string)
	for rows.Next() {
		var uid, team string
		if err := rows.Scan(&uid, &team); err != nil {
			return nil, err
		}
		activeCandidates[team] = append(activeCandidates[team], uid)
	}
	return activeCandidates, nil
}

func (r *Repository) getUserTeams(ctx context.Context, tx pgx.Tx, deactivated []string) (map[string]string, error) {
	userTeams := make(map[string]string)
	for _, uid := range deactivated {
		var team string
		err := tx.QueryRow(ctx, "SELECT team_name FROM users WHERE user_id=$1", uid).Scan(&team)
		if err != nil {
			return nil, err
		}
		userTeams[uid] = team
	}
	return userTeams, nil
}

func (r *Repository) reassignReviewers(
	ctx context.Context,
	tx pgx.Tx,
	affectedPRs map[string]*prData,
	userTeams map[string]string,
	activeCandidates map[string][]string,
	rng interface{ Intn(int) int },
) ([]map[string]string, error) {
	reassignments := []map[string]string{}

	for _, pr := range affectedPRs {
		for _, oldReviewer := range pr.reviewers {
			team := userTeams[oldReviewer]
			candidates := activeCandidates[team]

			exclude := make(map[string]bool)
			exclude[pr.authorID] = true
			for _, rev := range pr.reviewers {
				exclude[rev] = true
			}

			filtered := []string{}
			for _, c := range candidates {
				if !exclude[c] {
					filtered = append(filtered, c)
				}
			}

			var newReviewer string
			if len(filtered) > 0 {
				newReviewer = filtered[rng.Intn(len(filtered))]
			}

			_, err := tx.Exec(ctx,
				"DELETE FROM pr_reviewers WHERE pull_request_id=$1 AND user_id=$2",
				pr.prID, oldReviewer)
			if err != nil {
				return nil, err
			}

			if newReviewer != "" {
				_, err = tx.Exec(ctx,
					"INSERT INTO pr_reviewers(pull_request_id, user_id) VALUES($1, $2)",
					pr.prID, newReviewer)
				if err != nil {
					return nil, err
				}
			}

			reassignments = append(reassignments, map[string]string{
				"pr_id": pr.prID,
				"old":   oldReviewer,
				"new":   newReviewer,
			})
		}
	}
	return reassignments, nil
}

type prData struct {
	prID      string
	authorID  string
	reviewers []string
}
