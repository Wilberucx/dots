#!/usr/bin/env bash
# dots installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Wilberucx/dots/main/install.sh | bash
set -euo pipefail

REPO="https://github.com/Wilberucx/dots"

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

# ─── Detect package manager ───────────────────────────────────────────────────
detect_pm() {
    if command -v pacman &>/dev/null;   then echo "pacman"
    elif command -v apt-get &>/dev/null; then echo "apt"
    elif command -v brew &>/dev/null;    then echo "brew"
    else echo "unknown"
    fi
}

# ─── Ensure a command exists, install if not ─────────────────────────────────
ensure() {
    local cmd="$1"
    local pkg="${2:-$1}"
    if command -v "$cmd" &>/dev/null; then
        info "$cmd already available ✓"
        return
    fi
    info "Installing $pkg..."
    case "$(detect_pm)" in
        pacman) sudo pacman -S --noconfirm "$pkg" ;;
        apt)    sudo apt-get install -y "$pkg" ;;
        brew)   brew install "$pkg" ;;
        *)      error "No supported package manager. Install $pkg manually." ;;
    esac
}

# ─── Dependencies ─────────────────────────────────────────────────────────────
info "Checking dependencies..."

ensure python3

# Verify Python >= 3.10
PY_MINOR=$(python3 -c 'import sys; print(sys.version_info.minor)')
PY_MAJOR=$(python3 -c 'import sys; print(sys.version_info.major)')
if [[ "$PY_MAJOR" -lt 3 ]] || [[ "$PY_MAJOR" -eq 3 && "$PY_MINOR" -lt 10 ]]; then
    error "Python 3.10+ required. Found: $(python3 --version)"
fi
info "Python $(python3 --version) ✓"

ensure git

# pipx — special handling
if ! command -v pipx &>/dev/null; then
    info "Installing pipx..."
    case "$(detect_pm)" in
        pacman) sudo pacman -S --noconfirm python-pipx ;;
        apt)    sudo apt-get install -y pipx ;;
        brew)   brew install pipx ;;
        *)      python3 -m pip install --user pipx ;;
    esac
    export PATH="$PATH:$HOME/.local/bin"
fi
info "pipx ✓"

# ─── Install dots ─────────────────────────────────────────────────────────────
info "Installing dots..."

if command -v dots &>/dev/null; then
    warning "dots already installed — upgrading..."
    pipx upgrade dots 2>/dev/null || pipx install "git+${REPO}" --force
else
    pipx install "git+${REPO}"
fi

# ─── PATH ─────────────────────────────────────────────────────────────────────
export PATH="$PATH:$HOME/.local/bin"

SHELL_RC=""
case "${SHELL:-}" in
    */zsh)  SHELL_RC="$HOME/.zshrc" ;;
    */bash) SHELL_RC="$HOME/.bashrc" ;;
esac

PATH_LINE='export PATH="$PATH:$HOME/.local/bin"'
if [[ -n "$SHELL_RC" ]] && ! grep -q '.local/bin' "$SHELL_RC" 2>/dev/null; then
    echo "" >> "$SHELL_RC"
    echo "# added by dots installer" >> "$SHELL_RC"
    echo "$PATH_LINE" >> "$SHELL_RC"
    warning "Added ~/.local/bin to PATH in $SHELL_RC — run: source $SHELL_RC"
fi

# ─── Done ─────────────────────────────────────────────────────────────────────
if command -v dots &>/dev/null; then
    success "$(dots --version) installed successfully!"
    echo ""
    echo "  Next steps:"
    echo "  1. cd ~/your-dotfiles"
    echo "  2. dots init"
    echo "  3. dots link"
    echo ""
else
    error "Installation failed. Try: pipx install git+${REPO}"
fi
