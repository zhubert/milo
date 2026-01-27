# CLAUDE.md

## Language

This is a Go project. Use idiomatic Go conventions throughout.

## Error Handling

**Every error must be explicitly handled. No exceptions.**

- Never use `_` to discard an error value. Every `error` return must be checked.
- Return errors to the caller with added context using `fmt.Errorf("doing thing: %w", err)`.
- Use `%w` (not `%v`) when wrapping errors to preserve the error chain for `errors.Is` / `errors.As`.
- Only log-and-return at the top-level boundary (e.g., `main`, HTTP handler). Everywhere else, propagate.
- Do not use `log.Fatal` or `os.Exit` outside of `main`.
- When multiple cleanup steps can each fail, handle every error individually rather than silently ignoring any of them.

## Code Style

- Follow standard `gofmt` / `goimports` formatting.
- Use the stdlib and standard patterns before reaching for third-party dependencies.
- Keep functions short and focused on a single responsibility.
- Prefer returning early over deeply nested `if/else` blocks.
- Use named return values sparingly, and only when they genuinely improve readability.

## Naming

- Use `MixedCaps` / `mixedCaps`, never underscores in Go names.
- Acronyms should be all-caps (`HTTP`, `ID`, `URL`), not `Http`, `Id`, `Url`.
- Package names are lowercase, single-word where possible. No `_` or `mixedCaps`.
- Interface names should not have an `I` prefix. Use the `-er` convention when it fits (`Reader`, `Closer`).

## Project Structure

- Keep `main.go` in the project root or under `cmd/<binary-name>/`.
- Group related code into packages by domain, not by type (avoid generic `models/`, `utils/` packages).
- Tests live alongside the code they test (`foo_test.go` next to `foo.go`).

## Testing

- Write table-driven tests where there are multiple cases for the same logic.
- Use `t.Helper()` in test helper functions so failure output points to the caller.
- Use `t.Parallel()` for tests that are safe to run concurrently.
- Assert errors explicitly in tests too: check that errors are returned when expected and that they wrap correctly.

## Dependencies

- Run `go mod tidy` after adding or removing imports.
- Vet new dependencies carefully. Prefer well-maintained, widely-used libraries.
