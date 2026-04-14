package firmament

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sync"
)

// TrustScore implements ADR-002 Decision 2: the Mayer-Davis-Schoorman (1995)
// three-axis trust model as the session trust representation.
// Research basis: Mayer, Davis & Schoorman (1995) establish that trustworthiness
// comprises Ability (competence), Benevolence (alignment of interests), and
// Integrity (consistency between stated values and observed behavior). Their
// meta-analysis shows these dimensions are empirically separable and interact
// multiplicatively — a collapse in one depresses the aggregate. See docs/adr/002-trust-model.md.
// Related findings: the "trust and control interact" synthesis (research graph)
// establishes that trust and formal controls are complements, not substitutes;
// higher trust reduces monitoring intensity (via TrustThreshold in Constitution)
// while maintaining structural oversight through Critical patterns.
//
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

// FileTrustStore is a file-backed TrustStore that persists scores as JSON.
// It delegates all read/write operations to an embedded MemoryTrustStore.
// Call LoadFromFile to initialize; call SaveToFile to persist after mutations.
type FileTrustStore struct {
	path string
	mem  MemoryTrustStore
}

// LoadFromFile reads a JSON trust store from path and returns a FileTrustStore.
// If the file does not exist, an empty store is returned without error.
func LoadFromFile(path string) (*FileTrustStore, error) {
	store := &FileTrustStore{
		path: path,
		mem:  MemoryTrustStore{scores: make(map[string]TrustScore)},
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return store, nil
		}
		return nil, fmt.Errorf("load trust store %q: %w", path, err)
	}
	store.mem.mu.Lock()
	err = json.Unmarshal(data, &store.mem.scores)
	store.mem.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("parse trust store %q: %w", path, err)
	}
	return store, nil
}

// SaveToFile serializes the trust store to its configured path as JSON.
// The parent directory is created if it does not exist.
func (s *FileTrustStore) SaveToFile() error {
	s.mem.mu.RLock()
	data, err := json.MarshalIndent(s.mem.scores, "", "  ")
	s.mem.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal trust store: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("create trust store dir: %w", err)
	}
	return os.WriteFile(s.path, data, 0600)
}

// Scores returns a copy of all session scores keyed by session ID.
func (s *FileTrustStore) Scores() map[string]TrustScore {
	s.mem.mu.RLock()
	defer s.mem.mu.RUnlock()
	out := make(map[string]TrustScore, len(s.mem.scores))
	for k, v := range s.mem.scores {
		out[k] = v
	}
	return out
}

// Get implements TrustStore.
func (s *FileTrustStore) Get(sessionID string) (TrustScore, error) {
	return s.mem.Get(sessionID)
}

// Set implements TrustStore.
func (s *FileTrustStore) Set(sessionID string, score TrustScore) error {
	return s.mem.Set(sessionID, score)
}

// Delete implements TrustStore.
func (s *FileTrustStore) Delete(sessionID string) error {
	return s.mem.Delete(sessionID)
}
