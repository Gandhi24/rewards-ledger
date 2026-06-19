package domain

import (
	"errors"
	"testing"
)

func TestPointTypeValid(t *testing.T) {
	for _, pt := range []PointType{PurchaseEarning, ReferralBonus, Cashback, Redemption} {
		if !pt.Valid() {
			t.Errorf("expected %d to be valid", pt)
		}
	}
	for _, pt := range []PointType{0, 5, -1, 100} {
		if pt.Valid() {
			t.Errorf("expected %d to be invalid", pt)
		}
	}
}

func TestSignedPoints(t *testing.T) {
	tests := []struct {
		name      string
		pt        PointType
		magnitude int64
		want      int64
		wantErr   error
	}{
		{"purchase is credit", PurchaseEarning, 500, 500, nil},
		{"referral is credit", ReferralBonus, 200, 200, nil},
		{"cashback is credit", Cashback, 50, 50, nil},
		{"redemption is debit", Redemption, 300, -300, nil},
		{"invalid type", PointType(9), 100, 0, ErrInvalidPointType},
		{"zero points", PurchaseEarning, 0, 0, ErrNonPositivePoints},
		{"negative points", PurchaseEarning, -10, 0, ErrNonPositivePoints},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SignedPoints(tc.pt, tc.magnitude)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestBalance(t *testing.T) {
	rewards := []Reward{
		{Points: 500}, {Points: 200}, {Points: 50}, {Points: -300},
	}
	if got := Balance(rewards); got != 450 {
		t.Errorf("got %d, want 450", got)
	}
	if got := Balance(nil); got != 0 {
		t.Errorf("empty balance got %d, want 0", got)
	}
}
