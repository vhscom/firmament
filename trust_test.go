package firmament

import (
	"errors"
	"math"
	"sync"
	"testing"
)

func TestNewTrustScoreDefaults(t *testing.T) {
	ts := NewTrustScore()
	if ts.Ability != defaultTrust {
		t.Errorf("Ability: got %v want %v", ts.Ability, defaultTrust)
	}
	if ts.Benevolence != defaultTrust {
		t.Errorf("Benevolence: got %v want %v", ts.Benevolence, defaultTrust)
	}
	if ts.Integrity != defaultTrust {
		t.Errorf("Integrity: got %v want %v", ts.Integrity, defaultTrust)
	}
}

func TestTrustScoreGeometricMean(t *testing.T) {
	ts := TrustScore{Ability: 0.8, Benevolence: 0.6, Integrity: 0.9}
	want := math.Cbrt(0.8 * 0.6 * 0.9)
	if got := ts.Score(); math.Abs(got-want) > 1e-9 {
		t.Errorf("Score() = %v, want %v", got, want)
	}
}

func TestTrustScoreScoreNeutral(t *testing.T) {
	ts := NewTrustScore()
	// geometric mean of 0.5^3 = 0.5
	if got := ts.Score(); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("neutral score = %v, want 0.5", got)
	}
}

func TestTrustScoreScoreCollapseOneDimension(t *testing.T) {
	ts := TrustScore{Ability: 1.0, Benevolence: 1.0, Integrity: 0.0}
	if got := ts.Score(); got != 0.0 {
		t.Errorf("zero integrity should collapse score to 0, got %v", got)
	}
}

func TestUpdateFromReviewPassed(t *testing.T) {
	ts := NewTrustScore()
	before := ts.Ability
	ts.UpdateFromReview(true)
	if ts.Ability <= before {
		t.Error("Ability should increase on passed review")
	}
	if ts.Benevolence <= defaultTrust {
		t.Error("Benevolence should increase on passed review")
	}
	// Integrity unchanged by review.
	if ts.Integrity != defaultTrust {
		t.Error("Integrity should not change on review")
	}
}

func TestUpdateFromReviewFailed(t *testing.T) {
	ts := NewTrustScore()
	ts.UpdateFromReview(false)
	if ts.Ability >= defaultTrust {
		t.Error("Ability should decrease on failed review")
	}
	if ts.Benevolence >= defaultTrust {
		t.Error("Benevolence should decrease on failed review")
	}
}

func TestUpdateFromSelfReportConsistent(t *testing.T) {
	ts := NewTrustScore()
	ts.UpdateFromSelfReport(true)
	if ts.Integrity <= defaultTrust {
		t.Error("Integrity should increase on consistent self-report")
	}
	// Ability and Benevolence unchanged.
	if ts.Ability != defaultTrust || ts.Benevolence != defaultTrust {
		t.Error("Ability/Benevolence should not change on self-report")
	}
}

func TestUpdateFromSelfReportInconsistent(t *testing.T) {
	ts := NewTrustScore()
	ts.UpdateFromSelfReport(false)
	if ts.Integrity >= defaultTrust {
		t.Error("Integrity should decrease on inconsistent self-report")
	}
}

func TestDecayReducesAllDimensions(t *testing.T) {
	ts := NewTrustScore()
	ts.Decay()
	if ts.Ability >= defaultTrust {
		t.Error("Ability should decrease after Decay")
	}
	if ts.Benevolence >= defaultTrust {
		t.Error("Benevolence should decrease after Decay")
	}
	if ts.Integrity >= defaultTrust {
		t.Error("Integrity should decrease after Decay")
	}
}

func TestTrustClampFloor(t *testing.T) {
	ts := TrustScore{Ability: 0.01, Benevolence: 0.5, Integrity: 0.5}
	// Drive Ability to floor.
	for i := 0; i < 100; i++ {
		ts.UpdateFromReview(false)
	}
	if ts.Ability < 0 {
		t.Errorf("Ability went below floor: %v", ts.Ability)
	}
}

func TestTrustClampCeil(t *testing.T) {
	ts := TrustScore{Ability: 0.99, Benevolence: 0.5, Integrity: 0.5}
	for i := 0; i < 100; i++ {
		ts.UpdateFromReview(true)
	}
	if ts.Ability > 1.0 {
		t.Errorf("Ability exceeded ceil: %v", ts.Ability)
	}
}

func TestDecayFloor(t *testing.T) {
	ts := NewTrustScore()
	for i := 0; i < 1000; i++ {
		ts.Decay()
	}
	if ts.Ability < 0 || ts.Benevolence < 0 || ts.Integrity < 0 {
		t.Errorf("decay went below zero: %+v", ts)
	}
}

// TrustStore tests

func TestMemoryTrustStoreRoundTrip(t *testing.T) {
	store := NewMemoryTrustStore()
	ts := NewTrustScore()
	ts.Ability = 0.8

	if err := store.Set("sess-1", ts); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := store.Get("sess-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Ability != 0.8 {
		t.Errorf("Ability: got %v want 0.8", got.Ability)
	}
}

func TestMemoryTrustStoreNotFound(t *testing.T) {
	store := NewMemoryTrustStore()
	_, err := store.Get("missing")
	if !errors.Is(err, ErrTrustNotFound) {
		t.Errorf("want ErrTrustNotFound, got %v", err)
	}
}

func TestMemoryTrustStoreDelete(t *testing.T) {
	store := NewMemoryTrustStore()
	_ = store.Set("s", NewTrustScore())
	_ = store.Delete("s")
	_, err := store.Get("s")
	if !errors.Is(err, ErrTrustNotFound) {
		t.Errorf("want ErrTrustNotFound after delete, got %v", err)
	}
}

func TestMemoryTrustStoreDeleteNoOp(t *testing.T) {
	store := NewMemoryTrustStore()
	if err := store.Delete("ghost"); err != nil {
		t.Errorf("delete non-existent: %v", err)
	}
}

func TestMemoryTrustStoreOverwrite(t *testing.T) {
	store := NewMemoryTrustStore()
	ts1 := NewTrustScore()
	ts2 := NewTrustScore()
	ts2.Ability = 0.9
	_ = store.Set("s", ts1)
	_ = store.Set("s", ts2)
	got, _ := store.Get("s")
	if got.Ability != 0.9 {
		t.Errorf("overwrite failed: got %v", got.Ability)
	}
}

func TestMemoryTrustStoreConcurrency(t *testing.T) {
	store := NewMemoryTrustStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ts := NewTrustScore()
			_ = store.Set("shared", ts)
			_, _ = store.Get("shared")
		}(i)
	}
	wg.Wait()
}
