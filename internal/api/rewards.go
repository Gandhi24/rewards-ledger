package api

import (
	"errors"
	"net/http"

	"rewards-ledger/internal/domain"
)

type createRewardRequest struct {
	MemberID    int64            `json:"member_id"`
	PointTypeID domain.PointType `json:"point_type_id"`
	Points      int64            `json:"points"`
	Description string           `json:"description"`
}

type rewardResponse struct {
	RewardID    int64            `json:"reward_id"`
	MemberID    int64            `json:"member_id"`
	PointTypeID domain.PointType `json:"point_type_id"`
	Points      int64            `json:"points"`
	Description string           `json:"description"`
	EventDate   string           `json:"event_date"`
}

func (s *Server) handleListRewards(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "memberID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid member id")
		return
	}

	rewards, err := s.store.ListRewards(id)
	if err != nil {
		writeError(w, http.StatusNotFound, domain.ErrMemberNotFound.Error())
		return
	}

	out := make([]rewardResponse, 0, len(rewards))
	for _, rw := range rewards {
		out = append(out, toRewardResponse(rw))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateReward(w http.ResponseWriter, r *http.Request) {
	var req createRewardRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	rw, err := s.store.CreateReward(req.MemberID, req.PointTypeID, req.Points, req.Description)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrMemberNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, domain.ErrInvalidPointType),
			errors.Is(err, domain.ErrNonPositivePoints):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, domain.ErrInsufficientBalance):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			s.logger.ErrorContext(r.Context(), "create reward failed",
				"request_id", requestIDFromCtx(r.Context()),
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "could not create reward")
		}
		return
	}

	writeJSON(w, http.StatusCreated, toRewardResponse(rw))
}

func toRewardResponse(rw domain.Reward) rewardResponse {
	return rewardResponse{
		RewardID:    rw.ID,
		MemberID:    rw.MemberID,
		PointTypeID: rw.PointTypeID,
		Points:      rw.Points,
		Description: rw.Description,
		EventDate:   rw.EventDate.Format(isoFormat),
	}
}
