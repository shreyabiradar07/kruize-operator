#!/bin/bash

# Script to build and push operator and bundle images with verification
# Usage: ./scripts/operator_build_and_push.sh -o <operator_image> -b <bundle_image>

set -e  # Exit on error

# Default values
OPERATOR_IMAGE=""
BUNDLE_IMAGE=""
CONTAINER_TOOL="${CONTAINER_TOOL:-docker}"

# Helper functions
log_info() {
    echo "[INFO] $1"
}

log_success() {
    echo "[SUCCESS] $1"
}

log_error() {
    echo "[ERROR] $1"
}

usage() {
    echo "Usage: $0 -o <operator_image> -b <bundle_image>"
    echo ""
    echo "Options:"
    echo "  -o    Operator image (e.g., quay.io/kruize/kruize-operator:0.0.4)"
    echo "  -b    Bundle image (e.g., quay.io/kruize/kruize-operator-bundle:0.0.4)"
    echo "  -h    Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  CONTAINER_TOOL  Container tool to use for build, push, and verification (default: docker)"
    echo "                  Supported values: docker, podman"
    echo ""
    echo "Examples:"
    echo "  # Using default docker"
    echo "  $0 -o quay.io/kruize/kruize-operator:0.0.5 -b quay.io/kruize/kruize-operator-bundle:0.0.5"
    echo ""
    echo "  # Using podman"
    echo "  CONTAINER_TOOL=podman $0 -o quay.io/kruize/kruize-operator:0.0.5 -b quay.io/kruize/kruize-operator-bundle:0.0.5"
}

# Parse command line arguments
while getopts "o:b:h" opt; do
    case $opt in
        o)
            OPERATOR_IMAGE="$OPTARG"
            ;;
        b)
            BUNDLE_IMAGE="$OPTARG"
            ;;
        h)
            usage
            exit 0
            ;;
        \?)
            log_error "Invalid option: -$OPTARG"
            usage
            exit 1
            ;;
        :)
            log_error "Option -$OPTARG requires an argument"
            usage
            exit 1
            ;;
    esac
done

# Validate required arguments
if [[ -z "$OPERATOR_IMAGE" ]] || [[ -z "$BUNDLE_IMAGE" ]]; then
    log_error "Both operator image (-o) and bundle image (-b) are required"
    echo ""
    usage
    exit 1
fi

# Extract version from image tags
extract_version() {
    local image=$1
    echo ${image} | grep -oP ':\K[^:]+$'
}

OPERATOR_VERSION=$(extract_version "$OPERATOR_IMAGE")
BUNDLE_VERSION=$(extract_version "$BUNDLE_IMAGE")

if [[ "$OPERATOR_VERSION" != "$BUNDLE_VERSION" ]]; then
    log_error "Version mismatch: operator version ($OPERATOR_VERSION) != bundle version ($BUNDLE_VERSION)"
    exit 1
fi

VERSION=$OPERATOR_VERSION

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v ${CONTAINER_TOOL} &> /dev/null; then
        log_error "${CONTAINER_TOOL} is not installed"
        exit 1
    fi
    
    if ! command -v make &> /dev/null; then
        log_error "make is not installed"
        exit 1
    fi
    
    if ! command -v operator-sdk &> /dev/null && [[ ! -f "./bin/operator-sdk" ]]; then
        log_info "Downloading operator-sdk..."
        make operator-sdk
    fi
    
    log_success "Prerequisites check passed"
}

# Update Makefile version
update_makefile_version() {
    log_info "Updating Makefile version to ${VERSION}..."
    
    if [[ -f "Makefile" ]]; then
        # Use portable sed -i that works on both GNU (Linux) and BSD (macOS) sed
        # GNU sed accepts -i without argument, BSD sed requires -i '' for no backup
        if [[ "$OSTYPE" == "Darwin" ]]; then
            sed -i '' "s/^VERSION ?= .*/VERSION ?= ${VERSION}/" Makefile
        else
            sed -i "s/^VERSION ?= .*/VERSION ?= ${VERSION}/" Makefile
        fi
        log_success "Makefile version updated"
    else
        log_error "Makefile not found"
        exit 1
    fi
}

# Update CSV version
update_csv_version() {
    log_info "Updating CSV version to ${VERSION}..."
    
    # Update bundle CSV file
    CSV_FILE="bundle/manifests/kruize-operator.clusterserviceversion.yaml"
    if [[ -f "$CSV_FILE" ]]; then
        sed -i "s|containerImage: .*|containerImage: ${OPERATOR_IMAGE}|" "$CSV_FILE"
        sed -i "s/name: kruize-operator\.v.*/name: kruize-operator.v${VERSION}/" "$CSV_FILE"
        log_success "Bundle CSV updated"
    else
        log_info "Bundle CSV not found (will be generated during bundle build)"
    fi
    
    # Update base CSV file
    BASE_CSV_FILE="config/manifests/bases/kruize-operator.clusterserviceversion.yaml"
    if [[ -f "$BASE_CSV_FILE" ]]; then
        sed -i "s|containerImage: .*|containerImage: ${OPERATOR_IMAGE}|" "$BASE_CSV_FILE"
        log_success "Base CSV updated"
    else
        log_info "Base CSV not found"
    fi
}

# Build and push operator image
build_and_push_operator() {
    log_info "Building and pushing operator image: ${OPERATOR_IMAGE}"
    
    log_info "Running: make docker-build docker-push IMG=${OPERATOR_IMAGE} CONTAINER_TOOL=${CONTAINER_TOOL}"
    make docker-build docker-push IMG=${OPERATOR_IMAGE} CONTAINER_TOOL=${CONTAINER_TOOL}
    
    log_success "Operator image built and pushed: ${OPERATOR_IMAGE}"
}

# Verify operator image
verify_operator_image() {
    log_info "Verifying operator image..."
    
    if ${CONTAINER_TOOL} image inspect ${OPERATOR_IMAGE} &> /dev/null; then
        log_success "Operator image verified"
        ${CONTAINER_TOOL} image inspect ${OPERATOR_IMAGE} --format='  Size: {{.Size}} bytes'
    else
        log_error "Operator image not found"
        return 1
    fi
}

# Build and push bundle
build_and_push_bundle() {
    log_info "Building and pushing bundle image: ${BUNDLE_IMAGE}"
    
    log_info "Running: make bundle bundle-build bundle-push VERSION=${VERSION} IMG=${OPERATOR_IMAGE} BUNDLE_IMG=${BUNDLE_IMAGE} CONTAINER_TOOL=${CONTAINER_TOOL}"
    make bundle bundle-build bundle-push VERSION=${VERSION} IMG=${OPERATOR_IMAGE} BUNDLE_IMG=${BUNDLE_IMAGE} CONTAINER_TOOL=${CONTAINER_TOOL}
    
    log_success "Bundle image built and pushed: ${BUNDLE_IMAGE}"
}

# Verify bundle
verify_bundle() {
    log_info "Verifying bundle with operator-sdk..."
    
    OPERATOR_SDK_BIN="operator-sdk"
    if [[ -f "./bin/operator-sdk" ]]; then
        OPERATOR_SDK_BIN="./bin/operator-sdk"
    fi
    
    log_info "Validating bundle directory..."
    ${OPERATOR_SDK_BIN} bundle validate ./bundle
    
    log_success "Bundle validation passed"
    
    if ${CONTAINER_TOOL} image inspect ${BUNDLE_IMAGE} &> /dev/null; then
        log_success "Bundle image verified"
        ${CONTAINER_TOOL} image inspect ${BUNDLE_IMAGE} --format='  Size: {{.Size}} bytes'
    else
        log_error "Bundle image not found"
        return 1
    fi
}

# Main execution
main() {
    echo ""
    log_info "=========================================="
    log_info "Kruize Operator Build & Push Script"
    log_info "=========================================="
    log_info "Version: ${VERSION}"
    log_info "Operator Image: ${OPERATOR_IMAGE}"
    log_info "Bundle Image: ${BUNDLE_IMAGE}"
    log_info "Container Tool: ${CONTAINER_TOOL}"
    log_info "=========================================="
    echo ""
    
    check_prerequisites
    
    # Update versions
    echo ""
    log_info "=========================================="
    log_info "Updating Version Files"
    log_info "=========================================="
    update_makefile_version
    update_csv_version
    
    # Build and push operator
    echo ""
    log_info "=========================================="
    log_info "Building and Pushing Operator Image"
    log_info "=========================================="
    build_and_push_operator
    verify_operator_image
    
    # Build and push bundle
    echo ""
    log_info "=========================================="
    log_info "Building and Pushing Bundle Image"
    log_info "=========================================="
    build_and_push_bundle
    verify_bundle
    
    # Final summary
    echo ""
    log_info "=========================================="
    log_success "Build and Push Complete!"
    log_info "=========================================="
    log_info "Version: ${VERSION}"
    log_info "Operator Image: ${OPERATOR_IMAGE}"
    log_info "Bundle Image: ${BUNDLE_IMAGE}"
    echo ""
    log_info "Next steps:"
    log_info "  Deploy: make deploy IMG=${OPERATOR_IMAGE}"
    log_info "  Test: operator-sdk run bundle ${BUNDLE_IMAGE}"
    echo ""
}

# Run main function
main

