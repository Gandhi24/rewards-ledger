// Package store provides an in-memory, concurrency-safe implementation of the
// persistence needed by the rewards ledger. It is intentionally simple so the
// focus stays on the domain rules; swapping in a SQL-backed store later only
// requires satisfying the same method set used by the API layer.
package store

import (
	"sync"
	"time"

	"rewards-ledger/internal/domain"
)

// Clock lets tests inject deterministic timestamps. Defaults to time.Now.
type Clock func() time.Time

// MemoryStore holds members and rewards in memory behind a mutex.
type MemoryStore struct {
	mu sync.RWMutex

	members      map[int64]domain.Member
	emailIndex   map[string]int64 // lowercased email -> member id
	rewards      map[int64]domain.Reward
	rewardsByMem map[int64][]int64 // member id -> reward ids (insertion order)

	nextMemberID int64
	nextRewardID int64

	now Clock
}

// New returns an empty MemoryStore using time.Now as its clock.
func New() *MemoryStore {
	return NewWithClock(time.Now)
}

// NewWithClock returns an empty MemoryStore using the supplied clock.
func NewWithClock(now Clock) *MemoryStore {
	return &MemoryStore{
		members:      make(map[int64]domain.Member),
		emailIndex:   make(map[string]int64),
		rewards:      make(map[int64]domain.Reward),
		rewardsByMem: make(map[int64][]int64),
		nextMemberID: 1,
		nextRewardID: 1,
		now:          now,
	}
}

// CreateMember stores a new member. Email uniqueness is enforced
// case-insensitively. Returns domain.ErrDuplicateEmail on conflict.
func (s *MemoryStore) CreateMember(name, email string) (domain.Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := normalizeEmail(email)
	if _, exists := s.emailIndex[key]; exists {
		return domain.Member{}, domain.ErrDuplicateEmail
	}

	m := domain.Member{
		ID:        s.nextMemberID,
		Name:      name,
		Email:     email,
		CreatedAt: s.now().UTC(),
	}
	s.members[m.ID] = m
	s.emailIndex[key] = m.ID
	s.nextMemberID++
	return m, nil
}

// GetMember returns the member by id or domain.ErrMemberNotFound.
func (s *MemoryStore) GetMember(id int64) (domain.Member, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getMemberLocked(id)
}

func (s *MemoryStore) getMemberLocked(id int64) (domain.Member, error) {
	m, ok := s.members[id]
	if !ok {
		return domain.Member{}, domain.ErrMemberNotFound
	}
	return m, nil
}

// ListRewards returns all reward entries for a member in insertion order.
// Returns domain.ErrMemberNotFound if the member does not exist.
func (s *MemoryStore) ListRewards(memberID int64) ([]domain.Reward, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, err := s.getMemberLocked(memberID); err != nil {
		return nil, err
	}
	return s.listRewardsLocked(memberID), nil
}

func (s *MemoryStore) listRewardsLocked(memberID int64) []domain.Reward {
	ids := s.rewardsByMem[memberID]
	out := make([]domain.Reward, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.rewards[id])
	}
	return out
}

// Balance returns the member's available balance, or domain.ErrMemberNotFound.
func (s *MemoryStore) Balance(memberID int64) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, err := s.getMemberLocked(memberID); err != nil {
		return 0, err
	}
	return domain.Balance(s.listRewardsLocked(memberID)), nil
}

// CreateReward validates and stores a reward entry. The caller passes a
// positive magnitude; the store applies the sign based on point type. For
// redemptions it ensures the resulting balance does not go negative. The whole
// check-then-write is performed under a single write lock so concurrent
// redemptions cannot both pass the balance check (no overdraft race).
func (s *MemoryStore) CreateReward(memberID int64, pt domain.PointType, magnitude int64, description string) (domain.Reward, error) {
	signed, err := domain.SignedPoints(pt, magnitude)
	if err != nil {
		return domain.Reward{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.getMemberLocked(memberID); err != nil {
		return domain.Reward{}, err
	}

	if pt.IsDebit() {
		current := domain.Balance(s.listRewardsLocked(memberID))
		if current+signed < 0 {
			return domain.Reward{}, domain.ErrInsufficientBalance
		}
	}

	r := domain.Reward{
		ID:          s.nextRewardID,
		MemberID:    memberID,
		PointTypeID: pt,
		Points:      signed,
		Description: description,
		EventDate:   s.now().UTC(),
	}
	s.rewards[r.ID] = r
	s.rewardsByMem[memberID] = append(s.rewardsByMem[memberID], r.ID)
	s.nextRewardID++
	return r, nil
}

func normalizeEmail(email string) string {
	out := make([]byte, len(email))
	for i := 0; i < len(email); i++ {
		c := email[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}
