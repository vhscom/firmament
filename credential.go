package firmament

import (
	"errors"
	"sync"
)

// CredentialTier controls how permission resolution behaves for a credential.
type CredentialTier string

const (
	// TierStandard credentials receive permissions filtered through ResolvePermissions.
	// Their overrides can only restrict from AllPermissions.
	TierStandard CredentialTier = "standard"

	// TierSupervisor credentials always receive AllPermissions regardless of any
	// PermissionOverrides. Supervisor tier is provisioned separately and cannot be
	// revoked by a standard-tier credential operation.
	TierSupervisor CredentialTier = "supervisor"
)

// Credential represents an identity with associated permissions.
// It models the monitoring relationship: supervisor credentials observe
// and signal; standard credentials operate within a restricted permission set.
type Credential struct {
	// ID is the unique identifier for this credential.
	ID string

	// Name is a human-readable label.
	Name string

	// Tier controls permission resolution. Supervisor tier cannot be restricted.
	Tier CredentialTier

	// PermissionOverrides restricts the effective permissions for standard-tier
	// credentials. Null or empty = full AllPermissions set. Supervisor-tier
	// credentials ignore this field.
	PermissionOverrides []Permission
}

// Permissions returns the effective permission set for this credential.
// Supervisor credentials always receive AllPermissions.
// Standard credentials receive the intersection of PermissionOverrides and AllPermissions.
func (c *Credential) Permissions() map[Permission]struct{} {
	if c.Tier == TierSupervisor {
		result := make(map[Permission]struct{}, len(AllPermissions))
		for p := range AllPermissions {
			result[p] = struct{}{}
		}
		return result
	}
	return ResolvePermissions(c.PermissionOverrides)
}

// Has reports whether the credential holds the given permission.
func (c *Credential) Has(p Permission) bool {
	_, ok := c.Permissions()[p]
	return ok
}

// ErrNotFound is returned by CredentialStore when a credential is not found.
var ErrNotFound = errors.New("credential not found")

// CredentialStore is the interface for credential persistence.
type CredentialStore interface {
	// Get retrieves a credential by ID. Returns ErrNotFound if absent.
	Get(id string) (*Credential, error)

	// Put inserts or replaces a credential.
	Put(c Credential) error

	// Delete removes a credential by ID. No-ops if not found.
	Delete(id string) error
}

// memCredentialStore is a thread-safe in-memory CredentialStore.
type memCredentialStore struct {
	mu    sync.RWMutex
	store map[string]Credential
}

// NewMemCredentialStore creates an in-memory CredentialStore.
func NewMemCredentialStore() CredentialStore {
	return &memCredentialStore{store: make(map[string]Credential)}
}

func (s *memCredentialStore) Get(id string) (*Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.store[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &c, nil
}

func (s *memCredentialStore) Put(c Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[c.ID] = c
	return nil
}

func (s *memCredentialStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, id)
	return nil
}
