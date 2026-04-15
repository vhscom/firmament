package firmament

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	// installationKeyFile is the path relative to the user's home directory
	// where the per-installation HMAC key is stored.
	installationKeyFile = ".firmament/installation.key"

	// installationKeyBytes is the size of the installation secret in bytes (256 bits).
	installationKeyBytes = 32

	// agentIDHexLen is the output length of DeriveAgentID in hex characters (128 bits).
	agentIDHexLen = 32
)

// DeriveAgentID computes HMAC-SHA256(installationSecret, credentialFingerprint)
// and returns the first 128 bits as a 32-character lowercase hex string.
//
// Research basis: ADR-004 Decision 5. The construction satisfies four constraints:
//
//   - Stable identity: the same credential always produces the same agent_id,
//     enabling GetAgentTrustHistory to correlate sessions for the Mayer-Davis-Schoorman
//     integrity dimension (perceived consistency-over-time).
//
//   - Installation scoping: different installations produce different agent_ids
//     for the same credential (because installationSecret differs), preserving the
//     information asymmetry between installations as described in
//     syntheses/information asymmetry as resource vs threat.
//
//   - One-way property: the stored agent_id cannot be reversed to the underlying
//     credential — the structural analog of Fox & Jordan's (2011) ex-post
//     verification posture ("structured reporting, not total surveillance").
//
//   - HMAC-not-hash: using HMAC instead of a plain hash prevents offline
//     brute-forcing of credentialFingerprint values against a stolen agent_id.
func DeriveAgentID(installationSecret []byte, credentialFingerprint string) AgentID {
	mac := hmac.New(sha256.New, installationSecret)
	mac.Write([]byte(credentialFingerprint))
	full := mac.Sum(nil) // 32 bytes = 256 bits
	return AgentID(hex.EncodeToString(full[:agentIDHexLen/2])) // truncate to 128 bits
}

// CredentialFingerprint computes a stable fingerprint for a credential identifier.
// It is a hex-encoded SHA-256 hash of the credential ID string.
// This is the credentialFingerprint argument to DeriveAgentID.
func CredentialFingerprint(credentialID string) string {
	h := sha256.Sum256([]byte(credentialID))
	return hex.EncodeToString(h[:])
}

// LoadOrCreateInstallationSecret reads the per-installation HMAC key from
// ~/.firmament/installation.key. If the file does not exist, a new cryptographically
// random 256-bit key is generated, written to disk with mode 0600, and returned.
//
// The installation.key file is the root of agent identity stability within a
// Firmament installation. Its loss means historical agent_id values can no
// longer be correlated with future sessions for the same underlying credential.
// Back it up if long-term trust history is important.
func LoadOrCreateInstallationSecret() ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("installation secret: get home dir: %w", err)
	}
	path := filepath.Join(home, installationKeyFile)

	data, err := os.ReadFile(path)
	if err == nil {
		// File exists — decode the hex-encoded key.
		key, decErr := hex.DecodeString(string(data))
		if decErr == nil && len(key) == installationKeyBytes {
			return key, nil
		}
		// File exists but is malformed — generate a fresh key and overwrite.
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("installation secret: read %q: %w", path, err)
	}

	// Generate a new installation secret.
	key := make([]byte, installationKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("installation secret: generate random key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("installation secret: create dir: %w", err)
	}
	encoded := hex.EncodeToString(key)
	if err := os.WriteFile(path, []byte(encoded), 0600); err != nil {
		return nil, fmt.Errorf("installation secret: write %q: %w", path, err)
	}
	return key, nil
}

// DefaultAgentID derives the agent ID from the current installation's secret
// and the given credential ID. Returns an empty AgentID on error.
//
// This is a convenience wrapper for cases where the installation secret is
// loaded on demand (e.g., in the watch daemon). Callers that manage the
// installation secret lifecycle explicitly should call DeriveAgentID directly.
func DefaultAgentID(credentialID string) (AgentID, error) {
	secret, err := LoadOrCreateInstallationSecret()
	if err != nil {
		return "", err
	}
	return DeriveAgentID(secret, CredentialFingerprint(credentialID)), nil
}
