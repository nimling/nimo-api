# nimo-api

`nimo`, a Go cli and GitHub Action for OpenAPI specifications: `generate` writes a spec from Echo handler code through a local Ollama endpoint, `convert` turns a spec into nginx configuration and VitePress pages, `merge` combines several specs, `sync` copies documentation files across repositories by pattern. The cli is `cmd/` and `internal/`, with its skill embedded from `internal/skill`; the converter is `converter/`, the parser, merger and ai clients `pkg/`, the Go tests and example specs `test/`. `action.yml` declares the action and pins the cli version it installs, and one tag ships the binary through the `Build and Publish` workflow and the action together. `OPENAPI.md` holds the validation rules a spec must meet.

## Commands

1. `just build`: the binary into `dist/nimo` with `APP_VERSION` stamped in.
2. `just install`: build, copy the binary onto `GOPATH/bin` and install `sbump`.
3. `just dev <args>`: `go run ./cmd`.
4. `just test`: the Go tests in `test/go`. `just vet`, `just clean`.
5. `just deploy`: sbump patch bump on `APP_VERSION` and on `action.yml` `inputs.nimo-version.default` together, tag and push, following the workflow.

## Conventions

1. The Go binary and the action go out in one tag or not at all; `action.yml` moves with `APP_VERSION` through the recipe, never by hand.
2. Every change with an observable contract, a flag, a file written, a normalization rule, gets a test under `test/go/` against a fixture in `test/examples/`.
3. `generate` reaches a model only through the Ollama endpoint it is given; no api key lives in the tool.
4. Code, files, git, stack and tools rules are `~/.claude/rules/`; the cli formula is the user level `cli` skill. Neither is restated here.

## Rules

None of its own.

## Skills

1. `.claude/skills/deploy`: test, build, sanity check the cli, commit, push, then the bump.
2. `internal/skill`: the cli's own skill, embedded in the binary.
3. User level, invoked by name: `cli`, `git`, `github`, `review`.
