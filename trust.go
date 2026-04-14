package firmament

import (
	"errors"
	"math"
	"sync"
)

// TrustScore holds the three dimensions of agent trust as defined by
// Mayer, Davis & Schoorman (1995): Ability, Benevolence, and Integrity.
// Each dimension is a float in [0, 1]. All three start at 0.5 (neutral).
//
// Score() returns the geometric mean of the three dimensions so that a
// collapse in any single dimension pulls the aggregate down sharply.
type TrustScore struct {
	// Ability is the agent's demonstrated competence in performing its task.
	Ability float64

	// Benevolence measures whether the agent's actions align with the
	// principal's interests rather than its own.
	Benevolence float64

	// Integrity reflects consistency between the agent's stated values and
	// its observed behaviour.
	Integrity float64
}

// defaultTrust is the starting value for each dimension.
const defaultTrust = 0.5

// trustLearnRate controls how fast each review shifts the score.
const trustLearnRate = 0.05

// trustDecayRate is subtracted from each dimension on Decay.
const trustDecayRate = 0.01

// trustFloor is the minimum value any dimension can reach.
const trustFloor = 0.0

// trustCeil is the maximum value any dimension can reach.
const trustCeil = 1.0

// NewTrustScore returns a TrustScore with all dimensions at the neutral midpoint.
func NewTrustScore() TrustScore {
	return TrustScore{
		Ability:     defaultTrust,
		Benevolence: defaultTrust,
		Integrity:   defaultTrust,
	}
}

// Score returns the geometric mean of the three dimensions.
// A collapse in any single dimension depresses the aggregate sharply.
func (t TrustScore) Score() float64 {
	return math.Cbrt(t.Ability * t.Benevolence * t.Integrity)
}

// UpdateFromReview adjusts Ability and Benevolence based on an external review.
// passed=true increases both; passed=false decreases both.
func (t *TrustScore) UpdateFromReview(passed bool) {
	delta := trustLearnRate
	if !passed {
		delta = -trustLearnRate
	}
	t.Ability = clampTrust(t.Ability + delta)
	t.Benevolence = clampTrust(t.Benevolence + delta)
}

// UpdateFromSelfReport adjusts Integrity based on whether the agent's
// self-reported state was consistent with observed behaviour.
// consistent=true increases Integrity; consistent=false decreases it.
func (t *TrustScore) UpdateFromSelfReport(consistent bool) {
	delta := trustLearnRate
	if !consistent {
		delta = -trustLearnRate
	}
	t.Integrity = clampTrust(t.Integrity + delta)
}

// Decay applies a small downward drift to all three dimensions.
// Trust must be actively maintained; absence of evidence is mild evidence of absence.
func (t *TrustScore) Decay() {
	t.Ability = clampTrust(t.Ability - trustDecayRate)
	t.Benevolence = clampTrust(t.Benevolence - trustDecayRate)
	t.Integrity = clampTrust(t.Integrity - trustDecayRate)
}

// clampTrust constrains v to [trustFloor, trustCeil].
func clampTrust(v float64) float64 {
	if v < trustFloor {
		return trustFloor
	}
	if v > trustCeil {
		return trustCeil
	}
	return v
}

// ErrTrustNotFound is returned by TrustStore when no score exists for a session.
var ErrTrustNotFound = errors.New("trust score not found")

// TrustStore persists TrustScore values keyed by session ID.
type TrustStore interface {
	// Get retrieves the TrustScore for sessionID.
	// Returns ErrTrustNotFound if no score exists.
	Get(sessionID string) (TrustScore, error)

	// Set stores or replaces the TrustScore for sessionID.
	Set(sessionID string, score TrustScore) error

	// Delete removes the TrustScore for sessionID. No-op if absent.
	Delete(sessionID string) error
}

// MemoryTrustStore is a thread-safe in-memory TrustStore.
type MemoryTrustStore struct {
	mu     sync.RWMutex
	scores map[string]TrustScore
}

// NewMemoryTrustStore creates an empty MemoryTrustStore.
func NewMemoryTrustStore() *MemoryTrustStore {
	return &MemoryTrustStore{scores: make(map[string]TrustScore)}
}

// Get implements TrustStore.
func (s *MemoryTrustStore) Get(sessionID string) (TrustScore, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	score, ok := s.scores[sessionID]
	if !ok {
		return TrustScore{}, ErrTrustNotFound
	}
	return score, nil
}

// Set implements TrustStore.
func (s *MemoryTrustStore) Set(sessionID string, score TrustScore) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scores[sessionID] = score
	return nil
}

// Delete implements TrustStore.
func (s *MemoryTrustStore) Delete(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.scores, sessionID)
	return nil
}
