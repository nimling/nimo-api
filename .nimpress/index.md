---
layout: doc
title: Nimo
description: The OpenAPI toolkit that turns one specification into a bundled spec, documentation, gateway configuration and ready made client collections.
tags: nimo, openapi, postman, hurl, nginx, specification
lastUpdated: true
---

# Nimo

Nimo is the OpenAPI toolkit behind the API references in the Nimtech and Samna stack. Its CLI binary `nimo` reads a specification from disk and writes files to disk; nothing talks to a running API. The same release ships the binary and a GitHub Action, so a pipeline and a terminal run the same version.

**Repository**: [nimling/nimo-api](https://github.com/nimling/nimo-api)

## Command surface

| Command | Use |
|---|---|
| `generate` | Write a specification from Go Echo handler code through a local Ollama endpoint. |
| `convert` | Turn a specification into a bundled spec, VitePress pages, nginx configuration, a Postman collection and a hurl archive. |
| `merge` | Combine several specifications into one. |
| `sync` | Copy documentation files across repositories by pattern. |
| `skill` | Print the Claude skill or install it. |
| `completion` | Print the shell completion script. |

## What convert writes

`convert` takes one or more files, globs or directories. Each output has its own flag, and an output whose flag is left out is not written.

| Flag | Output |
|---|---|
| `-o`, `--output` | The specification, as `spec.json`. With `--inline` it is one self contained file. |
| `--docs` | VitePress markdown, one page per tag. |
| `--nginx-output` | One nginx `location` block per path, as `<input name>.conf.template`. |
| `--postman-output` | A Postman v2.1 collection, as `postman.json`. |
| `--hurl-output` | A hurl archive, as `hurl.zip`. |

```sh
nimo convert openapi.yml -o ./out --inline --postman-output ./out --hurl-output ./out
```

## Client collections

The Postman collection and the hurl archive are generated from the same specification, so they never drift from the reference.

1. The collection has one folder per tag and one request per operation, with the server as the `baseUrl` variable.

2. The archive holds `api.hurl`, one entry per operation in path order, and `variables.env` for `hurl --variables-file`.

3. A request body is the example of the request when it has one, and is otherwise built from the schema: its example, default, first enum value, format and type.

4. An OAuth2 client credentials flow becomes the OAuth2 settings of the collection and a token request at the top of `api.hurl`, which captures the token for the entries below. The client id, the client secret and the audience are variables the reader fills in. An API key and HTTP bearer or basic authentication are carried the same way.

5. Path parameters become variables, filled from their examples.

```sh
unzip hurl.zip -d hurl
hurl --variables-file hurl/variables.env hurl/api.hurl
```

## In a pipeline

```yaml
- uses: nimling/nimo-api@latest
  with:
    command: convert
    openapi-file: openapi.yml
    output-dir: out
    postman-output-dir: out
    hurl-output-dir: out
    inline: 'true'
```
