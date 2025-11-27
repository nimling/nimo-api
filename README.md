# Nimo

A comprehensive CLI tool and GitHub Action for complete OpenAPI workflow automation. From code to documentation, Nimo handles your entire API documentation lifecycle.

## Why Nimo?

Nimo is the only tool you need for OpenAPI documentation workflows. It combines four essential capabilities:

### 🤖 Generate - AI-Powered Spec Creation
Extract OpenAPI specifications directly from your Go Echo handlers using AI (DeepSeek-r1). No manual YAML writing - let AI analyze your code and generate comprehensive API documentation automatically.

### 🔄 Convert - Multi-Format Transformation
Transform OpenAPI specs into production-ready outputs:
- **Nginx configurations** for API gateway routing
- **VitePress documentation** with interactive API references
- Automatic external reference resolution
- Batch processing support

### 🔗 Merge - Multi-Service Aggregation
Combine OpenAPI specs from multiple microservices into a single unified API documentation. Perfect for:
- Monorepo architectures
- Multi-service platforms
- API gateway documentation
- Environment variable-driven metadata

### 📦 Sync - Documentation Distribution
Synchronize documentation files across repositories using regex pattern matching. Ideal for:
- Centralizing docs from multiple repos
- Maintaining documentation portals
- Keeping API references in sync

## Key Features

- ✅ **Zero manual YAML** - Generate specs from code
- ✅ **Production-ready outputs** - Nginx configs, VitePress docs
- ✅ **Microservice-friendly** - Merge multiple services
- ✅ **CI/CD integration** - GitHub Action support
- ✅ **Environment-aware** - Full env variable support
- ✅ **Type-safe** - Leverages kin-openapi for OpenAPI 3.1
- ✅ **Fast** - Concurrent processing with rate limiting

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

Nimo is available as a GitHub Action with full support for all commands. The action dynamically installs the tool and runs your specified command.

### Quick Start

```yaml
- uses: nimling/nimo-api@v2
  with:
    command: 'generate'  # or 'convert', 'merge', 'sync'
    # command-specific inputs below
```

### Complete Examples

#### 1. Generate OpenAPI from Go Code

```yaml
name: Generate API Spec
on:
  push:
    paths: ['**/*.go']

jobs:
  generate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Generate OpenAPI Spec
        uses: nimling/nimo-api@v2
        with:
          command: 'generate'
          main: './cmd/api/main.go'
          readme: './README.md'
          output: 'openapi.yaml'
          format: 'yaml'
```

#### 2. Convert Spec to Documentation

```yaml
name: Build API Documentation
on:
  push:
    paths: ['api.yaml']

jobs:
  convert:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Convert to VitePress Docs
        uses: nimling/nimo-api@v2
        with:
          command: 'convert'
          openapi-file: './api.yaml'
          docs-dir: './docs/api'
          common-prefix: '/api/v1'
          write-introduction: 'true'
```

#### 3. Merge Multiple Service Specs

```yaml
name: Merge Service Specs
on:
  push:
    paths: ['services/**/api.json']

jobs:
  merge:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Merge Microservice Specs
        uses: nimling/nimo-api@v2
        with:
          command: 'merge'
          specs: 'services/auth/api.json services/user/api.json services/payment/api.json'
          output: 'platform-api.json'
          title: 'Platform API'
          version: ${{ github.ref_name }}
          force: 'true'
```

#### 4. Sync Documentation Across Repos

```yaml
name: Sync Documentation
on:
  workflow_dispatch:

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Sync Docs to Portal
        uses: nimling/nimo-api@v2
        with:
          command: 'sync'
          sync-map: './docs/sync-map.json'
```

#### 5. Complete CI/CD Pipeline

Full workflow combining all commands:

```yaml
name: Complete API Documentation Pipeline

on:
  push:
    branches: [main]
    paths:
      - 'services/**/*.go'
      - 'api/**/*.yaml'

jobs:
  build-unified-docs:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      # Step 1: Generate specs from each service
      - name: Generate Auth Service Spec
        uses: nimling/nimo-api@v2
        with:
          command: 'generate'
          main: './services/auth/main.go'
          readme: './services/auth/README.md'
          output: 'services/auth/api.json'
          format: 'json'

      - name: Generate User Service Spec
        uses: nimling/nimo-api@v2
        with:
          command: 'generate'
          main: './services/user/main.go'
          readme: './services/user/README.md'
          output: 'services/user/api.json'
          format: 'json'

      # Step 2: Merge all service specs
      - name: Merge Service Specs
        uses: nimling/nimo-api@v2
        with:
          command: 'merge'
          specs: 'services/auth/api.json services/user/api.json'
          output: 'platform-api.json'
          title: 'Platform API'
          description: 'Unified API for all platform services'
          version: ${{ github.ref_name }}
          contact-name: 'API Team'
          contact-email: 'api@company.com'
          force: 'true'

      # Step 3: Convert to documentation
      - name: Convert to VitePress
        uses: nimling/nimo-api@v2
        with:
          command: 'convert'
          openapi-file: 'platform-api.json'
          docs-dir: './docs/api'
          common-prefix: '/api'
          write-introduction: 'true'
          merge-responses: 'true'

      # Step 4: Sync to documentation portal
      - name: Sync to Docs Portal
        uses: nimling/nimo-api@v2
        with:
          command: 'sync'
          sync-map: './docs/sync-map.json'

      # Step 5: Commit generated docs
      - name: Commit Documentation
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add docs/ platform-api.json
          git commit -m "docs: update API documentation [skip ci]" || echo "No changes"
          git push
```

### Environment Variables

All commands support environment variables for default values:

```yaml
env:
  # Generate command
  AI_ENDPOINT: 'http://ollama:11434'
  OUTPUT_FILE: 'api.yaml'
  OUTPUT_FORMAT: 'yaml'

  # Merge command
  API_TITLE: 'My API'
  API_VERSION: ${{ github.ref_name }}
  CONTACT_NAME: 'API Team'
  CONTACT_EMAIL: 'api@example.com'
  FORCE_OVERWRITE: 'true'
```

### Action Inputs Reference

All available inputs per command are documented in [action.yml](./action.yml).

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
