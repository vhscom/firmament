package firmament

// Permission names a capability that can be granted to a credential.
type Permission string

const (
	// PermQueryEvents allows reading events from the EventRing.
	PermQueryEvents Permission = "query_events"

	// PermSubscribeEvents allows receiving real-time event streams.
	PermSubscribeEvents Permission = "subscribe_events"

	// PermSignal allows sending agent.signal messages to target sessions.
	PermSignal Permission = "agent_signal"

	// PermCloakControl allows enabling or disabling the cloak layer.
	PermCloakControl Permission = "cloak_control"

	// PermRevokeSession allows forcibly terminating an agent session.
	PermRevokeSession Permission = "revoke_session"

	// PermKeyRotate allows rotating credential keys.
	PermKeyRotate Permission = "key_rotate"
)

// AllPermissions is the complete set of defined permissions.
// Overrides can only restrict from this set, never expand it.
var AllPermissions = map[Permission]struct{}{
	PermQueryEvents:     {},
	PermSubscribeEvents: {},
	PermSignal:          {},
	PermCloakControl:    {},
	PermRevokeSession:   {},
	PermKeyRotate:       {},
}

// ResolvePermissions implements ADR-001 Decision 1: an intersect-only permission
// model that prevents privilege escalation through credential overrides.
// Research basis: Holmstrom-Milgrom (1991) show that monitoring systems
// themselves become distortion vectors when operators can selectively disable
// observation — a consequence-of-monitoring problem. The intersect-only design
// ensures that no override path can expand the observable surface beyond what
// the system explicitly defines in AllPermissions. See docs/adr/001-firmament-core-architecture.md.
// Related findings: ValidSignalTypes (signal.go) applies the same allowlist
// principle to signal emission; both are defense-in-depth against privilege
// escalation through the monitoring plane itself.
//
// ResolvePermissions intersects overrides with AllPermissions and returns
// the effective permission set for a credential.
//
// Null or empty overrides return the full set — backward compatible with
// existing credentials that carry no override.
// Permissions not in AllPermissions are silently dropped from overrides;
// overrides can only restrict, never expand, the allowed set.
func ResolvePermissions(overrides []Permission) map[Permission]struct{} {
	if len(overrides) == 0 {
		result := make(map[Permission]struct{}, len(AllPermissions))
		for p := range AllPermissions {
			result[p] = struct{}{}
		}
		return result
	}
	result := make(map[Permission]struct{})
	for _, p := range overrides {
		if _, ok := AllPermissions[p]; ok {
			result[p] = struct{}{}
		}
	}
	return result
}
