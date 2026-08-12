set dotenv-load
set export
set windows-shell := ["powershell", "-NoProfile", "-Command"]

APP_NAME := "nimo"
bin_ext := if os() == "windows" { ".exe" } else { "" }

default:
    @just --list

build:
    go build -ldflags "-X github.com/nimling/nimo-api/internal.Version=$APP_VERSION" -o dist/{{APP_NAME}}{{bin_ext}} ./cmd

install: build install-tools
    cp dist/{{APP_NAME}}{{bin_ext}} $(go env GOPATH)/bin/

install-tools:
    go install github.com/nimling/sbump/cmd@latest
    mv "$(go env GOPATH)/bin/cmd" "$(go env GOPATH)/bin/sbump"

vet:
    go vet ./...

test:
    cd test/go && go test -v ./...

dev *args:
    go run ./cmd {{args}}

clean:
    rm -rf dist

deploy:
    sbump patch --env APP_VERSION --yaml ./action.yml@.inputs.nimo-version.default --push-version --auto --workflow
