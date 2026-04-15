package firmament

import (
	"testing"
)

func TestDeriveAgentID(t *testing.T) {
	secret := []byte("testsecret-32-bytes-padding-here!")
	fp := "credential-fingerprint"

	id := DeriveAgentID(secret, fp)
	if id == "" {
		t.Fatal("DeriveAgentID returned empty ID")
	}
	if len(string(id)) != agentIDHexLen {
		t.Fatalf("DeriveAgentID length: got %d, want %d", len(string(id)), agentIDHexLen)
	}

	// Deterministic: same inputs produce same output.
	id2 := DeriveAgentID(secret, fp)
	if id != id2 {
		t.Fatalf("DeriveAgentID not deterministic: %q != %q", id, id2)
	}

	// Different credential fingerprint produces different ID.
	other := DeriveAgentID(secret, "other-fingerprint")
	if id == other {
		t.Fatal("DeriveAgentID: different fingerprints produced same ID")
	}

	// Different installation secret produces different ID.
	secret2 := []byte("different-secret-32-bytes-paddin!")
	byOtherInstall := DeriveAgentID(secret2, fp)
	if id == byOtherInstall {
		t.Fatal("DeriveAgentID: different secrets produced same ID (installation scoping broken)")
	}
}

func TestCredentialFingerprint(t *testing.T) {
	fp1 := CredentialFingerprint("cred-a")
	fp2 := CredentialFingerprint("cred-a")
	fp3 := CredentialFingerprint("cred-b")

	if fp1 != fp2 {
		t.Fatal("CredentialFingerprint: not deterministic")
	}
	if fp1 == fp3 {
		t.Fatal("CredentialFingerprint: different inputs produced same fingerprint")
	}
	// SHA-256 hex is always 64 characters.
	if len(fp1) != 64 {
		t.Fatalf("CredentialFingerprint length: got %d, want 64", len(fp1))
	}
}
