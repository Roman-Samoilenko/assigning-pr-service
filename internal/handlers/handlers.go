package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"log"
	"prreviewer/internal/apierr"
	"prreviewer/internal/models"
	"prreviewer/internal/service"
)

type Handler struct {
	svc *service.Service
}

func New(s *service.Service) *Handler {
	return &Handler{svc: s}
}

func respond(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("respond: failed to encode response: %v", err)
			http.Error(w, "internal json error", http.StatusInternalServerError)
		}
	}
}

func (h *Handler) TeamAdd(w http.ResponseWriter, r *http.Request) {
	var team models.Team
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		log.Printf("TeamAdd: failed to decode request body: %v", err)
		apierr.JSON(w, http.StatusBadRequest, "BAD_REQUEST", "некорректный JSON")
		return
	}

	if err := h.svc.CreateTeam(r.Context(), team); err != nil {
		if errors.Is(err, service.ErrTeamExists) {
			log.Printf("TeamAdd: team already exists: %s", team.TeamName)
			apierr.Write(w, apierr.ErrTeamExists)
			return
		}
		log.Printf("TeamAdd: failed to create team %s: %v", team.TeamName, err)
		apierr.JSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "ошибка при создании команды")
		return
	}

	log.Printf("TeamAdd: team created successfully: %s", team.TeamName)
	respond(w, http.StatusCreated, map[string]models.Team{"team": team})
}

func (h *Handler) TeamGet(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		log.Println("TeamGet: team_name parameter missing")
		apierr.JSON(w, http.StatusBadRequest, "BAD_REQUEST", "параметр team_name обязателен")
		return
	}

	team, err := h.svc.GetTeam(r.Context(), teamName)
	if err != nil {
		if errors.Is(err, service.ErrTeamNotFound) {
			log.Printf("TeamGet: team not found: %s", teamName)
			apierr.Write(w, apierr.ErrTeamNotFound)
			return
		}
		log.Printf("TeamGet: failed to get team %s: %v", teamName, err)
		apierr.JSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "не удалось получить команду")
		return
	}

	respond(w, http.StatusOK, team)
}

func (h *Handler) UsersSetIsActive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("UsersSetIsActive: failed to decode request body: %v", err)
		apierr.JSON(w, http.StatusBadRequest, "BAD_REQUEST", "некорректный JSON")
		return
	}

	user, err := h.svc.SetUserActive(r.Context(), req.UserID, req.IsActive)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			log.Printf("UsersSetIsActive: user not found: %s", req.UserID)
			apierr.Write(w, apierr.ErrUserNotFound)
			return
		}
		log.Printf("UsersSetIsActive: failed to update user %s: %v", req.UserID, err)
		apierr.JSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "ошибка обновления статуса")
		return
	}

	log.Printf("UsersSetIsActive: user %s status updated to active=%v", req.UserID, req.IsActive)
	respond(w, http.StatusOK, map[string]*models.User{"user": user})
}

func (h *Handler) PRCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string `json:"pull_request_id"`
		Name     string `json:"pull_request_name"`
		AuthorID string `json:"author_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("PRCreate: failed to decode request body: %v", err)
		apierr.JSON(w, http.StatusBadRequest, "BAD_REQUEST", "некорректный JSON")
		return
	}

	pr, err := h.svc.CreatePullRequest(r.Context(), req.ID, req.Name, req.AuthorID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAuthorNotFound):
			log.Printf("PRCreate: author not found: %s", req.AuthorID)
			apierr.Write(w, apierr.ErrAuthorNotFound)
		case errors.Is(err, service.ErrPRExists):
			log.Printf("PRCreate: PR already exists: %s", req.ID)
			apierr.Write(w, apierr.ErrPRExists)
		default:
			log.Printf("PRCreate: failed to create PR %s: %v", req.ID, err)
			apierr.JSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	log.Printf("PRCreate: PR created successfully: %s", req.ID)
	respond(w, http.StatusCreated, map[string]*models.PR{"pr": pr})
}

func (h *Handler) PRMerge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"pull_request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("PRMerge: failed to decode request body: %v", err)
		apierr.JSON(w, http.StatusBadRequest, "BAD_REQUEST", "некорректный JSON")
		return
	}

	pr, err := h.svc.MergePullRequest(r.Context(), req.ID)
	if err != nil {
		if errors.Is(err, service.ErrPRNotFound) {
			log.Printf("PRMerge: PR not found: %s", req.ID)
			apierr.Write(w, apierr.ErrPRNotFound)
			return
		}
		log.Printf("PRMerge: failed to merge PR %s: %v", req.ID, err)
		apierr.JSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	log.Printf("PRMerge: PR merged successfully: %s", req.ID)
	respond(w, http.StatusOK, map[string]*models.PR{"pr": pr})
}

func (h *Handler) PRReassign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID        string `json:"pull_request_id"`
		OldUserID string `json:"old_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("PRReassign: failed to decode request body: %v", err)
		apierr.JSON(w, http.StatusBadRequest, "BAD_REQUEST", "некорректный JSON")
		return
	}

	pr, newReviewerID, err := h.svc.ReassignReviewer(r.Context(), req.ID, req.OldUserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPRNotFound):
			log.Printf("PRReassign: PR not found: %s", req.ID)
			apierr.Write(w, apierr.ErrPRNotFound)
		case errors.Is(err, service.ErrUserNotFound):
			log.Printf("PRReassign: user not found: %s", req.OldUserID)
			apierr.Write(w, apierr.ErrUserNotFound)
		case errors.Is(err, service.ErrPRMerged):
			log.Printf("PRReassign: PR already merged: %s", req.ID)
			apierr.Write(w, apierr.ErrPRMerged)
		case errors.Is(err, service.ErrNotAssigned):
			log.Printf("PRReassign: user %s not assigned to PR %s", req.OldUserID, req.ID)
			apierr.Write(w, apierr.ErrNotAssigned)
		case errors.Is(err, service.ErrNoCandidate):
			log.Printf("PRReassign: no replacement candidate for PR %s", req.ID)
			apierr.Write(w, apierr.ErrNoCandidate)
		default:
			log.Printf("PRReassign: failed to reassign PR %s: %v", req.ID, err)
			apierr.JSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	log.Printf("PRReassign: reviewer reassigned for PR %s: %s -> %s", req.ID, req.OldUserID, newReviewerID)
	respond(w, http.StatusOK, map[string]interface{}{
		"pr":          pr,
		"replaced_by": newReviewerID,
	})
}

func (h *Handler) UsersGetReview(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("user_id")
	if uid == "" {
		log.Println("UsersGetReview: user_id parameter missing")
		apierr.JSON(w, http.StatusBadRequest, "BAD_REQUEST", "user_id обязателен")
		return
	}

	_, prs, err := h.svc.GetUserReviews(r.Context(), uid)
	if err != nil {
		log.Printf("UsersGetReview: failed to get reviews for user %s: %v", uid, err)
		apierr.JSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	respond(w, http.StatusOK, map[string]interface{}{
		"user_id":       uid,
		"pull_requests": prs,
	})
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStats(r.Context())
	if err != nil {
		log.Printf("Stats: failed to get stats: %v", err)
		apierr.JSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	respond(w, http.StatusOK, stats)
}

func (h *Handler) TeamDeactivate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TeamName string `json:"team_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("TeamDeactivate: failed to decode request body: %v", err)
		apierr.JSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	deactivated, reassignments, err := h.svc.DeactivateTeam(r.Context(), req.TeamName)
	if err != nil {
		if errors.Is(err, service.ErrTeamNotFound) {
			log.Printf("TeamDeactivate: team not found: %s", req.TeamName)
			apierr.Write(w, apierr.ErrTeamNotFound)
			return
		}
		log.Printf("TeamDeactivate: failed to deactivate team %s: %v", req.TeamName, err)
		apierr.JSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	log.Printf(
		"TeamDeactivate: team %s deactivated, users: %d, reassignments: %d",
		req.TeamName,
		len(deactivated),
		len(reassignments),
	)
	respond(w, http.StatusOK, map[string]interface{}{
		"deactivated_users": deactivated,
		"reassignments":     reassignments,
	})
}
