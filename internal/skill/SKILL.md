---
name: nimo
description: Drive the nimo OpenAPI toolkit from the terminal. Use when generating an OpenAPI spec from Go Echo handlers, converting a spec into nginx location config or VitePress documentation, splitting or inlining a spec, merging several specs into one, or syncing documentation files across repos.
---

# nimo

`nimo` is an OpenAPI toolkit with four commands. `generate` writes a spec from Go Echo handler source, `convert` turns a spec into nginx config, VitePress markdown, and a written spec, `merge` folds several specs into one, and `sync` copies documentation files between projects by regexp. Every command reads files from disk and writes files to disk. Nothing talks to a running api.

## Boundaries

1. `convert` accepts `.yml` and `.yaml` input only. A path that names a directory is walked and every `.yml` and `.yaml` under it is processed. A path that matches nothing is the error `no matches found for pattern: <pattern>`, and a `.json` argument is silently skipped, so a convert that reports nothing was written is usually a json input.

2. `merge` reads json and yaml specs and writes one file. It refuses to overwrite an existing output with `output file <path> already exists (use --force to overwrite)`.

3. `generate` needs an Ollama compatible endpoint answering `POST /api/generate?format=json`. Without one every handler fails and the run still writes a spec, with the failures counted in `Completed with N errors`.

4. There is no config file and no environment discovery. Every default is a flag default or a named environment variable, listed per command below.

## Shape of every command

```
nimo <generate|convert|merge|sync> [files...] [flags]
```

`nimo --help` prints the banner and the command list, `nimo --version` prints the version compiled into the binary.

## convert

```
nimo convert [files...] [flags]
```

Takes one or more file paths, globs, or directories, and requires at least one argument.

| flag | default | what it does |
| --- | --- | --- |
| `--nginx-output` | empty | directory for the nginx configuration files |
| `-o`, `--output` | `$NIMO_OUTPUT` | directory the spec is written to |
| `--docs` | empty | directory for the VitePress markdown, overrides the output dir |
| `-d`, `--generate-docs` | false | write the VitePress markdown into the output directory |
| `-i`, `--index` | empty | path of a VitePress `index.md` whose features block is updated |
| `--file-prefix` | empty | prefix for the generated file names |
| `--common-prefix` | empty | url path prefix, and a folder level under the output directory |
| `--write-introduction` | false | also write the introduction page |
| `--merge-responses-inline` | false | merge `allOf` response definitions into single inline objects |
| `--inline-examples` | false | inline `#/components/examples/*` and drop the map |
| `--inline-responses` | false | inline `#/components/responses/*` and drop the map |
| `--inline-schemas` | false | inline `#/components/schemas/*` and drop the map, circular schemas keep their ref |
| `--inline-parameters` | false | inline `#/components/parameters/*` and drop the map |
| `--inline` | false | all four of the above at once |
| `--no-refs` | false | replace every remaining internal `$ref` with a deep copy, cycles keep the deepest ref, no-op without an inline flag |
| `-v`, `--verbose` | false | per iteration logs during ref resolution and inlining |
| `--verbose-write` | false | one bullet line per file written to disk |
| `--spec-dir` | empty | folder under the output directory that wraps the spec and its two folders |
| `--spec-file` | `spec.json` | file name of the top spec entry, `.json` is appended when missing |

The order inside one file is fixed: load, validate, normalize refs, `--merge-responses-inline`, parameters, examples, responses, schemas, `--no-refs`, nginx, spec, docs, index. Loading resolves every external `$ref` against the file that carries it and then checks that every internal ref points at something the document holds. Normalizing rewrites a file style ref such as `./schemas/Person.yml` into `#/components/schemas/Person` whenever the document defines a schema of that name.

### Where a converted file lands

1. The spec output directory is `--output`, else `--docs`, else the directory holding the input file.

2. With no inline flag the spec is split: `<output>/<common-prefix>/<spec-dir>/<spec-file>` next to an `operations/` and a `schemas/` folder, and every schema ref in the top file is rewritten to `./schemas/`. The run prints `Successfully wrote split spec to <dir>`.

3. With any inline flag the spec is one self contained file at `<output>/<common-prefix>/<file-prefix>spec.json`. `--spec-dir` and `--spec-file` do not apply to it.

4. `--nginx-output` writes `<nginx-output>/<input basename>.conf.template`, one nginx `location` block per path in the spec.

5. Docs go to `--docs`, else `--output`, else the input directory, under `<common-prefix>`, as `[tag].md` and `[tag].paths.js`, plus `<file-prefix>introduction.md` with `--write-introduction`.

### Recipes

```sh
nimo convert api.yml -d ./docs --write-introduction
nimo convert api.yml -o ./nginx --file-prefix api
nimo convert *.yml -d ./docs --common-prefix v1
nimo convert api.yml -o ./out --inline --no-refs
nimo convert ./specs -o ./out --spec-dir api --spec-file api.spec.json
```

## generate

```
nimo generate -m ./main.go -r ./README.md
```

Scans the one Go file named by `--main` for functions that return `error` and mention `echo.Context`, asks the ai endpoint about each one, and writes an OpenAPI 3.1.0 document whose description is the README content and whose components carry a `BearerAuth` and a `CookieAuth` security scheme.

| flag | env | default |
| --- | --- | --- |
| `-m`, `--main` | | required, path to the main.go file |
| `-r`, `--readme` | | required, path to the README.md file |
| `-a`, `--ai-endpoint` | `AI_ENDPOINT` | `http://localhost:11434` |
| `-c`, `--max-concurrent` | `MAX_CONCURRENT` | `5` |
| `-o`, `--output` | `NIMO_OUTPUT` then `OUTPUT_FILE` | `openapi.yaml` |
| `-f`, `--format` | `OUTPUT_FORMAT` | `yaml`, or `json` |

```sh
nimo generate -m ./main.go -r ./README.md -a http://localhost:11434 -o api.yaml
nimo generate -m ./main.go -r ./README.md -f json
```

Success prints `Successfully generated OpenAPI specification: <path>`. A per handler failure prints `Error processing <path>: <err>` and does not stop the run.

## merge

```
nimo merge [spec-files...] [flags]
```

Requires at least one spec file.

| flag | env | default |
| --- | --- | --- |
| `-o`, `--output` | `MERGE_OUTPUT` | `api.spec.json` |
| `-f`, `--force` | `FORCE_OVERWRITE` | false |
| `--title` | `API_TITLE` | empty |
| `--description` | `API_DESCRIPTION` | empty |
| `--api-version` | `API_VERSION` then `VERSION` | empty |
| `--contact-name` | `CONTACT_NAME` | empty |
| `--contact-email` | `CONTACT_EMAIL` | empty |
| `--format` | `OUTPUT_FORMAT` | `json`, or `yaml` |
| `--strategy` | `MERGE_STRATEGY` | `last`, or `first` |
| `--server` | `API_SERVER` | empty |
| `--text-format` | `TEXT_FORMAT` | `asis`, or `html`, or `markdown` |

`--strategy last` lets a later spec overwrite an operation or a component the earlier one defined, `first` keeps the earlier one. `--text-format` rewrites every description and summary in the merged document.

```sh
nimo merge spec1.json spec2.json spec3.json -o merged.json
nimo merge *.json --title "My API" --api-version v1.0.0 -o api.json --force
```

The run prints `✓ Merged N specs into <path>`, then the path count, then a report naming every overwritten operation, every component defined more than once, and every ref that resolves to nothing.

## sync

```
nimo sync -s mapping.json
```

`-s`, `--sync-map` is required and takes either the path of a json mapping file or the mapping json itself, recognised by a leading `{`. Keys are destinations, values are an ordered list of regexp patterns tried against the tree under the working directory, first match wins.

```json
{
  "output/guides/project/": [
    ".*docs/guide\\.md$",
    ".*docs/guide/index\\.md$"
  ],
  "output/api/reference/index.md": [
    ".*api/reference\\.md$"
  ]
}
```

A destination ending in `/` receives `index.md`. A pattern matching a directory copies the whole directory, and the directory must already hold a file named like the destination file. Every destination that matched nothing is collected and reported at the end rather than stopping the run.

## Failure reading

1. `no matches found for pattern: <p>` is a glob or a path that resolves to nothing. Quote a glob the shell would otherwise expand into a list the command then re-globs.

2. `failed to load OpenAPI specification: ...` is the file failing to parse, an external `$ref` that cannot be resolved, or `invalid spec <path>` naming an internal ref that points at nothing. External paths resolve relative to the referring file, not the working directory.

3. `validation error: ...` is the loaded spec failing document validation. Nothing on disk changed.

4. `output file <path> already exists (use --force to overwrite)` is `merge` refusing to clobber. Pass `--force` or point `-o` somewhere else.

5. `invalid strategy: <s> (must be 'first' or 'last')` and `invalid text-format: <f> (must be 'asis', 'html', or 'markdown')` are rejected before any file is read.

6. A convert that prints nothing wrote nothing: the argument was not a `.yml` or `.yaml` file, and no output flag was given.

## The skill and the completion

```sh
nimo skill get
nimo skill put
nimo skill put --project
nimo completion zsh
nimo completion --auto
nimo completion --auto --skill
```

`nimo skill put` writes `~/.claude/skills/nimo/SKILL.md`, and `--project` writes `.claude/skills/nimo/SKILL.md` under the working directory instead. `nimo completion --auto` detects the shell from `$SHELL`, writes a nimo owned file under `~/.config/nimo/completions`, and points the rc file at it inside a managed block.
