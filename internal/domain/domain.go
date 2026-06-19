// Package domain contains the core entities and business rules for the
// rewards points ledger. It has no dependencies on transport (HTTP) or
// storage, which keeps the rules easy to test and reason about.
package domain

import (
	"errors"
	"time"
)

// PointType identifies a category of reward entry.
type PointType int

const (
	PurchaseEarning PointType = 1 // credit
	ReferralBonus   PointType = 2 // credit
	Cashback        PointType = 3 // credit
	Redemption      PointType = 4 // debit
)

// Valid reports whether the point type is one of the known types (1-4).
func (pt PointType) Valid() bool {
	return pt >= PurchaseEarning && pt <= Redemption
}

// IsDebit reports whether entries of this type subtract from the balance.
func (pt PointType) IsDebit() bool {
	return pt == Redemption
}

// Member is a customer account in the rewards system.
type Member struct {
	ID        int64     `json:"member_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// Reward is a single ledger entry associated with a member's account.
// Points is stored already-signed: positive for credits, negative for debits.
type Reward struct {
	ID          int64     `json:"reward_id"`
	MemberID    int64     `json:"member_id"`
	PointTypeID PointType `json:"point_type_id"`
	Points      int64     `json:"points"`
	Description string    `json:"description"`
	EventDate   time.Time `json:"event_date"`
}

// Domain-level errors. The transport layer maps these to HTTP status codes.
var (
	ErrDuplicateEmail      = errors.New("a member with this email already exists")
	ErrMemberNotFound      = errors.New("member not found")
	ErrInvalidPointType    = errors.New("point_type_id must be a valid type (1-4)")
	ErrNonPositivePoints   = errors.New("points must be a positive number")
	ErrInsufficientBalance = errors.New("redemption exceeds the member's available balance")
)

// SignedPoints converts a caller-supplied positive magnitude into the signed
// value that should be stored, based on the point type. The system — not the
// caller — owns the sign. Returns ErrNonPositivePoints if magnitude <= 0 and
// ErrInvalidPointType if the type is unknown.
func SignedPoints(pt PointType, magnitude int64) (int64, error) {
	if !pt.Valid() {
		return 0, ErrInvalidPointType
	}
	if magnitude <= 0 {
		return 0, ErrNonPositivePoints
	}
	if pt.IsDebit() {
		return -magnitude, nil
	}
	return magnitude, nil
}

// Balance sums the signed point values of all reward entries.
func Balance(rewards []Reward) int64 {
	var total int64
	for _, r := range rewards {
		total += r.Points
	}
	return total
}
