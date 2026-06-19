package api

import (
	"errors"
	"net/http"
	"strings"

	"rewards-ledger/internal/domain"
)

type createMemberRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type memberResponse struct {
	MemberID  int64  `json:"member_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

type memberWithBalanceResponse struct {
	MemberID      int64  `json:"member_id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	PointsBalance int64  `json:"points_balance"`
	CreatedAt     string `json:"created_at"`
}

func (s *Server) handleCreateMember(w http.ResponseWriter, r *http.Request) {
	var req createMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if strings.TrimSpace(req.Email) == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	m, err := s.store.CreateMember(req.Name, req.Email)
	if err != nil {
		if errors.Is(err, domain.ErrDuplicateEmail) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create member")
		return
	}

	writeJSON(w, http.StatusCreated, memberResponse{
		MemberID:  m.ID,
		Name:      m.Name,
		Email:     m.Email,
		CreatedAt: m.CreatedAt.Format(isoFormat),
	})
}

func (s *Server) handleGetMember(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "memberID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid member id")
		return
	}

	m, err := s.store.GetMember(id)
	if err != nil {
		writeError(w, http.StatusNotFound, domain.ErrMemberNotFound.Error())
		return
	}
	balance, _ := s.store.Balance(id)

	writeJSON(w, http.StatusOK, memberWithBalanceResponse{
		MemberID:      m.ID,
		Name:          m.Name,
		Email:         m.Email,
		PointsBalance: balance,
		CreatedAt:     m.CreatedAt.Format(isoFormat),
	})
}
