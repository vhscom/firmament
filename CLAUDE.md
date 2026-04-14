# CLAUDE.md — Firmament project conventions

## Module

`github.com/vhscom/firmament` — Go module, root package `firmament`.

## Package layout

```
.                     # firmament package: core types and logic
cmd/firmament/        # main daemon binary
cmd/firmament-hook/   # Claude Code companion hook binary
docs/adr/             # Architecture Decision Records
```

All core types (`Event`, `Signal`, `Pattern`, `Monitor`, etc.) live in the root package. Binaries import the root package; they do not re-export its types.

## Key types and their files

| Type | File |
|---|---|
| `Event` | `event.go` |
| `Signal`, `SignalType`, `ValidSignalTypes` | `signal.go` |
| `Permission`, `AllPermissions`, `ResolvePermissions` | `permission.go` |
| `EventRing` | `eventring.go` |
| `EventSource` (interface) | `eventsource.go` |
| `HookEventSource` | `eventsource_hook.go` |
| `Pattern` (interface), `ActionConcealmentPattern`, `PatternByName` | `patterns.go` |
| `Monitor` | `monitor.go` |
| `Credential`, `CredentialStore`, `CredentialTier` | `credential.go` |
| `Router`, `SignalHandler`, `LogHandler`, `CallbackHandler` | `router.go` |
| `Config`, `LoadConfig` | `config.go` |

## Coding conventions

- Idiomatic Go. Small interfaces. No frameworks.
- JSDoc-style comments on all exported types, functions, and constants.
- Table-driven tests in `_test.go` files alongside the code they test.
- Use `sync.RWMutex` for concurrent state; prefer value copies over shared slices.
- HTTP handlers validate `Content-Type` and enforce body size limits via `http.MaxBytesReader`.
- Non-blocking channel sends (select with default) instead of blocking the caller.
- `context.Context` is the first parameter of any function that blocks or runs goroutines.

## Adding a new behavioral pattern

1. Add a struct implementing `Pattern` in `patterns.go`.
2. Register it in `PatternByName`.
3. Add a table-driven test in `patterns_test.go` covering sequences that fire and sequences that do not.
4. Add the pattern name to the `patterns` list in `firmament.yaml` if it should run by default.

## Adding a new EventSource

1. Create `eventsource_<name>.go` implementing `EventSource`.
2. Add `_test.go` using `net/http/httptest` or a fake transport.
3. Wire it in `cmd/firmament/main.go`.

## Testing

```sh
go test -race ./...
go vet ./...
```

All tests must pass with `-race`. No skipped tests in CI.

## Security checklist (apply to every PR)

- No hardcoded secrets, tokens, or credentials.
- HTTP handlers: `Content-Type` check, `http.MaxBytesReader` body cap.
- No path traversal: never use user-supplied strings as file paths without validation.
- Input from hooks (stdin, HTTP) is treated as untrusted.
- `ValidSignalTypes` is an allowlist; extending it requires a code change.
- Hook server binds loopback only by default.

## Commit style

Single-line commit messages, all lowercase, 5–7 words max.

Examples:
- `add action concealment pattern`
- `fix eventring snapshot off-by-one`
- `add hook server size limit`

## Dependencies

Minimal. Currently:
- `github.com/google/uuid` — event ID generation
- `gopkg.in/yaml.v3` — config file parsing

Avoid adding frameworks. Prefer standard library.
