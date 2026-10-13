<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="brand/logo/wordmark-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="brand/logo/wordmark-light.svg">
    <img alt="oops" src="brand/logo/wordmark-light.svg" width="280">
  </picture>
</p>

# oops

Ctrl+Z for coding agents.

The product requirements are in [docs/PRD.md](docs/PRD.md). The interface spec is in [docs/DESIGN.md](docs/DESIGN.md). Logo files and where each one is used are in [brand/BRAND.md](brand/BRAND.md).

中文说明见 [README.zh-CN.md](README.zh-CN.md)。

## Build

Go 1.27.2 is required.

```sh
make build
```

This writes `bin/oops`. Version, commit, and build date are injected with `-ldflags`.
