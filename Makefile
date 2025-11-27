# Variables
GO_FILES := $(shell find . -name '*.go')
VERSION := $(shell git describe --tags --always --dirty)
APP_NAME := nimo

# Go related variables
GOBASE := $(shell pwd)
GOBIN := $(GOBASE)/dist

# Default target
all: build

# Build the Go application
build: $(GO_FILES)
	@echo "Building $(APP_NAME)..."
	@go build -o $(GOBIN)/$(APP_NAME) ./cmd

# Run the built application
run:
	@echo "Running $(APP_NAME)..."
	@$(GOBIN)/$(APP_NAME)

# Run the current code directly
dev:
	@echo "Running $(APP_NAME) in dev mode..."
	@go run ./cmd/main.go

# Run tests
test:
	@echo "Running tests..."
	@cd test/go && go test -v ./...

# Build and install to system
install: build
	@echo "Installing $(APP_NAME) to /usr/local/bin..."
	@cp $(GOBIN)/$(APP_NAME) /usr/local/bin/

# Deploy command - increment alpha version, commit and push
deploy:
	@echo "Starting deployment process..."

	# Read current version from .env
	$(eval CURRENT_VERSION := $(shell grep "APP_VERSION=" .env | cut -d'=' -f2))
	@echo "Current version: $(CURRENT_VERSION)"

	# Extract the alpha number and increment it
	$(eval ALPHA_NUM := $(shell echo $(CURRENT_VERSION) | grep -o 'alpha[0-9]*' | grep -o '[0-9]*'))
	$(eval NEW_ALPHA_NUM := $(shell echo $$(($(ALPHA_NUM) + 1))))
	$(eval NEW_VERSION := $(shell echo $(CURRENT_VERSION) | sed 's/alpha[0-9]*/alpha$(NEW_ALPHA_NUM)/'))
	$(eval PACKAGE_VERSION := $(shell echo $(NEW_VERSION) | sed 's/^v//'))
	@echo "New version: $(NEW_VERSION)"
	@echo "Package version: $(PACKAGE_VERSION)"

	# Update .env file with new version
	@sed -i.bak 's/APP_VERSION=.*/APP_VERSION=$(NEW_VERSION)/' .env && rm .env.bak
	@echo "Updated .env with new version (local only)"

	# Add and commit all changes
	@git add -A
	@git commit -m "Release $(NEW_VERSION)" || echo "No changes to commit"

	# Create and push git tag
	@git tag $(NEW_VERSION)
	@echo "Created git tag: $(NEW_VERSION)"

	# Push commits and tag
	@git push origin main
	@git push origin $(NEW_VERSION)
	@echo "Pushed commits and tag to origin"

	@echo "Deployment complete! Version bumped to $(NEW_VERSION)"

.PHONY: build run dev test install deploy all