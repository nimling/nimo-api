set dotenv-load
set export
set windows-shell := ["powershell", "-NoProfile", "-Command"]

APP_NAME := "nimo"
bin_ext := if os() == "windows" { ".exe" } else { "" }

default:
    @just --list

build:
    go build -ldflags "-X github.com/nimling/nimo-api/internal.Version=$APP_VERSION" -o dist/{{APP_NAME}}{{bin_ext}} ./cmd

install: build
    cp dist/{{APP_NAME}}{{bin_ext}} $(go env GOPATH)/bin/

vet:
    go vet ./...

test:
    cd test/go && go test -v ./...

dev *args:
    go run ./cmd {{args}}

clean:
    rm -rf dist

deploy:
    ../samna/sbump/sbump.sh patch --env APP_VERSION --push-version
