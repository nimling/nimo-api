APP_NAME := "nimo"

build:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building {{APP_NAME}}..."
    VERSION=$(grep "APP_VERSION=" .env | cut -d'=' -f2)
    go build -ldflags "-X github.com/nimling/nimo-api/internal.Version=$VERSION" -o dist/{{APP_NAME}} ./cmd

run: build
    @echo "Running {{APP_NAME}}..."
    @dist/{{APP_NAME}}

install: build
    @echo "Installing {{APP_NAME}} to $(go env GOPATH)/bin..."
    @cp dist/{{APP_NAME}} $(go env GOPATH)/bin/

deploy:
    #!/usr/bin/env bash
    set -euo pipefail
    SBUMP_ENV_FILE=.env SBUMP_VERSION_VAR=APP_VERSION .github/scripts/sbump.sh patch --push-version
