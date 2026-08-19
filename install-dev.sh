#!/usr/bin/env bash
# dots dev installer — instala la versión en desarrollo (experimental)
# REEMPLAZA cualquier versión estable de dots.
# Para volver a la versión estable, ejecuta install.sh
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Wilberucx/dots/feature/unified-init/install-dev.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/Wilberucx/dots/feature/unified-init/install-dev.sh | DOTS_BRANCH=feature/otra-rama bash

set -euo pipefail

GITHUB_REPO="Wilberucx/dots"
BRANCH="${DOTS_BRANCH:-feature/unified-init}"
BIN_DIR="${HOME}/.local/bin"
BINARY="dots"               # mismo nombre que la versión estable (la reemplaza)
DOTS_REPO_DIR="${HOME}/.dots-dev-src"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

info()    { echo -e "${BLUE}[install-dev]${NC} $1"; }
success() { echo -e "${GREEN}[install-dev]${NC} $1"; }
warning() { echo -e "${YELLOW}[install-dev]${NC} $1"; }
error()   { echo -e "${RED}[install-dev]${NC} $1"; exit 1; }

echo -e "${BOLD}dots — experimental build${NC}"
echo -e "${BOLD}Branch: ${BRANCH}${NC}"
echo ""

# ─── Requisitos ──────────────────────────────────────────────────────────────
if ! command -v go &>/dev/null; then
    error "Go is required to build from source. Install Go from https://go.dev/dl/"
fi

if ! command -v git &>/dev/null; then
    error "git is required to clone the repository."
fi

# ─── Eliminar versión estable existente ───────────────────────────────────────
if [[ -f "${BIN_DIR}/${BINARY}" ]]; then
    # Verificar si es la versión estable o ya es dev (para mensaje)
    version_output=$("${BIN_DIR}/${BINARY}" --version 2>/dev/null || echo "unknown")
    if echo "$version_output" | grep -q "experimental"; then
        info "Removing previous experimental build..."
    else
        info "Removing stable 'dots' — replacing with experimental build..."
    fi
    rm -f "${BIN_DIR}/${BINARY}"
fi

# ─── Clonar (o actualizar) ────────────────────────────────────────────────────
if [[ -d "$DOTS_REPO_DIR" ]]; then
    info "Updating existing clone..."
    cd "$DOTS_REPO_DIR"
    git fetch origin "$BRANCH" 2>/dev/null || {
        error "Branch '$BRANCH' not found in remote. Check DOTS_BRANCH."
    }
    git checkout "$BRANCH"
    git pull origin "$BRANCH"
else
    info "Cloning ${GITHUB_REPO} (branch: ${BRANCH})..."
    git clone --branch "$BRANCH" --depth 1 \
        "https://github.com/${GITHUB_REPO}.git" "$DOTS_REPO_DIR" || {
        error "Failed to clone branch '$BRANCH'. Does it exist?"
    }
    cd "$DOTS_REPO_DIR"
fi

# ─── Compilar ─────────────────────────────────────────────────────────────────
info "Building experimental 'dots' from source..."
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)

# Version: branch + short commit (funciona con shallow clone)
DEV_VERSION=$(git rev-parse --short HEAD 2>/dev/null || echo "dev")

go build -o "/tmp/${BINARY}-${GOOS}-${GOARCH}" \
    -ldflags="-s -w \
        -X github.com/Wilberucx/dots/internal/cli.Version=${BRANCH}-${DEV_VERSION} \
        -X github.com/Wilberucx/dots/internal/cli.IsDev=1" \
    ./cmd/dots/

# ─── Instalar ─────────────────────────────────────────────────────────────────
mkdir -p "$BIN_DIR"
cp "/tmp/${BINARY}-${GOOS}-${GOARCH}" "${BIN_DIR}/${BINARY}"
chmod +x "${BIN_DIR}/${BINARY}"
rm -f "/tmp/${BINARY}-${GOOS}-${GOARCH}"

# ─── Verificar ────────────────────────────────────────────────────────────────
if [[ ! -f "${BIN_DIR}/${BINARY}" ]]; then
    error "Installation failed: binary not found at ${BIN_DIR}/${BINARY}"
fi

INSTALLED_VERSION=$("${BIN_DIR}/${BINARY}" version 2>/dev/null || echo "unknown")

# ─── PATH check ───────────────────────────────────────────────────────────────
if ! echo "$PATH" | grep -q "${BIN_DIR}"; then
    warning "${BIN_DIR} is not in your PATH."
    echo "  Add it to your shell config:"
    echo "    export PATH=\"\${PATH}:${BIN_DIR}\""
    echo ""
fi

# ─── Mensaje final ────────────────────────────────────────────────────────────
success "Experimental 'dots' ${INSTALLED_VERSION} installed! ✓"
echo ""
echo "  ⚠ This is an UNSTABLE development version."
echo "     All commands will show a warning banner."
echo ""
echo "  To revert to the stable version:"
echo "    curl -fsSL https://raw.githubusercontent.com/Wilberucx/dots/main/install.sh | bash"
echo ""
echo "  Source cloned at: ${DOTS_REPO_DIR}"
echo "  To update: re-run this script."
echo "  To uninstall: rm ${BIN_DIR}/dots && rm -rf ${DOTS_REPO_DIR}"
echo ""
echo "  Example with a different branch:"
echo "    DOTS_BRANCH=feature/otra-rama bash install-dev.sh"
echo ""
