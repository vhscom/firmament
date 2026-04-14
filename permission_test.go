package firmament

import (
	"testing"
)

func TestResolvePermissionsNilReturnsAll(t *testing.T) {
	got := ResolvePermissions(nil)
	if len(got) != len(AllPermissions) {
		t.Errorf("got %d permissions, want %d", len(got), len(AllPermissions))
	}
	for p := range AllPermissions {
		if _, ok := got[p]; !ok {
			t.Errorf("permission %q missing from full set", p)
		}
	}
}

func TestResolvePermissionsEmptyReturnsAll(t *testing.T) {
	got := ResolvePermissions([]Permission{})
	if len(got) != len(AllPermissions) {
		t.Errorf("got %d permissions, want %d", len(got), len(AllPermissions))
	}
}

func TestResolvePermissionsSubset(t *testing.T) {
	overrides := []Permission{PermQueryEvents, PermSubscribeEvents, PermSignal}
	got := ResolvePermissions(overrides)

	if len(got) != 3 {
		t.Errorf("got %d permissions, want 3", len(got))
	}
	for _, p := range overrides {
		if _, ok := got[p]; !ok {
			t.Errorf("permission %q missing", p)
		}
	}
	// restricted permissions must be absent
	for _, p := range []Permission{PermCloakControl, PermRevokeSession, PermKeyRotate} {
		if _, ok := got[p]; ok {
			t.Errorf("permission %q should be absent", p)
		}
	}
}

func TestResolvePermissionsDropsUnknown(t *testing.T) {
	overrides := []Permission{"nonexistent_perm", PermQueryEvents}
	got := ResolvePermissions(overrides)
	if len(got) != 1 {
		t.Errorf("got %d permissions, want 1", len(got))
	}
	if _, ok := got["nonexistent_perm"]; ok {
		t.Error("unknown permission should be dropped")
	}
}

func TestResolvePermissionsOnlyUnknown(t *testing.T) {
	got := ResolvePermissions([]Permission{"bad_perm"})
	if len(got) != 0 {
		t.Errorf("got %d permissions, want 0", len(got))
	}
}

func TestAllPermissionsContainsAllConstants(t *testing.T) {
	constants := []Permission{
		PermQueryEvents,
		PermSubscribeEvents,
		PermSignal,
		PermCloakControl,
		PermRevokeSession,
		PermKeyRotate,
	}
	for _, c := range constants {
		if _, ok := AllPermissions[c]; !ok {
			t.Errorf("constant %q missing from AllPermissions", c)
		}
	}
}
