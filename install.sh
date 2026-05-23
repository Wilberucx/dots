#!/usr/bin/env bash
# dots installer (Go binary)
# Usage: curl -fsSL https://raw.githubusercontent.com/Wilberucx/dots/main/install.sh | bash
set -euo pipefail

# ─── Repo info ───────────────────────────────────────────────────────────────
GITHUB_REPO="Wilberucx/dots"        # GitHub owner/repo (for releases)
GO_MODULE="github.com/Wilberucx/dots"  # Go module path (for go install)
BIN_DIR="${HOME}/.local/bin"
BINARY="${BIN_DIR}/dots"
VERSION="${DOTS_VERSION:-latest}"

# ─── Colors ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

info()    { echo -e "${BLUE}[dots]${NC} $1"; }
success() { echo -e "${GREEN}[dots]${NC} $1"; }
warning() { echo -e "${YELLOW}[dots]${NC} $1"; }
error()   { echo -e "${RED}[dots]${NC} $1"; exit 1; }

echo -e "${BOLD}dots — dotfile manager${NC}"
echo ""

# ─── Detect OS & arch ────────────────────────────────────────────────────────
detect_os() {
    case "$(uname -s)" in
        Linux)  echo "linux" ;;
        Darwin) echo "darwin" ;;
        *)      echo "" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *)             echo "" ;;
    esac
}

OS=$(detect_os)
ARCH=$(detect_arch)

if [[ -z "$OS" ]]; then
    error "Unsupported OS: $(uname -s). dots currently supports Linux and macOS."
fi

if [[ -z "$ARCH" ]]; then
    error "Unsupported architecture: $(uname -m). dots currently supports amd64 and arm64."
fi

# ─── Install methods ─────────────────────────────────────────────────────────
install_from_release() {
    local release_tag="$1"
    local url="https://github.com/${GITHUB_REPO}/releases/${release_tag}/download/dots-${OS}-${ARCH}.tar.gz"
    local tmp_dir
    tmp_dir=$(mktemp -d)
    cd "$tmp_dir"

    info "Downloading dots for ${OS}-${ARCH}..."
    if command -v curl &>/dev/null; then
        curl -fsSL "$url" -o dots.tar.gz 2>/dev/null || { cd /; rm -rf "$tmp_dir"; return 1; }
    elif command -v wget &>/dev/null; then
        wget -q "$url" -O dots.tar.gz 2>/dev/null || { cd /; rm -rf "$tmp_dir"; return 1; }
    else
        cd /; rm -rf "$tmp_dir"
        return 1
    fi

    info "Extracting..."
    tar -xzf dots.tar.gz 2>/dev/null || { cd /; rm -rf "$tmp_dir"; return 1; }

    # Find the extracted binary (may be named dots-{os}-{arch})
    local extracted
    extracted=$(find . -maxdepth 1 -type f -name 'dots-*' 2>/dev/null | head -1)
    if [[ -z "$extracted" ]]; then
        cd /; rm -rf "$tmp_dir"
        return 1
    fi

    chmod +x "$extracted"
    mkdir -p "$BIN_DIR"
    cp "$extracted" "$BINARY"
    cd /
    rm -rf "$tmp_dir"
    return 0
}

install_from_source() {
    if ! command -v go &>/dev/null; then
        error "No pre-built release available for ${OS}-${ARCH} and Go is not installed."
        error "Install Go from https://go.dev/dl/ then re-run this script, or wait for a release."
    fi

    info "Building from source (go install)..."
    local go_pkg="${GO_MODULE}/cmd/dots@${VERSION}"
    if ! go install "$go_pkg"; then
        # Fall back to @latest
        go install "${GO_MODULE}/cmd/dots@latest" || {
            error "Failed to build from source: ${GO_MODULE}/cmd/dots"
        }
    fi

    local go_bin
    go_bin=$(go env GOBIN 2>/dev/null || echo "")
    if [[ -z "$go_bin" ]]; then
        go_bin="${HOME}/go/bin"
    fi

    if [[ -f "${go_bin}/dots" ]]; then
        mkdir -p "$BIN_DIR"
        cp "${go_bin}/dots" "$BINARY"
    else
        error "Built binary not found at ${go_bin}/dots"
    fi
}

# ─── Install / Update ────────────────────────────────────────────────────────
INSTALL_MODE="install"
if [[ -f "$BINARY" ]]; then
    INSTALL_MODE="update"
    warning "dots already installed at ${BINARY} — updating..."
fi

# Try GitHub Release first, fall back to building from source
if [[ "$VERSION" == "latest" ]]; then
    RELEASE_TAG="latest"
else
    RELEASE_TAG="tags/${VERSION}"
fi

if install_from_release "$RELEASE_TAG"; then
    info "Downloaded dots binary ✓"
else
    if [[ "$INSTALL_MODE" == "update" ]]; then
        warning "No pre-built release found, falling back to source build..."
    fi
    install_from_source
fi

# ─── Verify ───────────────────────────────────────────────────────────────────
if [[ ! -f "$BINARY" ]]; then
    error "Installation failed: binary not found at ${BINARY}"
fi

chmod +x "$BINARY"
INSTALLED_VERSION=$("$BINARY" version 2>/dev/null || echo "unknown")

# ─── PATH ─────────────────────────────────────────────────────────────────────
export PATH="${BIN_DIR}:${PATH}"

SHELL_RC=""
case "${SHELL:-}" in
    */zsh)  SHELL_RC="${HOME}/.zshrc" ;;
    */bash) SHELL_RC="${HOME}/.bashrc" ;;
esac

PATH_LINE="export PATH=\"\${PATH}:${BIN_DIR}\""
if [[ -n "$SHELL_RC" ]] && ! grep -q "${BIN_DIR}" "$SHELL_RC" 2>/dev/null; then
    echo "" >> "$SHELL_RC"
    echo "# added by dots installer" >> "$SHELL_RC"
    echo "$PATH_LINE" >> "$SHELL_RC"
    warning "Added ${BIN_DIR} to PATH in ${SHELL_RC} — run: source ${SHELL_RC}"
fi

# ─── Done ─────────────────────────────────────────────────────────────────────
if [[ "$INSTALL_MODE" == "update" ]]; then
    success "dots updated to ${INSTALLED_VERSION} ✓"
else
    success "dots ${INSTALLED_VERSION} installed successfully! ✓"
    echo ""
    echo "  Next steps:"
    echo "  1. cd ~/your-dotfiles"
    echo "  2. dots init"
    echo "  3. dots link"
    echo ""
fi

if echo "$INSTALLED_VERSION" | grep -qi "dev"; then
    warning "Development version — run 'dots version' to check for updates."
fi
