APP_NAME := "nimo"
VERSION := `git describe --tags --always --dirty 2>/dev/null || echo "v2.0.0"`
GOBIN := justfile_directory() / "dist"

# List all available commands
default:
    @just --list

# Build the application
build:
    @echo "Building {{APP_NAME}}..."
    @go build -o {{GOBIN}}/{{APP_NAME}} ./cmd

# Run the built application
run: build
    @echo "Running {{APP_NAME}}..."
    @{{GOBIN}}/{{APP_NAME}}

# Run in development mode
dev:
    @echo "Running {{APP_NAME}} in dev mode..."
    @go run ./cmd/main.go

# Run tests
test:
    @echo "Running tests..."
    @cd test/go && go test -v ./...

# Build and install to Go bin directory
install: build
    @echo "Installing {{APP_NAME}} to $(go env GOPATH)/bin..."
    @cp {{GOBIN}}/{{APP_NAME}} $(go env GOPATH)/bin/

# Build and install to /usr/local/bin
install-system: build
    @echo "Installing {{APP_NAME}} to /usr/local/bin..."
    @cp {{GOBIN}}/{{APP_NAME}} /usr/local/bin/

# Clean build artifacts
clean:
    @echo "Cleaning build artifacts..."
    @rm -rf {{GOBIN}}

# Run all commands help to verify
test-all:
    @echo "Testing all commands..."
    @{{GOBIN}}/{{APP_NAME}} --help
    @{{GOBIN}}/{{APP_NAME}} generate --help
    @{{GOBIN}}/{{APP_NAME}} convert --help
    @{{GOBIN}}/{{APP_NAME}} merge --help
    @{{GOBIN}}/{{APP_NAME}} sync --help

# Deploy - increment alpha version and push
deploy:
    #!/usr/bin/env bash
    set -euo pipefail

    echo "Starting deployment process..."

    # Read current version from .env
    CURRENT_VERSION=$(grep "APP_VERSION=" .env | cut -d'=' -f2)
    echo "Current version: $CURRENT_VERSION"

    # Extract the alpha number and increment it
    ALPHA_NUM=$(echo $CURRENT_VERSION | grep -o 'alpha[0-9]*' | grep -o '[0-9]*')
    NEW_ALPHA_NUM=$((ALPHA_NUM + 1))
    NEW_VERSION=$(echo $CURRENT_VERSION | sed "s/alpha[0-9]*/alpha$NEW_ALPHA_NUM/")
    echo "New version: $NEW_VERSION"

    # Update .env file
    sed -i.bak "s/APP_VERSION=.*/APP_VERSION=$NEW_VERSION/" .env && rm .env.bak
    echo "Updated .env with new version"

    # Add and commit
    git add -A
    git commit -m "Release $NEW_VERSION" || echo "No changes to commit"

    # Create and push tag
    git tag $NEW_VERSION
    echo "Created git tag: $NEW_VERSION"

    git push origin main
    git push origin $NEW_VERSION
    echo "Pushed commits and tag to origin"

    echo "Deployment complete! Version bumped to $NEW_VERSION"
