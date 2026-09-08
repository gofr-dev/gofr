# Contributing to GoFr with an AI coding assistant

This file is for agents working **on the GoFr framework itself** — the code in
this repository. If you are building an application *with* GoFr, you want
[docs/AGENTS.md](docs/AGENTS.md) instead, published at
<https://gofr.dev/AGENTS.md>.

GoFr is an opinionated Go framework for production microservices. Apache 2.0,
Go 1.25+. Read [CONTRIBUTING.md](CONTRIBUTING.md) before your first change — the
rules below are the parts agents get wrong most often, not a replacement for it.

## Repository layout

| Path | What lives there |
| --- | --- |
| `pkg/gofr/` | The framework. Container, HTTP/gRPC servers, middleware, datasource clients, migrations, logging, metrics, tracing. |
| `pkg/gofr/datasource/` | One package per datasource (Postgres, Redis, Mongo, Cassandra, Kafka, …). Each is its own Go module with its own `go.mod`. |
| `examples/` | Runnable example services. These are also integration tests — they must build and pass. |
| `docs/` | The documentation site's content. Markdoc `page.md` files overlaid onto gofr-dev/website at build time. |
| `docs/Dockerfile` | Builds and serves gofr.dev. Changing it changes the live site. |

## Rules

1. **PRs target `development`, never `main`.** Feature branches merge to
   `development` first; `main` is release-only. A PR opened against `main` will
   be closed.
2. **Every behavioral change needs a test.** Table-driven, `testify` assertions,
   `go.uber.org/mock` for mocks. Do not hand-write mocks.
3. **Datasource packages are separate modules.** Adding a dependency to
   `pkg/gofr/datasource/redis` must not pull that dependency into the root
   module. Check `go.work` if you are unsure which module you are editing.
4. **Public API is a contract.** Changing an exported signature in `pkg/gofr`
   breaks every downstream service. Add, deprecate, then remove across releases —
   do not rename in place.
5. **Do not add a dependency to the root module casually.** GoFr's value is that
   `go get gofr.dev` pulls a bounded set. Justify any addition in the PR body.
6. **Documentation lives with the code.** A new feature that users can see needs
   a `docs/` page in the same PR, and a link added to `docs/navigation.js`.
7. **Run `golangci-lint` before pushing.** CI runs it and will reject the PR
   otherwise.
8. **Never commit secrets**, real connection strings, or `.env` files with
   values. `configs/.env` in examples holds local-only defaults.

## Before you open a PR

```bash
go build ./...
go test ./...          # add -race for anything touching goroutines
golangci-lint run
```

For a change under `pkg/gofr/datasource/<name>/`, run the module's own tests
from inside that directory — the root `go test ./...` does not cover it.

## Documentation and agent-facing files

The website is built by overlaying `docs/` onto the `gofr-dev/website` repo, so
some agent-facing files are split across the two repositories:

- `docs/AGENTS.md` → served at `/AGENTS.md`. Framework usage guidance.
- `docs/*/page.md` → the documentation pages, each also published as a Markdown
  twin (`/docs/quick-start/introduction.md`).
- `.claude/plugin.json` and `skills/` → agent plugin manifest and skills for this
  repository.
- Everything under `/.well-known/`, `llms.txt`, and `openapi.json` is generated
  or authored in `gofr-dev/website`, not here.

If you change a documentation page, the twin, the section index, and the
site-wide dump all regenerate automatically at build time. Do not hand-edit
generated output.

## Useful references

- Documentation: <https://gofr.dev/docs> (append `.md` to any page for Markdown)
- Machine-readable index: <https://gofr.dev/llms.txt>
- Everything in one file: <https://gofr.dev/llms-full.txt>
- Security policy: [SECURITY.md](SECURITY.md)
