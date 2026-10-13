<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="brand/logo/wordmark-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="brand/logo/wordmark-light.svg">
    <img alt="oops" src="brand/logo/wordmark-light.svg" width="280">
  </picture>
</p>

# oops

编程 Agent 的 Ctrl+Z。

产品需求见 [docs/PRD.md](docs/PRD.md)，界面规范见 [docs/DESIGN.md](docs/DESIGN.md)。标志文件和用法见 [brand/BRAND.md](brand/BRAND.md)。

English: [README.md](README.md).

## 构建

需要 Go 1.27.2。

```sh
make build
```

产物是 `bin/oops`。版本、提交和构建日期通过 `-ldflags` 注入。
