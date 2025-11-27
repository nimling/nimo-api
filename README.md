# Nimo

A powerful CLI tool and GitHub Action for working with OpenAPI specifications. Generate specs from code, convert to various formats, merge multiple specs, and sync documentation files.

## Features

- **Generate**: Create OpenAPI specs from Go Echo handler code using AI
- **Convert**: Transform OpenAPI specs into Nginx configurations and VitePress documentation
- **Merge**: Combine multiple OpenAPI specifications into one unified spec
- **Sync**: Synchronize documentation files across repositories using pattern-based mapping
- **External reference resolution**: Automatically resolves `$ref` references to external files
- **Batch processing**: Process multiple specifications at once
- **GitHub Action support**: Available as a GitHub Action for CI/CD pipelines
- **Strict validation**: Ensures OpenAPI specifications meet documentation standards

## OpenAPI Compliance

For detailed information about OpenAPI specification requirements, validation rules, and best practices, see [OPENAPI.md](./OPENAPI.md).

## Prerequisites

- Go 1.23 or later
- Git installed
- For `generate` command: Ollama running locally (default: http://localhost:11434)

## Installation

### Using Go Install

```bash
go install github.com/nimling/nimo-api@latest
```

For a specific version:

```bash
go install github.com/nimling/nimo-api@v2.0.0
```

### From Source

```bash
git clone git@github.com:nimling/nimo-api.git
cd nimo-api
just build
just install
```

### As GitHub Action

Add to your workflow:

```yaml
- uses: nimling/nimo-api@v2
  with:
    command: 'convert'
    openapi-file: './api.yml'
    docs-dir: './docs/api'
```

## Usage

The tool provides four main commands: `generate`, `convert`, `merge`, and `sync`.

### `generate` - Generate OpenAPI from Go Code

Generate OpenAPI specifications by analyzing Go Echo handler code using AI.

```bash
nimo generate -m <main.go> -r <README.md> [flags]
```

**Flags:**
- `-m, --main` - Path to main.go file (required)
- `-r, --readme` - Path to README.md file (required)
- `-a, --ai-endpoint` - AI API endpoint (default: `http://localhost:11434`)
- `-c, --max-concurrent` - Maximum concurrent AI API calls (default: `5`)
- `-o, --output` - Output file path (default: `openapi.yaml`)
- `-f, --format` - Output format: yaml or json (default: `yaml`)

**Examples:**
```bash
# Generate spec from Go handlers
nimo generate -m ./main.go -r ./README.md

# Specify AI endpoint and output format
nimo generate -m ./main.go -r ./README.md -a http://localhost:11434 -f json

# Use environment variables
export AI_ENDPOINT=http://localhost:11434
nimo generate -m ./main.go -r ./README.md
```

---

### `convert` - Convert OpenAPI to Docs/Config

Transform OpenAPI specifications into Nginx configurations and VitePress documentation.

```bash
nimo convert [input-files...] [flags]
```

#### Flags

| Flag | Short | Description | Example |
|------|-------|-------------|---------|
| `--output` | `-o` | Output directory for Nginx configuration files | `-o ./nginx/` |
| `--docs` | `-d` | Output directory for VitePress API documentation | `-d ./docs/api/` |
| `--index` | `-i` | Path to generate/update VitePress index.md with features | `-i ./docs/index.md` |
| `--file-prefix` | | Prefix for generated file names | `--file-prefix api-` |
| `--common-prefix` | | URL path prefix for VitePress documentation links | `--common-prefix /api/v1` |
| `--write-introduction` | | Generate introduction page for API documentation | `--write-introduction` |
| `--merge-responses-inline` | | Merge allOf response definitions into single inline objects | `--merge-responses-inline` |

#### Examples

```bash
# Convert single file to Nginx config
nimo convert api.yaml -o ./nginx/

# Generate VitePress documentation only
nimo convert api.yaml -d ./docs/api/

# Generate both with all options
nimo convert api.yaml \
  -o ./nginx/ \
  -d ./docs/api/ \
  -i ./docs/index.md \
  --file-prefix myapi \
  --common-prefix /api/v1 \
  --write-introduction \
  --merge-responses-inline
```

---

### `merge` - Merge Multiple OpenAPI Specs

Combine multiple OpenAPI specification files into a single unified spec.

```bash
nimo merge [spec-files...] [flags]
```

**Flags:**
- `-o, --output` - Output file path (default: `api.spec.json`)
- `-f, --force` - Force overwrite existing file
- `--title` - Override API title
- `--description` - Override API description
- `--version` - Override API version
- `--contact-name` - Override contact name
- `--contact-email` - Override contact email
- `--format` - Output format: yaml or json (default: `json`)

**Examples:**
```bash
# Merge multiple specs
nimo merge spec1.json spec2.json spec3.json -o merged.json

# Override metadata
nimo merge *.json --title "My API" --version "v1.0.0" -o api.json

# Use environment variables (thon-api pattern)
export VERSION=v1.0.0
export API_TITLE="Thon+ API"
nimo merge spec1.json spec2.json spec3.json -f
```

---

### `sync` - Synchronize Documentation

Synchronize documentation files between directories using pattern-based mapping. Supports both individual file copying with renaming and full directory copying when target files exist.

```bash
nimo sync -s <sync-map>
```

#### Flags

| Flag | Short | Description | Required |
|------|-------|-------------|----------|
| `--sync-map` | `-s` | JSON mapping file or inline JSON for sync command | Yes |

#### Mapping File Format

Create a JSON file or inline JSON with your synchronization rules. The destination must always specify a target filename:

```json
{
  "output/guides/myproject/index.md": [
    ".*docs/guide\\.md$",
    ".*docs/guide$"
  ],
  "output/api/myproject/index.md": [
    ".*docs/api\\.md$",
    ".*docs/api$"
  ],
  "output/tutorials/getting-started.md": [
    ".*tutorials/getting-started\\.md$",
    ".*docs/getting-started\\.md$"
  ]
}
```

**Behavior:**
- If source matches a **file**: Copies and renames to the destination filename
- If source matches a **directory**: Checks for a file with the destination filename inside, then copies the entire directory contents

#### Examples

```bash
# Sync documentation using mapping file
nimo sync -s ./sync-config.json

# Typical CI/CD usage
nimo sync --sync-map ./docs/mapping.json
```

---

## GitHub Action Usage

The tool is available as a GitHub Action supporting all four commands.

### Basic Usage

```yaml
- uses: nimling/nimo-api@v2
  with:
    command: 'convert'
    openapi-file: './api.yml'
    docs-dir: './docs/api'
```

### Examples

Generate OpenAPI from Go code:
```yaml
- uses: nimling/nimo-api@v2
  with:
    command: 'generate'
    main: './main.go'
    readme: './README.md'
    output: 'openapi.yaml'
```

Convert to documentation:
```yaml
- uses: nimling/nimo-api@v2
  with:
    command: 'convert'
    openapi-file: 'api.yaml'
    docs-dir: './docs/api'
    common-prefix: '/api/v1'
```

Merge multiple specs:
```yaml
- uses: nimling/nimo-api@v2
  with:
    command: 'merge'
    specs: 'spec1.json spec2.json spec3.json'
    output: 'merged.json'
    title: 'My API'
    version: 'v1.0.0'
```

Sync documentation:
```yaml
- uses: nimling/nimo-api@v2
  with:
    command: 'sync'
    sync-map: './sync-config.json'
```

See [action.yml](./action.yml) for all available inputs.

### Output Formats

The converter generates:

#### Nginx Configuration (.conf.template)
- Location blocks with path patterns
- Method restrictions (GET, POST, PUT, DELETE)
- Upstream proxy configurations
- Security headers and CORS settings

#### VitePress Documentation
- Markdown files for each endpoint
- Interactive API documentation
- Request/response examples
- Schema definitions
- Navigation structure

## Validation

The converter enforces strict validation to ensure high-quality API documentation:

- **Required fields**: All paths must have `summary` and `description`
- **Operation IDs**: Every operation must have a unique `operationId`
- **Path format**: Paths must start with `/` and have valid segments
- **External references**: All `$ref` references must resolve successfully

For complete validation rules and OpenAPI compliance guidelines, see [OPENAPI.md](./OPENAPI.md).

## Development

### Prerequisites
- Go 1.23+
- [just](https://github.com/casey/just) command runner
- Ollama (for testing generate command)

### Commands

```bash
# Build the binary
just build

# Install to $GOPATH/bin
just install

# Run in development mode
just dev

# Test all commands
just test-all

# Clean build artifacts
just clean
```

### Project Structure

```
nimo-api/
├── cmd/main.go              # CLI entry point
├── internal/
│   ├── generate.go          # Generate command
│   ├── convert.go           # Convert command
│   ├── merge.go             # Merge command
│   ├── sync.go              # Sync command
│   └── utils.go             # Banner and version utils
├── pkg/
│   ├── ai/                  # AI client (Ollama)
│   ├── parser/              # Go Echo handler parser
│   ├── merger/              # OpenAPI spec merger
│   └── converter/           # Spec conversion logic
├── action.yml               # GitHub Action definition
├── justfile                 # Build commands
└── .env                     # Version configuration
```

### Version Management

Version is read from `.env` file:
```bash
APP_VERSION=v1.0.0-alpha6
```

Update version and deploy:
```bash
just deploy  # Increments alpha, commits, tags, and pushes
```

## Troubleshooting

### Module Not Found Errors
If you get "module not found" errors, configure git:
```bash
git config --global url."git@github.com:nimling/".insteadOf "github.com/nimling/"
```

### Binary Not Found
Ensure `$GOPATH/bin` is in your PATH:
```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### AI Generation Issues
For the `generate` command, ensure Ollama is running:
```bash
# Check if Ollama is running
curl http://localhost:11434/api/tags

# Start Ollama if needed
ollama serve
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT
