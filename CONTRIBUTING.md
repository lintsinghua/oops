# Contributing

Thanks for helping with oops. Product behavior is still being specified, so
please keep changes to the foundation (build, CLI plumbing, tests, and project
tooling) unless a requirement has already been agreed.

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

## Pull requests

- Run `make fmt`, `make lint`, and `make test` before pushing.
- Describe what changed and how you checked it.
- Keep the diff limited to the problem you are solving.
