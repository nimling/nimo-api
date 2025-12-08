#!/bin/bash

SBUMP_ENV_FILE=${SBUMP_ENV_FILE:-.env}
SBUMP_VERSION_VAR=${SBUMP_VERSION_VAR:-VERSION}
SBUMP_OPENAPI_FILE=${SBUMP_OPENAPI_FILE:-""}
SBUMP_PACKAGE_JSON=${SBUMP_PACKAGE_JSON:-""}

function show_help() {
    cat << "EOF"


 ███████╗██████╗ ██╗   ██╗███╗   ███╗██████╗
 ██╔════╝██╔══██╗██║   ██║████╗ ████║██╔══██╗
 ███████╗██████╔╝██║   ██║██╔████╔██║██████╔╝
 ╚════██║██╔══██╗██║   ██║██║╚██╔╝██║██╔═══╝
 ███████║██████╔╝╚██████╔╝██║ ╚═╝ ██║██║
 ╚══════╝╚═════╝  ╚═════╝ ╚═╝     ╚═╝╚═╝
             SBump it til it's hot!
                 Samna© 2025

EOF
    echo "Usage: sbump [OPTIONS] [major|minor|patch|commit]"
    echo ""
    echo "A simple script to bump semantic version numbers in a .env file"
    echo ""
    echo "Options:"
    echo "  --help                  Show this help message and exit"
    echo "  --env-file=FILE         Path to .env file (default: .env)"
    echo "  --version-var=NAME      Name of version variable (default: VERSION)"
    echo "  --openapi-file=FILE     Path to OpenAPI file to update (optional)"
    echo "  --package-json=FILE     Path to package.json file to update (optional)"
    echo "  --push-version          Commit changes, create a git tag, and push to remote"
    echo ""
    echo "Commands:"
    echo "  major                   Bump major version (x.0.0)"
    echo "  minor                   Bump minor version (0.x.0)"
    echo "  patch                   Bump patch version (0.0.x)"
    echo "  commit                  Commit changes and create a git tag without bumping version"
    echo ""
    echo "Environment variables:"
    echo "  SBUMP_ENV_FILE       Path to .env file (default: .env)"
    echo "  SBUMP_VERSION_VAR    Name of version variable (default: VERSION)"
    echo "  SBUMP_OPENAPI_FILE   Path to OpenAPI file to update (optional)"
    echo "  SBUMP_PACKAGE_JSON   Path to package.json file to update (optional)"
    echo ""
    echo "Examples:"
    echo "  sbump patch                     # Bump patch version"
    echo "  sbump minor --push-version      # Bump minor version, commit, tag and push"
    echo "  sbump commit                    # Commit current version without bumping"
    exit 0
}

function bump_version() {
    local current_version=$1
    local bump_type=$2

    local major=$(echo $current_version | sed -E 's/v([0-9]+)\.([0-9]+)\.([0-9]+)(-[a-zA-Z]+([0-9]+))?/\1/')
    local minor=$(echo $current_version | sed -E 's/v([0-9]+)\.([0-9]+)\.([0-9]+)(-[a-zA-Z]+([0-9]+))?/\2/')
    local patch=$(echo $current_version | sed -E 's/v([0-9]+)\.([0-9]+)\.([0-9]+)(-[a-zA-Z]+([0-9]+))?/\3/')
    local suffix=$(echo $current_version | sed -E 's/v([0-9]+)\.([0-9]+)\.([0-9]+)(-[a-zA-Z]+([0-9]+))?/\4/')
    local suffix_num=$(echo $current_version | sed -E 's/v([0-9]+)\.([0-9]+)\.([0-9]+)(-[a-zA-Z]+([0-9]+))?/\5/')

    if [ -z "$suffix_num" ] && [ ! -z "$suffix" ]; then
        suffix_num=0
    fi

    case "$bump_type" in
        "major")
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        "minor")
            minor=$((minor + 1))
            patch=0
            ;;
        "patch")
            if [ -z "$suffix" ]; then
                patch=$((patch + 1))
            else
                suffix_num=$((suffix_num + 1))
            fi
            ;;
        *)
            echo "Invalid bump type. Use 'major', 'minor', or 'patch'"
            show_help
            ;;
    esac

    if [ -z "$suffix" ]; then
        echo "v$major.$minor.$patch"
    else
        local suffix_prefix=$(echo $suffix | sed -E 's/(-[a-zA-Z]+)([0-9]+)?/\1/')
        echo "v$major.$minor.$patch$suffix_prefix$suffix_num"
    fi
}

function update_env_file() {
    local new_version=$1
    sed -i.bak "s/^$SBUMP_VERSION_VAR=.*/$SBUMP_VERSION_VAR=$new_version/" "$SBUMP_ENV_FILE" && rm -f "$SBUMP_ENV_FILE.bak"
}

function update_openapi_file() {
    local new_version=$1
    local file=$2

    if [ -z "$file" ]; then
        return
    fi

    if [[ "$file" == *.json ]]; then
        sed -i.bak "s/\"version\":\s*\"[^\"]*\"/\"version\": \"$new_version\"/" "$file" && rm -f "$file.bak"
    elif [[ "$file" == *.yml ]] || [[ "$file" == *.yaml ]]; then
        if command -v yq &> /dev/null; then
            yq -i '.info.version = "'"$new_version"'"' "$file"
        else
            echo "yq not found, falling back to sed"
            local line_num=$(grep -n "^\s*version:" "$file" | cut -d: -f1)
            if [ -z "$line_num" ]; then
                line_num=$(grep -n -A2 "^info:" "$file" | grep "version:" | cut -d: -f1)
            fi

            if [ -n "$line_num" ]; then
                sed -i.bak "${line_num}s/.*version:.*/  version: $new_version/" "$file" && rm -f "$file.bak"
            else
                echo "Version field not found in OpenAPI file"
                show_help
            fi
        fi
    else
        echo "Unsupported OpenAPI file format. Use .json, .yml, or .yaml"
        show_help
    fi
}

function update_package_json() {
    local new_version=$1
    local file=$2

    if [ -z "$file" ]; then
        return
    fi

    if [ ! -f "$file" ]; then
        echo "Package.json file $file not found"
        return 1
    fi

    local version_without_v=$(echo "$new_version" | sed 's/^v//')
    sed -i.bak "s/\"version\":\s*\"[^\"]*\"/\"version\": \"$version_without_v\"/" "$file" && rm -f "$file.bak"
}

function commit_and_push() {
    local version=$1
    local push=$2

    if [ -z "$version" ]; then
        echo "No version provided to commit"
        exit 1
    fi

    git add "$SBUMP_ENV_FILE"

    if [ ! -z "$SBUMP_OPENAPI_FILE" ] && [ -f "$SBUMP_OPENAPI_FILE" ]; then
        git add "$SBUMP_OPENAPI_FILE"
    fi

    if [ ! -z "$SBUMP_PACKAGE_JSON" ] && [ -f "$SBUMP_PACKAGE_JSON" ]; then
        git add "$SBUMP_PACKAGE_JSON"
    fi

    git commit -m "Bump version to $version"
    git tag "$version"

    if [ "$push" = "true" ]; then
        git push && git push --tags
    fi

    echo "Created git tag: $version"
    if [ "$push" = "true" ]; then
        echo "Pushed changes and tags"
    fi
}

# Parse command line arguments
BUMP_TYPE=""
PUSH_VERSION="false"
DO_COMMIT="false"

for arg in "$@"; do
    case "$arg" in
        "--help")
            show_help
            ;;
        "--push-version")
            PUSH_VERSION="true"
            ;;
        "commit")
            DO_COMMIT="true"
            ;;
        --env-file=*)
            SBUMP_ENV_FILE="${arg#*=}"
            ;;
        --version-var=*)
            SBUMP_VERSION_VAR="${arg#*=}"
            ;;
        --openapi-file=*)
            SBUMP_OPENAPI_FILE="${arg#*=}"
            ;;
        --package-json=*)
            SBUMP_PACKAGE_JSON="${arg#*=}"
            ;;
        "major"|"minor"|"patch")
            BUMP_TYPE="$arg"
            ;;
        *)
            ;;
    esac
done

# Validate input
if [ ! -f "$SBUMP_ENV_FILE" ]; then
    echo "Environment file $SBUMP_ENV_FILE not found"
    show_help
fi

# Source environment variables
source "$SBUMP_ENV_FILE"

# Main execution logic
if [ "$DO_COMMIT" = "true" ]; then
    CURRENT_VERSION=$(grep "^$SBUMP_VERSION_VAR=" "$SBUMP_ENV_FILE" | cut -d= -f2)
    if [ -z "$CURRENT_VERSION" ]; then
        echo "Version variable $SBUMP_VERSION_VAR not found in $SBUMP_ENV_FILE"
        show_help
    fi
    commit_and_push "$CURRENT_VERSION" "$PUSH_VERSION"
    exit 0
fi

if [ -z "$BUMP_TYPE" ]; then
    echo "No bump type specified"
    show_help
fi

# Check for uncommitted changes BEFORE making any modifications
if [[ -n $(git status --porcelain) ]]; then
    echo "Error: Working directory has uncommitted changes"
    exit 1
fi

CURRENT_VERSION=$(grep "^$SBUMP_VERSION_VAR=" "$SBUMP_ENV_FILE" | cut -d= -f2)

if [ -z "$CURRENT_VERSION" ]; then
    echo "Version variable $SBUMP_VERSION_VAR not found in $SBUMP_ENV_FILE"
    show_help
fi

NEW_VERSION=$(bump_version "$CURRENT_VERSION" "$BUMP_TYPE")
update_env_file "$NEW_VERSION"

if [ ! -z "$SBUMP_OPENAPI_FILE" ]; then
    if [ -f "$SBUMP_OPENAPI_FILE" ]; then
        update_openapi_file "$NEW_VERSION" "$SBUMP_OPENAPI_FILE"
    else
        echo "OpenAPI file $SBUMP_OPENAPI_FILE not found"
        show_help
    fi
fi

if [ ! -z "$SBUMP_PACKAGE_JSON" ]; then
    if [ -f "$SBUMP_PACKAGE_JSON" ]; then
        update_package_json "$NEW_VERSION" "$SBUMP_PACKAGE_JSON"
    else
        echo "Package.json file $SBUMP_PACKAGE_JSON not found"
        show_help
    fi
fi

if [ "$PUSH_VERSION" = "true" ]; then
    commit_and_push "$NEW_VERSION" "true"
else
    echo "$NEW_VERSION"
fi