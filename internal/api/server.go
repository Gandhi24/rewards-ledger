package api

import (
	"net/http"

	"rewards-ledger/internal/domain"
)

// Store is the persistence contract the API layer depends on.
// Any implementation that satisfies these methods can be passed to NewServer.
type Store interface {
	CreateMember(name, email string) (domain.Member, error)
	GetMember(id int64) (domain.Member, error)
	Balance(memberID int64) (int64, error)
	ListRewards(memberID int64) ([]domain.Reward, error)
	CreateReward(memberID int64, pt domain.PointType, magnitude int64, description string) (domain.Reward, error)
}

type Server struct {
	store Store
}

func NewServer(s Store) *Server {
	return &Server{store: s}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /members", s.handleCreateMember)
	mux.HandleFunc("GET /members/{memberID}", s.handleGetMember)
	mux.HandleFunc("GET /members/{memberID}/rewards", s.handleListRewards)
	mux.HandleFunc("POST /rewards", s.handleCreateReward)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}
