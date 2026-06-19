package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"rewards-ledger/internal/store"
)

func newTestServer() http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(store.New(), logger).Routes()
}

func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCreateMemberEndpoint(t *testing.T) {
	h := newTestServer()

	rec := do(t, h, "POST", "/members", map[string]string{
		"name": "Alice Johnson", "email": "alice@example.com",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}

	var resp memberResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.MemberID != 1 || resp.Email != "alice@example.com" {
		t.Errorf("unexpected response: %+v", resp)
	}

	// Duplicate email -> 409.
	rec = do(t, h, "POST", "/members", map[string]string{
		"name": "Bob", "email": "alice@example.com",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409", rec.Code)
	}
}

func TestCreateMemberValidation(t *testing.T) {
	h := newTestServer()
	rec := do(t, h, "POST", "/members", map[string]string{"name": "", "email": ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestFullLedgerFlow(t *testing.T) {
	h := newTestServer()
	do(t, h, "POST", "/members", map[string]string{"name": "Alice", "email": "alice@example.com"})

	type rwReq struct {
		MemberID    int64  `json:"member_id"`
		PointTypeID int    `json:"point_type_id"`
		Points      int64  `json:"points"`
		Description string `json:"description"`
	}
	entries := []rwReq{
		{1, 1, 500, "Purchase at Store A"},
		{1, 2, 200, "Referred user Bob"},
		{1, 3, 50, "Cashback on bill payment"},
		{1, 4, 300, "Redeemed for voucher"},
	}
	for _, e := range entries {
		rec := do(t, h, "POST", "/rewards", e)
		if rec.Code != http.StatusCreated {
			t.Fatalf("reward %q status = %d, body=%s", e.Description, rec.Code, rec.Body.String())
		}
	}

	// Redemption sign should be applied by the system.
	var lastReward rewardResponse
	rec := do(t, h, "GET", "/members/1/rewards", nil)
	var list []rewardResponse
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 4 {
		t.Fatalf("expected 4 rewards, got %d", len(list))
	}
	lastReward = list[3]
	if lastReward.Points != -300 {
		t.Errorf("redemption points = %d, want -300", lastReward.Points)
	}

	// Balance via member endpoint.
	rec = do(t, h, "GET", "/members/1", nil)
	var mem memberWithBalanceResponse
	json.Unmarshal(rec.Body.Bytes(), &mem)
	if mem.PointsBalance != 450 {
		t.Errorf("balance = %d, want 450", mem.PointsBalance)
	}
}

func TestRewardValidationErrors(t *testing.T) {
	h := newTestServer()
	do(t, h, "POST", "/members", map[string]string{"name": "Alice", "email": "a@example.com"})

	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{"unknown member", map[string]any{"member_id": 99, "point_type_id": 1, "points": 10}, http.StatusNotFound},
		{"invalid point type", map[string]any{"member_id": 1, "point_type_id": 9, "points": 10}, http.StatusBadRequest},
		{"non-positive points", map[string]any{"member_id": 1, "point_type_id": 1, "points": 0}, http.StatusBadRequest},
		{"overdraft", map[string]any{"member_id": 1, "point_type_id": 4, "points": 10}, http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, h, "POST", "/rewards", tc.body)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d; body=%s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestGetMemberNotFound(t *testing.T) {
	h := newTestServer()
	rec := do(t, h, "GET", "/members/123", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
