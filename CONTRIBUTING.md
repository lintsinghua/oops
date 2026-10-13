# Contributing

Thanks for helping with oops. Bug reports, agent adapters, and docs fixes are all welcome.

## Prerequisites

- Go 1.27.2
- [golangci-lint](https://golangci-lint.run/) v2.14.0
- GNU make

Install the linter binary (do not `go install` it into this module):

```sh
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/v2.14.0/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v2.14.0
```

## Development

```sh
make build   # bin/oops, with version metadata from git
make test    # go test -race ./...
make lint    # gofmt, go vet, golangci-lint
make fmt     # gofmt -w .
```

`make build` passes `-ldflags` that set:

- `github.com/lintsinghua/oops/internal/version.Version`
- `github.com/lintsinghua/oops/internal/version.Commit`
- `github.com/lintsinghua/oops/internal/version.Date`

Release builds set the same variables from GoReleaser. See `.goreleaser.yaml`.

Tests must not mutate the process environment for the data directory. Pass `--home` and `--user-home` (the hidden CLI flags) or the engine `Open` arguments. Packages run in parallel.

The hot-path numbers in the README come from:

```sh
OOPS_PERF=1 go test -count=1 -timeout 45m -run TestHotCapturePercentiles ./internal/engine/
```

That test is skipped unless `OOPS_PERF=1`. `make test` does not run it. Replace a published number only with a fresh run of this test, and say which machine produced it.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, or `chore:`, then a short imperative subject.

```
fix: start the snapshot deadline when the scan starts
```

## Sign-off

Add a Developer Certificate of Origin sign-off with `git commit -s`. The commit message must end with:

```
Signed-off-by: Your Name <you@example.com>
```

That line certifies [Developer Certificate of Origin 1.1](https://developercertificate.org/).

## Pull requests

- Run `make fmt`, `make lint`, and `make test` before pushing.
- Describe what changed and how you checked it.
- Keep the diff limited to the problem you are solving.
- For a bug, include the output of `oops doctor --json`.

## Adding an agent adapter

Adapters live in `internal/agents`, in the same package as the others. A subpackage would import-cycle with `internal/hook`.

Add a constructor next to `Claude()` and register it in `Agents()`:

```go
func Example() Agent {
    return Agent{
        Name:   "Example CLI",
        Binary: []string{"example"},
        Docs:   "https://example.com/docs/hooks",
        Config: func(home, project string, projectLevel bool) string {
            if projectLevel {
                return filepath.Join(project, ".example", "hooks.json")
            }
            return filepath.Join(home, ".example", "hooks.json")
        },
        SlashNote: "/oops",
        Install: func(exe, path string, write bool) (Change, bool, error) {
            // Read JSON, add only commands that contain "oops hook",
            // and call finishWrite. Do not rewrite unrelated keys' meaning.
            return Change{}, false, nil
        },
        Uninstall: uninstallFile,
    }
}
```

Hook commands are `oops hook <agent> <event>`. Map the agent's events onto `session`, `start`, `end`, `step`, or `slash` in `Parse` (`internal/agents/parse.go`). `start` opens a turn, `end` closes it, `step` opens one only if that session has no running turn, and `slash` answers `/oops` locally.

`Install` must leave a config that is already connected unchanged, and `Uninstall` must delete only entries whose command contains `oops hook`. Write a backup (`*.oops-backup-<pid>`) the same way `finishWrite` does. Add a unit test that installs into a temp home, checks the JSON, then uninstalls and checks that unrelated keys remain.

Document the upstream hook URL on the constructor. Do not invent a minimum tested version; say it is unrecorded until someone has actually run that version.
