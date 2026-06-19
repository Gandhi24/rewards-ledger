package store

import (
	"errors"
	"sync"
	"testing"
	"time"

	"rewards-ledger/internal/domain"
)

func fixedClock() Clock {
	t := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

func TestCreateMemberAndDuplicateEmail(t *testing.T) {
	s := NewWithClock(fixedClock())

	m, err := s.CreateMember("Alice Johnson", "alice@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ID != 1 {
		t.Errorf("expected first id 1, got %d", m.ID)
	}

	// Same email, different case -> still a duplicate.
	if _, err := s.CreateMember("Alice 2", "ALICE@example.com"); !errors.Is(err, domain.ErrDuplicateEmail) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
}

func TestGetMemberNotFound(t *testing.T) {
	s := New()
	if _, err := s.GetMember(999); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestCreateRewardAppliesSignAndBalance(t *testing.T) {
	s := New()
	m, _ := s.CreateMember("Alice", "alice@example.com")

	s.CreateReward(m.ID, domain.PurchaseEarning, 500, "Purchase")
	s.CreateReward(m.ID, domain.ReferralBonus, 200, "Referral")
	s.CreateReward(m.ID, domain.Cashback, 50, "Cashback")
	r, err := s.CreateReward(m.ID, domain.Redemption, 300, "Voucher")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Points != -300 {
		t.Errorf("redemption stored as %d, want -300", r.Points)
	}

	bal, _ := s.Balance(m.ID)
	if bal != 450 {
		t.Errorf("balance = %d, want 450", bal)
	}
}

func TestRedemptionCannotExceedBalance(t *testing.T) {
	s := New()
	m, _ := s.CreateMember("Alice", "alice@example.com")
	s.CreateReward(m.ID, domain.PurchaseEarning, 100, "Purchase")

	if _, err := s.CreateReward(m.ID, domain.Redemption, 101, "Too much"); !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf("expected insufficient balance, got %v", err)
	}
	// Exact balance is allowed (brings balance to 0).
	if _, err := s.CreateReward(m.ID, domain.Redemption, 100, "Exact"); err != nil {
		t.Fatalf("exact redemption should succeed, got %v", err)
	}
}

func TestCreateRewardUnknownMember(t *testing.T) {
	s := New()
	if _, err := s.CreateReward(42, domain.PurchaseEarning, 10, "x"); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Fatalf("expected member not found, got %v", err)
	}
}

func TestListRewardsOrder(t *testing.T) {
	s := New()
	m, _ := s.CreateMember("Alice", "alice@example.com")
	s.CreateReward(m.ID, domain.PurchaseEarning, 1, "first")
	s.CreateReward(m.ID, domain.PurchaseEarning, 2, "second")

	rewards, err := s.ListRewards(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rewards) != 2 || rewards[0].Description != "first" || rewards[1].Description != "second" {
		t.Errorf("rewards not in insertion order: %+v", rewards)
	}
}

// TestConcurrentRedemptionsNoOverdraft fires many simultaneous redemptions
// against a small balance and asserts the account never goes negative.
func TestConcurrentRedemptionsNoOverdraft(t *testing.T) {
	s := New()
	m, _ := s.CreateMember("Alice", "alice@example.com")
	s.CreateReward(m.ID, domain.PurchaseEarning, 100, "seed")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.CreateReward(m.ID, domain.Redemption, 10, "concurrent")
		}()
	}
	wg.Wait()

	bal, _ := s.Balance(m.ID)
	if bal < 0 {
		t.Fatalf("balance went negative: %d", bal)
	}
	if bal != 0 {
		// 100 / 10 = exactly 10 redemptions should have succeeded.
		t.Errorf("balance = %d, want 0", bal)
	}
}
