---
name: deploy
description: Build, test, commit, push, and tag a new version of nimo so consumers can pull the updated binary and Go action.
---

# deploy

Ship a new version of `nimo` (the OpenAPI toolkit at `github.com/nimling/nimo-api`). The repo publishes two artefacts that share one tag: the Go module consumed via `go install github.com/nimling/nimo-api/cmd@vX.Y.Z` and the reusable GitHub Action declared in `action.yml`. A deploy that ships a Go change without the action.yml version bump is a deploy that breaks consumers.

When the user says "deploy" in this repo, execute these steps in order. Stop on the first failure and surface the error.

## 1. Write or update Go tests for meaningful parts of the change

Every piece of new behaviour in `converter/`, `internal/`, `pkg/`, or `cmd/` that has an observable contract needs a test under `test/go/`. Look at the diff with `git diff` and `git status`, identify the meaningful surface, and either extend an existing test file under `test/go/` or add one. Trivial moves and rename only diffs do not need a new test. Anything that changes a return value, a CLI flag, a file written to disk, a struct field, an exported function shape, or a normalization rule MUST be covered.

The test must assert the observable contract, not the internal call graph. Hit the exported function or run the CLI against a fixture under `test/examples/`, then assert on the output file content or returned error.

## 2. Run the Go tests

```
just test
```

Runs `cd test/go && go test -v ./...`. Failure: report the failing test names and stop. Do not commit, do not bump.

## 3. Build the Go binary

```
just build
```

`go build -o dist/nimo ./cmd`. Failure: report the build output and stop.

## 4. Sanity check the CLI

```
./dist/nimo --help
./dist/nimo convert --help
```

The two help screens must print without panic. Optional sanity convert against `test/examples/` if the change touched `converter/`:

```
./dist/nimo convert test/examples/<example>.yml -o /tmp/nimo-sanity
```

The file at `/tmp/nimo-sanity/spec.json` must parse as JSON and contain the changed surface (e.g. if the change normalizes external `$ref` strings, grep the output and confirm no file-style refs remain). Failure: report and stop.

## 5. Commit and push

Stage every changed file relevant to the release. Write a single sentence describing the user visible change. Push to `origin/main`.

```
git add -A
git commit -m "<one sentence describing the change>"
git push
```

If the working tree has nothing to commit after the build outputs are excluded by `.gitignore`, skip the commit but still push so the bump in the next step pushes a clean ref.

## 6. Bump and tag

```
just deploy
```

Calls `sbump patch --env APP_VERSION --yaml ./action.yml@.inputs.nimo-version.default --push-version --auto --workflow`. Patch bump, tag write, tag push, workflow track. Two side effects:

1. `APP_VERSION` in `.env` (or wherever sbump points) advances. The `just build` recipe embeds `APP_VERSION` into the binary via `-ldflags`.

2. `action.yml`'s `inputs.nimo-version.default` advances in lockstep so reusable workflow consumers pick up the new binary without changing their workflow.

For minor or major releases, run the same `sbump` line with `minor` or `major` instead of `patch`, on the user's word.

## Never

1. Skip steps 1, 2, 3, or 4. A missing test, a failing test, a failing Go build, or a panicking CLI must block the deploy.

2. Edit `APP_VERSION` in `.env` or `action.yml`'s `nimo-version.default` by hand. `sbump` owns both fields.

3. Use `--no-verify` or `--no-gpg-sign`.

4. Cut a release without confirming the binary actually runs the change. Help-screen output is not enough when the diff touches `converter/` — run a sanity convert against `test/examples/`.

5. Ship a change to `converter/` (which spec-bundling consumers depend on) without bumping `action.yml`'s `nimo-version.default`. The reusable workflow path and the Go binary path go out in the same tag or not at all.
