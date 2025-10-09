#!/bin/bash

# GoFr Multi-Module Tagging & Release Script
# This script helps tag, push tags, and create GitHub releases for individual modules in the GoFr repository

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# GoFr repository information
REPO_URL="https://github.com/gofr-dev/gofr"
REPO_OWNER="gofr-dev"
REPO_NAME="gofr"
DEFAULT_BRANCH="development"

# All external modules from go.work file
declare -a MODULES=(
    "pkg/gofr/datasource/arangodb"
    "pkg/gofr/datasource/cassandra"
    "pkg/gofr/datasource/clickhouse"
    "pkg/gofr/datasource/couchbase"
    "pkg/gofr/datasource/dgraph"
    "pkg/gofr/datasource/elasticsearch"
    "pkg/gofr/datasource/file/ftp"
    "pkg/gofr/datasource/file/s3"
    "pkg/gofr/datasource/file/sftp"
    "pkg/gofr/datasource/influxdb"
    "pkg/gofr/datasource/kv-store/badger"
    "pkg/gofr/datasource/kv-store/dynamodb"
    "pkg/gofr/datasource/kv-store/nats"
    "pkg/gofr/datasource/mongo"
    "pkg/gofr/datasource/opentsdb"
    "pkg/gofr/datasource/oracle"
    "pkg/gofr/datasource/pubsub/eventhub"
    "pkg/gofr/datasource/pubsub/nats"
    "pkg/gofr/datasource/scylladb"
    "pkg/gofr/datasource/solr"
    "pkg/gofr/datasource/surrealdb"
)

# Module display names for releases
declare -A MODULE_NAMES=(
    ["pkg/gofr/datasource/arangodb"]="ArangoDB Datasource"
    ["pkg/gofr/datasource/cassandra"]="Cassandra Datasource"
    ["pkg/gofr/datasource/clickhouse"]="ClickHouse Datasource"
    ["pkg/gofr/datasource/couchbase"]="Couchbase Datasource"
    ["pkg/gofr/datasource/dgraph"]="DGraph Datasource"
    ["pkg/gofr/datasource/elasticsearch"]="Elasticsearch Datasource"
    ["pkg/gofr/datasource/file/ftp"]="FTP File System"
    ["pkg/gofr/datasource/file/s3"]="AWS S3 File System"
    ["pkg/gofr/datasource/file/sftp"]="SFTP File System"
    ["pkg/gofr/datasource/influxdb"]="InfluxDB Datasource"
    ["pkg/gofr/datasource/kv-store/badger"]="BadgerDB Key-Value Store"
    ["pkg/gofr/datasource/kv-store/dynamodb"]="AWS DynamoDB Key-Value Store"
    ["pkg/gofr/datasource/kv-store/nats"]="NATS Key-Value Store"
    ["pkg/gofr/datasource/mongo"]="MongoDB Datasource"
    ["pkg/gofr/datasource/opentsdb"]="OpenTSDB Datasource"
    ["pkg/gofr/datasource/oracle"]="Oracle Database Datasource"
    ["pkg/gofr/datasource/pubsub/eventhub"]="Azure Event Hubs Pub/Sub"
    ["pkg/gofr/datasource/pubsub/nats"]="NATS Pub/Sub"
    ["pkg/gofr/datasource/scylladb"]="ScyllaDB Datasource"
    ["pkg/gofr/datasource/solr"]="Apache Solr Datasource"
    ["pkg/gofr/datasource/surrealdb"]="SurrealDB Datasource"
)

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_release() {
    echo -e "${PURPLE}[RELEASE]${NC} $1"
}

print_github() {
    echo -e "${CYAN}[GITHUB]${NC} $1"
}

# Function to check if GitHub CLI is installed
check_github_cli() {
    if ! command -v gh &> /dev/null; then
        print_warning "GitHub CLI (gh) is not installed"
        print_info "Install it from: https://cli.github.com/"
        print_info "Or use: brew install gh / apt install gh / scoop install gh"
        return 1
    fi

    # Check if authenticated
    if ! gh auth status &> /dev/null; then
        print_warning "GitHub CLI is not authenticated"
        print_info "Run: gh auth login"
        return 1
    fi

    return 0
}

# Function to check if we're in a git repository
check_git_repo() {
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        print_error "Not in a Git repository. Please run this script from the GoFr repository root."
        exit 1
    fi
}

# Function to check current branch
check_branch() {
    current_branch=$(git branch --show-current)
    if [ "$current_branch" != "$DEFAULT_BRANCH" ]; then
        print_warning "Current branch is '$current_branch', but recommended branch is '$DEFAULT_BRANCH'"
        read -p "Do you want to switch to $DEFAULT_BRANCH? [y/N]: " switch_branch
        if [[ $switch_branch =~ ^[Yy]$ ]]; then
            print_info "Switching to $DEFAULT_BRANCH branch..."
            git checkout $DEFAULT_BRANCH
            git pull origin $DEFAULT_BRANCH
        fi
    else
        print_success "On $DEFAULT_BRANCH branch"
        git pull origin $DEFAULT_BRANCH
    fi
}

# Function to get latest tag for a module
get_latest_tag() {
    local module_path=$1
    git tag -l "${module_path}/*" --sort=-version:refname | head -1
}

# Function to list all tags for a module
list_module_tags() {
    local module_path=$1
    print_info "Tags for module: $module_path"
    local tags=$(git tag -l "${module_path}/*" --sort=-version:refname)
    if [ -z "$tags" ]; then
        print_warning "No tags found for $module_path"
    else
        echo "$tags"
    fi
    echo
}

# Function to validate semantic version format
validate_semver() {
    local version=$1
    if [[ $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        return 0
    else
        print_error "Invalid semantic version format: $version"
        print_info "Expected format: vX.Y.Z (e.g., v1.0.0, v2.1.3)"
        return 1
    fi
}

# Function to check if tag already exists
tag_exists() {
    local tag=$1
    git tag -l "$tag" | grep -q "^$tag$"
}

# Function to get changes since last tag
get_changes_since_tag() {
    local module_path=$1
    local latest_tag=$2

    print_info "Changes in $module_path since $latest_tag:"
    if [ -n "$latest_tag" ]; then
        git log $latest_tag..HEAD --oneline --no-merges -- "$module_path/" | head -10
    else
        print_info "No previous tags found. Showing recent commits:"
        git log HEAD --oneline --no-merges -- "$module_path/" | head -10
    fi
    echo
}

# Function to get detailed changes for release notes
get_detailed_changes() {
    local module_path=$1
    local latest_tag=$2

    if [ -n "$latest_tag" ]; then
        git log $latest_tag..HEAD --pretty=format:"- %s" --no-merges -- "$module_path/"
    else
        git log HEAD --pretty=format:"- %s" --no-merges -- "$module_path/" | head -20
    fi
}

# Function to suggest next version
suggest_next_version() {
    local latest_tag=$1
    local module_path=$2

    if [ -z "$latest_tag" ]; then
        echo "v1.0.0"
        return
    fi

    # Extract version number (remove module path and v prefix)
    local version=${latest_tag#${module_path}/v}

    # Split version into parts
    IFS='.' read -ra VERSION_PARTS <<< "$version"
    local major=${VERSION_PARTS[0]}
    local minor=${VERSION_PARTS[1]}
    local patch=${VERSION_PARTS[2]}

    # Suggest patch increment by default
    local next_patch=$((patch + 1))
    echo "v${major}.${minor}.${next_patch}"
}

# Function to generate release notes
generate_release_notes() {
    local module_path=$1
    local version=$2
    local latest_tag=$3
    local module_name="${MODULE_NAMES[$module_path]}"

    cat << EOF
## ${module_name} ${version}

### What's Changed

$(get_detailed_changes "$module_path" "$latest_tag")

### Module Information
- **Module Path**: \`${module_path}\`
- **Go Module**: \`gofr.dev/${module_path}\`
- **Previous Version**: ${latest_tag:-"None"}

### Installation
\`\`\`bash
go get gofr.dev/${module_path}@${version}
\`\`\`

### Documentation
For more information, visit the [GoFr Documentation](https://gofr.dev/docs).

---
**Full Changelog**: https://github.com/${REPO_OWNER}/${REPO_NAME}/compare/${latest_tag}...${module_path}/${version}
EOF
}

# Function to create GitHub release
create_github_release() {
    local module_path=$1
    local version=$2
    local tag=$3
    local latest_tag=$4
    local create_release=${5:-false}

    if [ "$create_release" != true ]; then
        return 0
    fi

    if ! check_github_cli; then
        print_warning "Cannot create GitHub release without GitHub CLI"
        return 1
    fi

    local module_name="${MODULE_NAMES[$module_path]}"
    local release_title="${module_name} ${version}"
    local release_notes=$(generate_release_notes "$module_path" "$version" "$latest_tag")

    print_release "Creating GitHub release..."
    print_info "Title: $release_title"
    print_info "Tag: $tag"

    # Show preview of release notes
    echo "----------------------------------------"
    echo "Release Notes Preview:"
    echo "----------------------------------------"
    echo "$release_notes"
    echo "----------------------------------------"

    read -p "Create GitHub release with these notes? [y/N]: " confirm_release
    if [[ ! $confirm_release =~ ^[Yy]$ ]]; then
        print_warning "Skipping GitHub release creation"
        return 0
    fi

    # Create the release
    if gh release create "$tag" \
        --title "$release_title" \
        --notes "$release_notes" \
        --target "$DEFAULT_BRANCH"; then
        print_success "GitHub release created: $release_title"
        print_github "View release: https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/tag/${tag}"
    else
        print_error "Failed to create GitHub release"
        return 1
    fi
}

# Function to tag a single module
tag_module() {
    local module_path=$1
    local version=$2
    local push_immediately=${3:-false}
    local create_release=${4:-false}

    # Validate inputs
    if [ ! -d "$module_path" ]; then
        print_error "Module directory not found: $module_path"
        return 1
    fi

    if [ ! -f "$module_path/go.mod" ]; then
        print_error "go.mod not found in $module_path"
        return 1
    fi

    if ! validate_semver "$version"; then
        return 1
    fi

    local tag="${module_path}/${version}"

    # Check if tag already exists
    if tag_exists "$tag"; then
        print_error "Tag $tag already exists!"
        return 1
    fi

    # Get latest tag for comparison
    local latest_tag=$(get_latest_tag "$module_path")
    local module_name="${MODULE_NAMES[$module_path]}"

    echo "========================================"
    print_info "MODULE: $module_name"
    print_info "Path: $module_path"
    print_info "Latest tag: ${latest_tag:-'None'}"
    print_info "New tag: $tag"
    echo "========================================"

    # Show changes since last tag
    get_changes_since_tag "$module_path" "$latest_tag"

    # Confirm tagging with detailed breakdown
    echo "========================================"
    print_warning "CONFIRMATION REQUIRED"
    echo "Module: $module_name"
    echo "Version: $version"
    echo "Tag: $tag"
    echo "Push immediately: $([ "$push_immediately" = true ] && echo "Yes" || echo "No")"
    echo "Create GitHub release: $([ "$create_release" = true ] && echo "Yes" || echo "No")"
    echo "========================================"

    read -p "Proceed with tag creation? [y/N]: " confirm
    if [[ ! $confirm =~ ^[Yy]$ ]]; then
        print_warning "Skipping tag creation for $module_path"
        return 0
    fi

    # Create the tag
    print_info "Creating tag: $tag"
    git tag "$tag"

    if [ $? -eq 0 ]; then
        print_success "Tag created: $tag"

        # Push if requested
        if [ "$push_immediately" = true ]; then
            print_info "Pushing tag to remote..."
            git push origin "$tag"
            if [ $? -eq 0 ]; then
                print_success "Tag pushed: $tag"

                # Create GitHub release if requested
                if [ "$create_release" = true ]; then
                    create_github_release "$module_path" "$version" "$tag" "$latest_tag" true
                fi
            else
                print_error "Failed to push tag: $tag"
                return 1
            fi
        else
            print_info "Tag created locally. Use 'git push origin $tag' to push to remote."
            if [ "$create_release" = true ]; then
                print_warning "Cannot create GitHub release without pushing tag first"
                print_info "Push the tag first, then run the release creation separately"
            fi
        fi
    else
        print_error "Failed to create tag: $tag"
        return 1
    fi
}

# Function to show enhanced menu
show_menu() {
    echo
    print_info "GoFr Multi-Module Tagging & Release Script"
    echo "=============================================="
    echo "1.  List all modules"
    echo "2.  Show tags for a specific module"
    echo "3.  Show tags for all modules"
    echo "4.  Tag a specific module (simple)"
    echo "5.  Tag with GitHub release (advanced)"
    echo "6.  Interactive tagging session"
    echo "7.  Bulk tag multiple modules"
    echo "8.  Push all unpushed tags"
    echo "9.  Create release for existing tag"
    echo "10. Show repository status"
    echo "11. GitHub CLI status"
    echo "12. Exit"
    echo
}

# Function to list all modules
list_all_modules() {
    print_info "Available modules in GoFr repository:"
    for i in "${!MODULES[@]}"; do
        local module="${MODULES[$i]}"
        local name="${MODULE_NAMES[$module]}"
        printf "%2d. %-35s (%s)\n" $((i+1)) "$name" "$module"
    done
    echo
}

# Function to show repository status
show_repo_status() {
    print_info "Repository Status:"
    echo "Current branch: $(git branch --show-current)"
    echo "Repository URL: $REPO_URL"
    echo "Total modules: ${#MODULES[@]}"
    echo

    print_info "Recent commits on current branch:"
    git log --oneline -5
    echo
}

# Function to show GitHub CLI status
show_github_status() {
    print_github "GitHub CLI Status:"

    if ! command -v gh &> /dev/null; then
        print_error "GitHub CLI not installed"
        print_info "Install from: https://cli.github.com/"
        return
    fi

    print_success "GitHub CLI installed: $(gh --version | head -1)"

    if gh auth status &> /dev/null; then
        print_success "GitHub CLI authenticated"
        gh auth status
    else
        print_error "GitHub CLI not authenticated"
        print_info "Run: gh auth login"
    fi
    echo
}

# Function to push all unpushed tags
push_all_tags() {
    print_info "Checking for unpushed tags..."
    local unpushed_tags=$(git tag --sort=-version:refname | while read tag; do
        if ! git ls-remote --exit-code origin refs/tags/$tag >/dev/null 2>&1; then
            echo $tag
        fi
    done)

    if [ -z "$unpushed_tags" ]; then
        print_success "No unpushed tags found."
        return 0
    fi

    print_info "Unpushed tags:"
    echo "$unpushed_tags"
    echo

    read -p "Push all unpushed tags? [y/N]: " confirm
    if [[ $confirm =~ ^[Yy]$ ]]; then
        print_info "Pushing all tags..."
        git push origin --tags
        if [ $? -eq 0 ]; then
            print_success "All tags pushed successfully!"
        else
            print_error "Failed to push some tags"
        fi
    fi
}

# Function to create release for existing tag
create_release_for_tag() {
    if ! check_github_cli; then
        print_error "GitHub CLI required for release creation"
        return 1
    fi

    print_info "Available pushed tags:"
    git ls-remote --tags origin | grep -E 'pkg/gofr/datasource' | tail -10
    echo

    read -p "Enter tag name for release: " tag_name

    if [ -z "$tag_name" ]; then
        print_error "Tag name cannot be empty"
        return 1
    fi

    # Verify tag exists
    if ! git tag -l "$tag_name" | grep -q "^$tag_name$"; then
        print_error "Tag $tag_name not found locally"
        return 1
    fi

    # Extract module path and version from tag
    local module_path="${tag_name%/*}"
    local version="${tag_name##*/}"

    # Get previous tag for changelog
    local latest_tag=$(git tag -l "${module_path}/*" --sort=-version:refname | grep -v "^$tag_name$" | head -1)

    create_github_release "$module_path" "$version" "$tag_name" "$latest_tag" true
}

# Function for enhanced interactive tagging session
interactive_tagging_enhanced() {
    print_info "Enhanced Interactive Tagging Session"
    print_info "Tag modules with optional GitHub release creation"
    echo

    # Ask about default preferences
    read -p "Create GitHub releases by default? [y/N]: " default_release
    default_create_release=false
    [[ $default_release =~ ^[Yy]$ ]] && default_create_release=true

    read -p "Push tags immediately by default? [y/N]: " default_push
    default_push_immediately=false
    [[ $default_push =~ ^[Yy]$ ]] && default_push_immediately=true

    echo
    print_info "Default settings:"
    echo "- Create releases: $([ "$default_create_release" = true ] && echo "Yes" || echo "No")"
    echo "- Push immediately: $([ "$default_push_immediately" = true ] && echo "Yes" || echo "No")"
    echo

    for module in "${MODULES[@]}"; do
        echo "=========================================="
        local module_name="${MODULE_NAMES[$module]}"
        print_info "MODULE: $module_name"
        print_info "Path: $module"

        # Check if module directory exists
        if [ ! -d "$module" ]; then
            print_warning "Module directory not found: $module (skipping)"
            continue
        fi

        # Get latest tag
        latest_tag=$(get_latest_tag "$module")
        suggested_version=$(suggest_next_version "$latest_tag" "$module")

        print_info "Latest tag: ${latest_tag:-'None'}"
        print_info "Suggested version: $suggested_version"

        # Show recent changes
        get_changes_since_tag "$module" "$latest_tag"

        read -p "Tag this module? [y/N/q]: " action
        case $action in
            [Yy]*)
                read -p "Enter version [$suggested_version]: " version
                version=${version:-$suggested_version}

                # Ask about push (with default)
                push_prompt="Push immediately? [$([ "$default_push_immediately" = true ] && echo "Y/n" || echo "y/N")]: "
                read -p "$push_prompt" push_now
                if [ "$default_push_immediately" = true ]; then
                    push_flag=true
                    [[ $push_now =~ ^[Nn]$ ]] && push_flag=false
                else
                    push_flag=false
                    [[ $push_now =~ ^[Yy]$ ]] && push_flag=true
                fi

                # Ask about release (with default)
                release_prompt="Create GitHub release? [$([ "$default_create_release" = true ] && echo "Y/n" || echo "y/N")]: "
                read -p "$release_prompt" create_rel
                if [ "$default_create_release" = true ]; then
                    release_flag=true
                    [[ $create_rel =~ ^[Nn]$ ]] && release_flag=false
                else
                    release_flag=false
                    [[ $create_rel =~ ^[Yy]$ ]] && release_flag=true
                fi

                tag_module "$module" "$version" "$push_flag" "$release_flag"
                ;;
            [Qq]*)
                print_info "Exiting interactive session"
                break
                ;;
            *)
                print_info "Skipping $module"
                ;;
        esac
        echo
    done
}

# Function to select module by number
select_module() {
    list_all_modules
    read -p "Enter module number (1-${#MODULES[@]}): " module_num

    if [[ $module_num =~ ^[0-9]+$ ]] && [ $module_num -ge 1 ] && [ $module_num -le ${#MODULES[@]} ]; then
        echo "${MODULES[$((module_num-1))]}"
    else
        print_error "Invalid module number"
        return 1
    fi
}

# Main script logic
main() {
    print_info "GoFr Multi-Module Tagging & Release Script"
    print_info "Repository: $REPO_URL"
    echo

    # Check prerequisites
    check_git_repo
    check_branch

    # Interactive menu
    while true; do
        show_menu
        read -p "Choose an option [1-12]: " choice

        case $choice in
            1)
                list_all_modules
                ;;
            2)
                if module=$(select_module); then
                    list_module_tags "$module"
                fi
                ;;
            3)
                print_info "Tags for all modules:"
                echo "===================="
                for module in "${MODULES[@]}"; do
                    list_module_tags "$module"
                done
                ;;
            4)
                if module=$(select_module); then
                    latest_tag=$(get_latest_tag "$module")
                    suggested_version=$(suggest_next_version "$latest_tag" "$module")

                    print_info "Latest tag: ${latest_tag:-'None'}"
                    print_info "Suggested version: $suggested_version"

                    read -p "Enter version [$suggested_version]: " version
                    version=${version:-$suggested_version}

                    read -p "Push immediately? [y/N]: " push_now
                    push_flag=false
                    [[ $push_now =~ ^[Yy]$ ]] && push_flag=true

                    tag_module "$module" "$version" "$push_flag" false
                fi
                ;;
            5)
                if module=$(select_module); then
                    latest_tag=$(get_latest_tag "$module")
                    suggested_version=$(suggest_next_version "$latest_tag" "$module")

                    print_info "Latest tag: ${latest_tag:-'None'}"
                    print_info "Suggested version: $suggested_version"

                    read -p "Enter version [$suggested_version]: " version
                    version=${version:-$suggested_version}

                    read -p "Push immediately? [y/N]: " push_now
                    push_flag=false
                    [[ $push_now =~ ^[Yy]$ ]] && push_flag=true

                    read -p "Create GitHub release? [y/N]: " create_rel
                    release_flag=false
                    [[ $create_rel =~ ^[Yy]$ ]] && release_flag=true

                    tag_module "$module" "$version" "$push_flag" "$release_flag"
                fi
                ;;
            6)
                interactive_tagging_enhanced
                ;;
            7)
                print_info "Bulk tagging not implemented yet"
                ;;
            8)
                push_all_tags
                ;;
            9)
                create_release_for_tag
                ;;
            10)
                show_repo_status
                ;;
            11)
                show_github_status
                ;;
            12)
                print_success "Goodbye!"
                exit 0
                ;;
            *)
                print_error "Invalid option. Please choose 1-12."
                ;;
        esac

        # Pause before showing menu again
        echo
        read -p "Press Enter to continue..."
    done
}

# Help function
show_help() {
    cat << EOF
GoFr Multi-Module Tagging & Release Script

USAGE:
    $0 [OPTIONS]

OPTIONS:
    -h, --help              Show this help message
    -l, --list              List all available modules
    -m, --module PATH       Specify module path directly
    -v, --version VER       Specify version for tagging
    -p, --push              Push tag immediately after creation
    -r, --release           Create GitHub release after tagging
    --no-interactive        Run in non-interactive mode (requires -m and -v)

EXAMPLES:
    # Interactive mode (default)
    $0

    # Tag specific module
    $0 -m pkg/gofr/datasource/mongo -v v1.2.3

    # Tag, push, and create release
    $0 -m pkg/gofr/datasource/mongo -v v1.2.3 -p -r

    # List all modules
    $0 -l

REQUIREMENTS:
    - Git repository (GoFr)
    - GitHub CLI (gh) for release creation
    - Proper authentication for GitHub

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -l|--list)
            check_git_repo
            list_all_modules
            exit 0
            ;;
        -m|--module)
            MODULE_PATH="$2"
            shift 2
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -p|--push)
            PUSH_IMMEDIATELY=true
            shift
            ;;
        -r|--release)
            CREATE_RELEASE=true
            shift
            ;;
        --no-interactive)
            NON_INTERACTIVE=true
            shift
            ;;
        *)
            print_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Handle non-interactive mode
if [ "$NON_INTERACTIVE" = true ]; then
    if [ -z "$MODULE_PATH" ] || [ -z "$VERSION" ]; then
        print_error "Non-interactive mode requires -m (module) and -v (version) options"
        exit 1
    fi

    check_git_repo
    check_branch
    tag_module "$MODULE_PATH" "$VERSION" "${PUSH_IMMEDIATELY:-false}" "${CREATE_RELEASE:-false}"
    exit $?
fi

# Handle direct module tagging
if [ -n "$MODULE_PATH" ] && [ -n "$VERSION" ]; then
    check_git_repo
    check_branch
    tag_module "$MODULE_PATH" "$VERSION" "${PUSH_IMMEDIATELY:-false}" "${CREATE_RELEASE:-false}"
    exit $?
fi

# Run main interactive program
main
