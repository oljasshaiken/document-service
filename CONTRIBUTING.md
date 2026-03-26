# Contributing

Thank you for your interest in improving this project. Behavior, API, and configuration are documented in [README.md](README.md).

## Development setup

- Install **Go** using the version in [go.mod](go.mod).
- For local runs that actually render PDFs (not required for tests), install Chrome or Chromium and set `CHROME_PATH` if it is not discovered automatically—see the README **Configuration** section.

## Checks before opening a pull request

These mirror [`.github/workflows/ci-cd.yml`](.github/workflows/ci-cd.yml):

```bash
gofmt -l src
test -z "$(gofmt -l src)"
```

Use `gofmt -w src` to fix formatting. Then:

```bash
go test ./...
go build ./src/cmd/document-service
```

The test suite mocks PDF generation, so you do not need Chromium to run `go test ./...`.

## Pull requests

- Keep changes focused on a single concern when possible.
- Add or update tests when you change behavior.
- Briefly describe what changed and why.

## Security

If you find a security vulnerability, please report it privately to the maintainers instead of opening a public issue.

Pull requests run a Trivy filesystem scan (report-only) and the repository may use additional static analysis (for example CodeQL); fixing findings is appreciated when they are actionable.
