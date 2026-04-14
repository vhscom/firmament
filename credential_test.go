package firmament

import (
	"errors"
	"testing"
)

func TestCredentialSupervisorAlwaysFullPermissions(t *testing.T) {
	c := Credential{
		ID:   "sup-1",
		Name: "monitor",
		Tier: TierSupervisor,
		// Even with restrictive overrides, supervisor gets everything.
		PermissionOverrides: []Permission{PermQueryEvents},
	}

	perms := c.Permissions()
	if len(perms) != len(AllPermissions) {
		t.Errorf("supervisor: got %d permissions, want %d", len(perms), len(AllPermissions))
	}
	for p := range AllPermissions {
		if _, ok := perms[p]; !ok {
			t.Errorf("supervisor missing permission %q", p)
		}
	}
}

func TestCredentialStandardRespectsOverrides(t *testing.T) {
	c := Credential{
		ID:                  "std-1",
		Name:                "target",
		Tier:                TierStandard,
		PermissionOverrides: []Permission{PermQueryEvents, PermSubscribeEvents, PermSignal},
	}

	perms := c.Permissions()
	if len(perms) != 3 {
		t.Errorf("standard: got %d permissions, want 3", len(perms))
	}
	// Restricted permissions must be absent.
	for _, p := range []Permission{PermCloakControl, PermRevokeSession, PermKeyRotate} {
		if _, ok := perms[p]; ok {
			t.Errorf("permission %q should be absent for restricted standard credential", p)
		}
	}
}

func TestCredentialStandardNilOverridesFullSet(t *testing.T) {
	c := Credential{ID: "std-2", Name: "default", Tier: TierStandard}
	perms := c.Permissions()
	if len(perms) != len(AllPermissions) {
		t.Errorf("got %d permissions, want %d", len(perms), len(AllPermissions))
	}
}

func TestCredentialHas(t *testing.T) {
	c := Credential{
		ID:                  "std-3",
		Tier:                TierStandard,
		PermissionOverrides: []Permission{PermQueryEvents},
	}
	if !c.Has(PermQueryEvents) {
		t.Error("should have PermQueryEvents")
	}
	if c.Has(PermSignal) {
		t.Error("should not have PermSignal")
	}
}

func TestCredentialStoreRoundTrip(t *testing.T) {
	store := NewMemCredentialStore()

	c := Credential{ID: "c1", Name: "test", Tier: TierStandard}
	if err := store.Put(c); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Get("c1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != c.ID || got.Name != c.Name {
		t.Errorf("got %+v, want %+v", got, c)
	}
}

func TestCredentialStoreGetNotFound(t *testing.T) {
	store := NewMemCredentialStore()
	_, err := store.Get("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestCredentialStoreDelete(t *testing.T) {
	store := NewMemCredentialStore()
	c := Credential{ID: "del-1", Name: "to-delete", Tier: TierStandard}
	_ = store.Put(c)
	_ = store.Delete("del-1")

	_, err := store.Get("del-1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete, want ErrNotFound, got %v", err)
	}
}

func TestCredentialStoreDeleteNoOp(t *testing.T) {
	store := NewMemCredentialStore()
	// Deleting a non-existent credential should not error.
	if err := store.Delete("ghost"); err != nil {
		t.Errorf("delete non-existent: %v", err)
	}
}

func TestCredentialStorePutOverwrites(t *testing.T) {
	store := NewMemCredentialStore()
	c1 := Credential{ID: "c1", Name: "original", Tier: TierStandard}
	c2 := Credential{ID: "c1", Name: "updated", Tier: TierSupervisor}
	_ = store.Put(c1)
	_ = store.Put(c2)

	got, err := store.Get("c1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "updated" || got.Tier != TierSupervisor {
		t.Errorf("overwrite failed: got %+v", got)
	}
}

func TestTierResolution(t *testing.T) {
	tests := []struct {
		tier      CredentialTier
		overrides []Permission
		wantFull  bool
	}{
		{TierSupervisor, nil, true},
		{TierSupervisor, []Permission{PermQueryEvents}, true}, // overrides ignored
		{TierStandard, nil, true},
		{TierStandard, []Permission{PermQueryEvents}, false},
	}

	for _, tt := range tests {
		c := Credential{ID: "x", Tier: tt.tier, PermissionOverrides: tt.overrides}
		perms := c.Permissions()
		full := len(perms) == len(AllPermissions)
		if full != tt.wantFull {
			t.Errorf("tier=%s overrides=%v: wantFull=%v got full=%v", tt.tier, tt.overrides, tt.wantFull, full)
		}
	}
}
