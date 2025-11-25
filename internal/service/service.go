package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"prreviewer/internal/models"
	"prreviewer/internal/repo"
)

var (
	ErrTeamExists     = errors.New("team already exists")
	ErrTeamNotFound   = errors.New("team not found")
	ErrUserNotFound   = errors.New("user not found")
	ErrAuthorNotFound = errors.New("author not found")
	ErrPRExists       = errors.New("pull request already exists")
	ErrPRNotFound     = errors.New("pull request not found")
	ErrPRMerged       = errors.New("cannot modify merged PR")
	ErrNotAssigned    = errors.New("reviewer is not assigned to this PR")
	ErrNoCandidate    = errors.New("no suitable replacement found")
)

type Service struct {
	repo *repo.Repository
	rng  *rand.Rand
}

func New(r *repo.Repository, rng *rand.Rand) *Service {
	return &Service{repo: r, rng: rng}
}

func (s *Service) CreateTeam(ctx context.Context, team models.Team) error {
	exists, err := s.repo.TeamExists(ctx, team.TeamName)
	if err != nil {
		return fmt.Errorf("проверка существования команды: %w", err)
	}
	if exists {
		return ErrTeamExists
	}
	return s.repo.CreateTeam(ctx, team)
}

func (s *Service) GetTeam(ctx context.Context, teamName string) (*models.Team, error) {
	team, err := s.repo.GetTeam(ctx, teamName)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrTeamNotFound
	}
	return team, err
}

func (s *Service) SetUserActive(ctx context.Context, uid string, active bool) (*models.User, error) {
	err := s.repo.UpdateUserActiveStatus(ctx, uid, active)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.repo.GetUser(ctx, uid)
}

func (s *Service) CreatePullRequest(ctx context.Context, prID, prName, authorID string) (*models.PR, error) {
	exists, err := s.repo.PRExists(ctx, prID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrPRExists
	}

	author, err := s.repo.GetUser(ctx, authorID)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrAuthorNotFound
	}
	if err != nil {
		return nil, err
	}

	candidates, err := s.repo.GetActiveTeamMembers(ctx, author.TeamName, []string{authorID})
	if err != nil {
		return nil, fmt.Errorf("поиск кандидатов: %w", err)
	}

	candidatesCount := 2
	reviewers := s.pickRandomReviewers(candidates, candidatesCount)

	pr := models.PR{
		ID:                prID,
		Name:              prName,
		AuthorID:          authorID,
		Status:            "OPEN",
		AssignedReviewers: reviewers,
	}

	if err := s.repo.CreatePR(ctx, pr); err != nil {
		return nil, err
	}

	return s.repo.GetPR(ctx, prID)
}

func (s *Service) MergePullRequest(ctx context.Context, prID string) (*models.PR, error) {
	currentPR, err := s.repo.GetPR(ctx, prID)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrPRNotFound
	}
	if err != nil {
		return nil, err
	}

	if currentPR.Status == "MERGED" {
		return currentPR, nil
	}

	if err := s.repo.MergePR(ctx, prID); err != nil {
		return nil, err
	}
	return s.repo.GetPR(ctx, prID)
}

func (s *Service) ReassignReviewer(ctx context.Context, prID, oldReviewerID string) (*models.PR, string, error) {
	pr, err := s.repo.GetPR(ctx, prID)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, "", ErrPRNotFound
	}
	if err != nil {
		return nil, "", err
	}

	if pr.Status == "MERGED" {
		return nil, "", ErrPRMerged
	}

	if !contains(pr.AssignedReviewers, oldReviewerID) {
		return nil, "", ErrNotAssigned
	}

	oldReviewer, err := s.repo.GetUser(ctx, oldReviewerID)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, "", ErrUserNotFound
	}

	excludeList := make([]string, 0, len(pr.AssignedReviewers)+1)
	excludeList = append(excludeList, pr.AssignedReviewers...)
	excludeList = append(excludeList, pr.AuthorID)

	candidates, err := s.repo.GetActiveTeamMembers(ctx, oldReviewer.TeamName, excludeList)
	if err != nil {
		return nil, "", err
	}

	if len(candidates) == 0 {
		return nil, "", ErrNoCandidate
	}

	newReviewer := candidates[s.rng.Intn(len(candidates))]

	if err := s.repo.ReplaceReviewer(ctx, prID, oldReviewerID, newReviewer); err != nil {
		return nil, "", err
	}

	updatedPR, err := s.repo.GetPR(ctx, prID)
	return updatedPR, newReviewer, err
}

func (s *Service) GetUserReviews(ctx context.Context, uid string) (string, []models.PRShort, error) {
	prs, err := s.repo.GetUserReviews(ctx, uid)
	if err != nil {
		return uid, nil, err
	}
	if prs == nil {
		prs = []models.PRShort{}
	}
	return uid, prs, nil
}

func (s *Service) GetStats(ctx context.Context) (*models.Stats, error) {
	return s.repo.GetStats(ctx)
}

func (s *Service) DeactivateTeam(ctx context.Context, teamName string) ([]string, []map[string]string, error) {
	exists, err := s.repo.TeamExists(ctx, teamName)
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		return nil, nil, ErrTeamNotFound
	}

	result, err := s.repo.DeactivateTeamAndReassignPRs(ctx, teamName, s.rng)
	if err != nil {
		return nil, nil, err
	}

	return result.DeactivatedUsers, result.Reassignments, nil
}

// Вспомогательные функции.
func (s *Service) pickRandomReviewers(candidates []string, n int) []string {
	if len(candidates) <= n {
		return candidates
	}
	shuffled := make([]string, len(candidates))
	copy(shuffled, candidates)

	s.rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	return shuffled[:n]
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
