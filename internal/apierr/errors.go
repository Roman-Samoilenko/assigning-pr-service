package apierr

import (
	"encoding/json"
	"net/http"
)

type ErrResp struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

var (
	ErrTeamExists     = &AppError{400, "TEAM_EXISTS", "team_name already exists"}
	ErrPRExists       = &AppError{409, "PR_EXISTS", "PR id already exists"}
	ErrPRMerged       = &AppError{409, "PR_MERGED", "cannot reassign on merged PR"}
	ErrNotAssigned    = &AppError{409, "NOT_ASSIGNED", "reviewer is not assigned to this PR"}
	ErrNoCandidate    = &AppError{409, "NO_CANDIDATE", "no active replacement candidate in team"}
	ErrTeamNotFound   = &AppError{404, "NOT_FOUND", "team not found"}
	ErrUserNotFound   = &AppError{404, "NOT_FOUND", "user not found"}
	ErrPRNotFound     = &AppError{404, "NOT_FOUND", "PR not found"}
	ErrAuthorNotFound = &AppError{404, "NOT_FOUND", "author not found"}
)

type AppError struct {
	Status  int
	Code    string
	Message string
}

func (e *AppError) Error() string { return e.Message }

func JSON(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	e := ErrResp{}
	e.Error.Code = code
	e.Error.Message = msg
	if err := json.NewEncoder(w).Encode(e); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func Write(w http.ResponseWriter, e *AppError) {
	JSON(w, e.Status, e.Code, e.Message)
}
